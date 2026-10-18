#!/usr/bin/env python3
"""Repository Markdown RAG; adapted from Week4's cached SQLite cosine index.

Only Python's standard library is needed. Source text and line numbers are kept
verbatim; embedding prompts match the previous embeddinggemma pipeline.
"""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import sqlite3
import subprocess
import sys
import tempfile
import urllib.request
from urllib.parse import urlparse

VERSION = 2
CORPUS_SNAPSHOT = None
EXCLUDED = {'.git', '.venv', 'node_modules', '__pycache__', 'data'}


def digest(value):
    return hashlib.sha256(value.encode()).hexdigest()


def corpus(root):
    if CORPUS_SNAPSHOT is not None:
        return CORPUS_SNAPSHOT
    root = Path(root).resolve()
    try:
        listing = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], stderr=subprocess.DEVNULL)
    except subprocess.CalledProcessError:
        listing = b'\0'.join(os.fsencode(str(p.relative_to(root))) for p in root.rglob('*.md'))
    docs = []
    for name in sorted(set(os.fsdecode(p) for p in listing.split(b'\0') if p)):
        path = root / name
        if path.suffix.lower() != '.md' or any(p in EXCLUDED for p in Path(name).parts):
            continue
        if not path.is_file() or not path.resolve().is_relative_to(root):
            continue
        # Resolve internal links, then open every resolved component without following
        # links again. A concurrent link replacement cannot bypass the boundary.
        resolved = path.resolve().relative_to(root)
        if any(p.lower() == '.git' for p in resolved.parts):
            continue
        fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY)
        try:
            for part in resolved.parts[:-1]:
                child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                os.close(fd)
                fd = child
            leaf = os.open(resolved.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
            with os.fdopen(leaf, 'rb') as stream:
                import stat
                if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
                    continue
                data = stream.read(256 * 1024 + 1)
                if len(data) > 256 * 1024:
                    raise ValueError('Markdown file exceeds 256 KiB')
                text = data.decode('utf-8')
        finally:
            os.close(fd)
        docs.append({'path': name, 'text': text, 'hash': digest(text)})
    return docs


def fingerprint(docs):
    return digest(json.dumps([(d['path'], d['hash']) for d in docs]))


def chunks(doc, limit=1800):
    """Split on headings and paragraphs; bound oversized blocks by whole lines."""
    lines = doc['text'].splitlines(keepends=True)
    headings, start, size, fenced = [], 0, 0, False
    section = []
    def emit(end):
        text = ''.join(lines[start:end])
        if not text.strip():
            return None
        return {'id': digest(doc['path'] + doc['hash'] + str((start, end))),
                'source': doc['path'], 'title': doc['path'], 'sections': list(section),
                'week': next((p for p in Path(doc['path']).parts if re.fullmatch(r'week\d+', p)), 'root'),
                'line_start': start + 1, 'line_end': end, 'text': text}
    for i, line in enumerate(lines):
        heading = re.match(r'^(#{1,6})\s+(.+)', line) if not fenced else None
        if i > start and (heading or (size >= limit and not fenced)):
            item = emit(i)
            if item:
                yield item
            start, size = i, 0
        if heading:
            level = len(heading[1])
            headings = headings[:level - 1] + [heading[2].strip()]
            section = list(headings)
        if line.lstrip().startswith(('```', '~~~')):
            fenced = not fenced
        size += len(line)
    item = emit(len(lines))
    if item:
        yield item


class Embedder:
    def __init__(self, url, model, cache):
        parsed = urlparse(url)
        if parsed.scheme != 'http' or parsed.hostname not in ('127.0.0.1', 'localhost', '::1') or parsed.username:
            raise ValueError('Embeddings require a loopback HTTP Ollama endpoint')
        self.url, self.model = url.rstrip('/'), model
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        models = self.request('/api/tags')['models']
        item = next((m for m in models if m['name'] == model), None)
        if not item:
            raise ValueError('Embedding model missing; run ollama pull ' + model)
        show = self.request('/api/show', {'model': model})
        if model.endswith(':cloud') or show.get('remote_host') or show.get('remote_model'):
            raise ValueError('Remote embedding models are unsupported')
        self.identity = {'model': model, 'digest': item['digest'], 'format': VERSION}
        self.cache = sqlite3.connect(cache)
        self.cache.execute('CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, vector TEXT)')

    def request(self, path, payload=None):
        data = None if payload is None else json.dumps(payload).encode()
        req = urllib.request.Request(self.url + path, data=data, headers={'Content-Type': 'application/json'})
        with self.opener.open(req, timeout=180) as response:
            return json.load(response)

    def embed(self, text):
        key = digest(json.dumps(self.identity, sort_keys=True) + text)
        row = self.cache.execute('SELECT vector FROM cache WHERE key=?', (key,)).fetchone()
        if row:
            return json.loads(row[0])
        result = self.request('/api/embed', {'model': self.model, 'input': [text], 'truncate': False})
        if len(result.get('embeddings', [])) != 1:
            raise ValueError('Invalid embedding count')
        vector = normalize(result['embeddings'][0])
        self.cache.execute('INSERT OR REPLACE INTO cache VALUES (?,?)', (key, json.dumps(vector)))
        self.cache.commit()
        return vector


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ValueError('Ollama redirects are unsupported')


def normalize(vector):
    if not vector or not all(isinstance(x, (int, float)) and math.isfinite(x) for x in vector):
        raise ValueError('Invalid embedding vector')
    norm = math.sqrt(sum(x*x for x in vector))
    if not norm or not math.isfinite(norm):
        raise ValueError('Invalid embedding norm')
    return [x/norm for x in vector]


def build(root, database, embedder):
    docs = corpus(root)
    if not docs:
        raise ValueError('No Markdown documents found')
    fd, temporary = tempfile.mkstemp(dir=Path(database).parent, suffix='.sqlite')
    os.close(fd)
    try:
        with sqlite3.connect(temporary) as db:
            db.executescript('CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT); CREATE TABLE chunks (id TEXT PRIMARY KEY, data TEXT, vector TEXT);')
            count, dimension = 0, None
            for doc in docs:
                for chunk in chunks(doc):
                    vector = embedder.embed('title: ' + chunk['title'] + ' > ' + ' / '.join(chunk['sections']) + ' | text: ' + chunk['text'])
                    if dimension is not None and dimension != len(vector):
                        raise ValueError('Embedding dimensions changed')
                    dimension = len(vector)
                    db.execute('INSERT INTO chunks VALUES (?,?,?)', (chunk['id'], json.dumps(chunk), json.dumps(vector)))
                    count += 1
            metadata = {'version': VERSION, 'root': str(Path(root).resolve()), 'fingerprint': fingerprint(docs), 'embedding': embedder.identity, 'documents': len(docs), 'chunks': count}
            db.executemany('INSERT INTO metadata VALUES (?,?)', [(k, json.dumps(v)) for k, v in metadata.items()])
        if fingerprint(corpus(root)) != metadata['fingerprint']:
            raise ValueError('Documents changed during indexing; retry')
        os.replace(temporary, database)
        return metadata
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def status(root, database):
    if not Path(database).exists():
        raise ValueError('Index missing; run advent-agent rag index')
    with sqlite3.connect('file:' + str(Path(database).resolve()) + '?mode=ro', uri=True) as db:
        meta = {k: json.loads(v) for k, v in db.execute('SELECT key,value FROM metadata')}
    meta['stale'] = (meta['version'] != VERSION or meta['root'] != str(Path(root).resolve()) or meta['fingerprint'] != fingerprint(corpus(root)))
    return meta


def search(root, database, embedder, query, limit=20):
    meta = status(root, database)
    if meta['stale']:
        raise ValueError('Documentation index is stale; run advent-agent rag index')
    if meta['embedding'] != embedder.identity:
        raise ValueError('Embedding model changed; rebuild with advent-agent rag index')
    vector = embedder.embed('task: search result | query: ' + query)
    ranked = []
    with sqlite3.connect('file:' + str(Path(database).resolve()) + '?mode=ro', uri=True) as db:
        for data, values in db.execute('SELECT data,vector FROM chunks'):
            other = json.loads(values)
            if len(other) != len(vector):
                raise ValueError('Embedding dimension mismatch')
            chunk = json.loads(data)
            chunk['similarity'] = sum(a*b for a, b in zip(vector, other))
            ranked.append(chunk)
    if status(root, database)['fingerprint'] != meta['fingerprint'] or status(root, database)['stale']:
        raise ValueError('Index or documents changed during search; retry')
    return sorted(ranked, key=lambda c: (-c['similarity'], c['id']))[:limit]


def main():
    global CORPUS_SNAPSHOT
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--corpus-stdin', action='store_true')
    p.add_argument('--root', required=True)
    p.add_argument('--database', required=True)
    p.add_argument('--ollama', default='http://127.0.0.1:11434')
    p.add_argument('--model', default='embeddinggemma:300m')
    p.add_argument('command', choices=['index', 'status', 'search'])
    p.add_argument('query', nargs='?', default='')
    a = p.parse_args()
    if a.corpus_stdin:
        CORPUS_SNAPSHOT = json.load(sys.stdin)
    Path(a.database).parent.mkdir(parents=True, exist_ok=True)
    if a.command == 'status':
        result = status(a.root, a.database)
    else:
        if a.command == 'search':
            meta = status(a.root, a.database)
            if meta['stale']:
                raise ValueError('Documentation index is stale; run advent-agent rag index')
        e = Embedder(a.ollama, a.model, str(Path(a.database).with_suffix('.cache.sqlite')))
        result = build(a.root, a.database, e) if a.command == 'index' else search(a.root, a.database, e, a.query)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
