import hashlib
import json
import re
import sqlite3
import time
import urllib.request
import urllib.error
from pathlib import Path

import numpy as np
from bs4 import BeautifulSoup


def digest(value):
    return hashlib.sha256(value.encode('utf-8')).hexdigest()


def request_json(url, body=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, headers={'Content-Type': 'application/json', 'User-Agent': 'kotlin-index/0.1'})
    with urllib.request.urlopen(req, timeout=180) as response:
        return json.load(response)


def extract(html):
    soup = BeautifulSoup(html, 'html.parser')
    article = soup.find('article') or soup.find('main')
    if article is None:
        raise ValueError('No article/main content found')
    for node in article.select('script, style, nav, button, .feedback, .navigation-links'):
        node.decompose()
    # Kotlin WebHelp renders code as div.code-block, not HTML pre.
    for code in article.select('div.code-block'):
        code.name = 'pre'
    # Flatten list wrappers, while preserving nested paragraphs and code blocks.
    for item in article.find_all('li'):
        if item.find(['p', 'pre', 'ul', 'ol', 'table']):
            item.name = 'div'
        else:
            item.name = 'p'
            item.insert(0, '- ')
    for listing in article.find_all(['ul', 'ol']):
        listing.name = 'div'
    blocks, section, anchors = [], [], []
    tags = {'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'pre', 'ul', 'ol', 'table'}
    for node in article.find_all(list(tags)):
        if any(parent.name in tags for parent in node.parents if parent is not article):
            continue
        kind = node.name
        if kind == 'pre':
            text = node.get_text().strip('\n')
        elif kind == 'table':
            text = '\n'.join(' | '.join(cell.get_text(' ', strip=True) for cell in row.find_all(['td', 'th'])) for row in node.find_all('tr'))
        elif kind in ('ul', 'ol'):
            text = '\n'.join('- ' + item.get_text(' ', strip=True) for item in node.find_all('li', recursive=False))
        else:
            text = node.get_text(' ', strip=True)
        if not text:
            continue
        if kind.startswith('h'):
            level = int(kind[1])
            section = section[:level - 1] + [text]
            anchors = anchors[:level - 1] + [node.get('id') or (node.parent.get('id') if node.parent else '') or '']
        blocks.append({'kind': kind, 'text': text, 'section': list(section), 'anchor': next((a for a in reversed(anchors) if a), '')})
    offset = 0
    for block in blocks:
        block['start'] = offset
        block['end'] = offset + len(block['text'])
        offset = block['end'] + 2
    text = '\n\n'.join(b['text'] for b in blocks)
    if not text:
        raise ValueError('Empty extracted article')
    return text, blocks


def fetch(manifest_path, root):
    root.mkdir(parents=True, exist_ok=True)
    manifest = json.loads(Path(manifest_path).read_text())
    checksums = []
    for entry in manifest['documents']:
        req = urllib.request.Request(entry['url'], headers={'User-Agent': 'kotlin-index/0.1'})
        with urllib.request.urlopen(req, timeout=90) as response:
            html = response.read().decode('utf-8')
        text, blocks = extract(html)
        document = dict(entry, text=text, blocks=blocks, retrieved_at=time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()), html_hash=digest(html), text_hash=digest(text))
        (root / (entry['id'] + '.html')).write_text(html)
        (root / (entry['id'] + '.txt')).write_text(text)
        (root / (entry['id'] + '.json')).write_text(json.dumps(document, ensure_ascii=False, indent=2))
        checksums.append({k: document[k] for k in ('id', 'url', 'retrieved_at', 'html_hash', 'text_hash')})
        print('Fetched', entry['id'], len(text), 'characters', flush=True)
    return checksums


class Embedder:
    def __init__(self, url, model, cache_path):
        self.url, self.model = url.rstrip('/'), model
        try:
            tags = request_json(self.url + '/api/tags')['models']
        except urllib.error.URLError as exc:
            raise ValueError(
                'Cannot connect to Ollama at ' + self.url +
                '. Start it with `ollama serve` or, on macOS, '
                '`brew services start ollama`, then retry. Details: ' + str(exc)
            ) from exc
        item = next((m for m in tags if m['name'] == model or m.get('model') == model), None)
        if item is None:
            raise ValueError('Model not installed: run ollama pull ' + model)
        self.identity = {'model': model, 'digest': item['digest'], 'ollama': request_json(self.url + '/api/version')['version'], 'format': 'title: TITLE | text: BODY; task: search result | query: QUERY'}
        cache_path.parent.mkdir(parents=True, exist_ok=True)
        self.cache = sqlite3.connect(str(cache_path))
        self.cache.execute('CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, vector BLOB)')
        self.calls = 0

    def embed(self, texts):
        results = []
        for text in texts:
            key = digest(json.dumps(self.identity, sort_keys=True) + text)
            row = self.cache.execute('SELECT vector FROM cache WHERE key=?', (key,)).fetchone()
            if row:
                vector = np.frombuffer(row[0], dtype='<f4').copy()
            else:
                try:
                    result = request_json(self.url + '/api/embed', {'model': self.model, 'input': [text], 'truncate': False})
                except Exception as exc:
                    raise ValueError('Embedding failed; verify Ollama/model and reduce chunk size if input exceeds context: ' + str(exc)) from exc
                if len(result.get('embeddings', [])) != 1:
                    raise ValueError('Invalid embedding count')
                vector = np.asarray(result['embeddings'][0], dtype='<f4')
                self.calls += 1
                if vector.ndim != 1 or not vector.size or not np.isfinite(vector).all() or np.linalg.norm(vector) == 0:
                    raise ValueError('Invalid embedding vector')
                vector /= np.linalg.norm(vector)
                self.cache.execute('INSERT OR REPLACE INTO cache VALUES (?, ?)', (key, vector.tobytes()))
                self.cache.commit()
            results.append(vector)
        if results and len({len(v) for v in results}) != 1:
            raise ValueError('Embedding dimension mismatch')
        return results


def document_input(title, text):
    return 'title: ' + title + ' | text: ' + text


def query_input(text):
    return 'task: search result | query: ' + text


def chunk(doc, strategy, embedder, config):
    text = doc['text']
    spans = []
    if strategy == 'fixed':
        size, overlap = config['fixed_size'], config['overlap']
        if size <= 0 or overlap < 0 or overlap >= size:
            raise ValueError('Require fixed size > overlap >= 0')
        start = 0
        while start < len(text):
            end = min(start + size, len(text))
            spans.append((start, end))
            if end == len(text):
                break
            start = end - overlap
    else:
        minimum, maximum = config['minimum'], config['maximum']
        if not 0 < minimum <= maximum:
            raise ValueError('Require 0 < semantic minimum <= maximum')
        units = []
        for block in doc['blocks']:
            start = block['start']
            while start < block['end']:
                end = min(start + maximum, block['end'])
                if end < block['end']:
                    newline = text.rfind('\n', start, end)
                    if newline > start:
                        end = newline + 1
                units.append(dict(block, start=start, end=end))
                start = end
        vectors = embedder.embed([document_input(doc['title'], text[u['start']:u['end']]) for u in units])
        start, end, previous = None, None, None
        for unit, vector in zip(units, vectors):
            if start is not None:
                heading = unit['kind'].startswith('h')
                too_big = unit['end'] - start > maximum
                topic = end - start >= minimum and float(np.dot(previous, vector)) < config['threshold']
                # Code belongs to its preceding explanation when it fits.
                if heading or too_big or (topic and unit['kind'] != 'pre'):
                    spans.append((start, end))
                    start = None
            if start is None:
                start = unit['start']
            end, previous = unit['end'], vector
        if start is not None:
            spans.append((start, end))
    chunks = []
    for start, end in spans:
        intersect = [b for b in doc['blocks'] if b['end'] > start and b['start'] < end]
        sections = list(dict.fromkeys(' > '.join(b['section']) for b in intersect))
        broken = any(b['kind'] == 'pre' and (start > b['start'] or end < b['end']) for b in intersect)
        content = text[start:end]
        chunks.append({'id': digest(doc['id'] + doc['text_hash'] + strategy + json.dumps(config, sort_keys=True) + str((start, end))), 'document_id': doc['id'], 'strategy': strategy, 'text': content, 'start': start, 'end': end, 'line_start': text.count('\n', 0, start) + 1, 'line_end': text.count('\n', 0, end) + 1, 'sections': sections, 'anchor': intersect[0]['anchor'] if intersect else '', 'split_code': broken})
    return chunks


def load_documents(manifest_path, root):
    manifest = json.loads(Path(manifest_path).read_text())
    documents = []
    for entry in manifest['documents']:
        document = json.loads((root / (entry['id'] + '.json')).read_text())
        if document['url'] != entry['url'] or document['text_hash'] != digest(document['text']):
            raise ValueError('Snapshot mismatch: ' + entry['id'])
        documents.append(document)
    return documents


def corpus_stats(documents):
    seen, words, code = set(), 0, 0
    for doc in documents:
        for block in doc['blocks']:
            key = digest(block['text'])
            if key in seen:
                continue
            seen.add(key)
            if block['kind'] == 'pre':
                code += len(block['text'])
            else:
                words += len(re.findall(r'\b[\w]+\b', block['text']))
    return {'documents': len(documents), 'unique_prose_words': words, 'equivalent_pages': words / 500, 'code_characters': code}


def build(documents, db_path, embedder, strategies, config):
    stats = corpus_stats(documents)
    if stats['unique_prose_words'] < 15000:
        raise ValueError('Corpus below 15,000 unique prose words: ' + str(stats))
    db_path.parent.mkdir(parents=True, exist_ok=True)
    temporary = db_path.with_suffix('.building.sqlite')
    if temporary.exists():
        temporary.unlink()
    db = sqlite3.connect(str(temporary))
    db.executescript('CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT); CREATE TABLE documents (id TEXT PRIMARY KEY, data TEXT); CREATE TABLE chunks (id TEXT PRIMARY KEY, document_id TEXT, strategy TEXT, data TEXT, vector BLOB);')
    timings, dimensions = {}, None
    try:
        for document in documents:
            db.execute('INSERT INTO documents VALUES (?,?)', (document['id'], json.dumps(document)))
        for strategy in strategies:
            start, calls = time.monotonic(), embedder.calls
            count = 0
            for document in documents:
                chunks = chunk(document, strategy, embedder, config)
                vectors = embedder.embed([document_input(document['title'] + ' > ' + ' / '.join(c['sections']), c['text']) for c in chunks])
                for c, vector in zip(chunks, vectors):
                    if dimensions is not None and len(vector) != dimensions:
                        raise ValueError('Embedding dimension mismatch')
                    dimensions = len(vector)
                    db.execute('INSERT INTO chunks VALUES (?,?,?,?,?)', (c['id'], document['id'], strategy, json.dumps(c), vector.tobytes()))
                count += len(chunks)
                print(strategy, document['id'], len(chunks), 'chunks', flush=True)
            timings[strategy] = {'seconds': time.monotonic() - start, 'embedding_calls': embedder.calls - calls, 'chunks': count}
        metadata = {'schema_version': 1, 'embedding': embedder.identity, 'dimensions': dimensions, 'config': config, 'corpus': stats, 'build': timings}
        for key, value in metadata.items():
            db.execute('INSERT INTO metadata VALUES (?,?)', (key, json.dumps(value)))
        db.commit()
        db.close()
        temporary.replace(db_path)
        return metadata
    except BaseException:
        db.close()
        raise


def read_index(path):
    db = sqlite3.connect('file:' + str(path.resolve()) + '?mode=ro', uri=True)
    meta = {k: json.loads(v) for k, v in db.execute('SELECT key,value FROM metadata')}
    docs = {k: json.loads(v) for k, v in db.execute('SELECT id,data FROM documents')}
    rows = [(json.loads(data), np.frombuffer(vector, dtype='<f4')) for data, vector in db.execute('SELECT data,vector FROM chunks ORDER BY id')]
    db.close()
    return meta, docs, rows


def rank(rows, vector, strategy, platforms=None):
    selected = [(c, v) for c, v in rows if c['strategy'] == strategy and (platforms is None or c['document_id'] in platforms)]
    if not selected:
        return []
    matrix = np.stack([v for _, v in selected])
    if matrix.shape[1] != len(vector):
        raise ValueError('Query embedding dimension mismatch')
    scores = matrix @ vector
    return [dict(selected[i][0], score=float(scores[i])) for i in np.argsort(-scores, kind='stable')]


def compatible(meta, embedder):
    for key in ('model', 'digest', 'format'):
        if meta['embedding'][key] != embedder.identity[key]:
            raise ValueError('Index embedding model/configuration mismatch; rebuild index')
