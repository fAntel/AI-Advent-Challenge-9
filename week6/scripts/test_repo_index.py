import json
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import repo_index as r

class Fake:
    identity = {'model': 'fake', 'digest': 'fixed'}
    def embed(self, text):
        return r.normalize([1, text.lower().count('branch'), text.lower().count('commit')])

class IndexTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        (self.root / '.gitignore').write_text('data/\nignored.md\n')
        (self.root / 'README.md').write_text('# Root\n\nProject branch overview.\n')
        (self.root / 'week6/docs').mkdir(parents=True)
        (self.root / 'week6/docs/commit.md').write_text('# Commit format\n\nWeekN TaskM: description\n')
        (self.root / 'week5/results').mkdir(parents=True)
        (self.root / 'week5/results/review.md').write_text('# Historical result\n\nA measured result.\n')
        (self.root / 'data').mkdir()
        self.db = self.root / 'data/index.sqlite'

    def test_discovery_and_staleness(self):
        (self.root / 'ignored.md').write_text('ignored')
        (self.root / 'data/generated.md').write_text('generated')
        (self.root / 'escape.md').symlink_to('/etc/hosts')
        docs = r.corpus(self.root)
        self.assertEqual(len(docs), 3)
        meta = r.build(self.root, self.db, Fake())
        self.assertEqual(meta['documents'], 3)
        self.assertFalse(r.status(self.root, self.db)['stale'])
        (self.root / 'README.md').write_text('changed')
        with self.assertRaisesRegex(ValueError, 'stale'):
            r.search(self.root, self.db, Fake(), 'branch')

    def test_quotes_lines_and_search(self):
        text = '# Title\n\nIntro\n\n## Details\n\n```go\nhello\n```\n'
        doc = {'path': 'week2/README.md', 'text': text, 'hash': r.digest(text)}
        for c in r.chunks(doc, 10):
            self.assertEqual(c['text'], ''.join(text.splitlines(keepends=True)[c['line_start']-1:c['line_end']]))
            self.assertEqual(c['week'], 'week2')
        r.build(self.root, self.db, Fake())
        result = r.search(self.root, self.db, Fake(), 'branch')
        self.assertEqual(result[0]['source'], 'README.md')
        wrong = Fake()
        wrong.identity = {'model': 'other'}
        with self.assertRaisesRegex(ValueError, 'model changed'):
            r.search(self.root, self.db, wrong, 'branch')

    def test_failed_build_preserves_index(self):
        r.build(self.root, self.db, Fake())
        before = self.db.read_bytes()
        class Failing(Fake):
            def embed(self, text):
                raise ValueError('unavailable')
        with self.assertRaisesRegex(ValueError, 'unavailable'):
            r.build(self.root, self.db, Failing())
        self.assertEqual(before, self.db.read_bytes())

    def test_embedding_cache_and_validation(self):
        calls = []
        def request(self, path, payload=None):
            calls.append(path)
            if path == '/api/tags':
                return {'models': [{'name': 'fake', 'digest': 'one'}]}
            if path == '/api/show':
                return {}
            return {'embeddings': [[3, 4]]}
        with patch.object(r.Embedder, 'request', request):
            e = r.Embedder('http://127.0.0.1:11434', 'fake', self.root / 'cache.sqlite')
            self.assertEqual(e.embed('same'), [.6, .8])
            self.assertEqual(e.embed('same'), [.6, .8])
            self.assertEqual(calls.count('/api/embed'), 1)
            e.cache.close()
        for v in ([], [0, 0], [float('nan')]):
            with self.assertRaises(ValueError):
                r.normalize(v)

    def test_subdirectory_plain_directory_and_git_alias(self):
        docs = r.corpus(self.root / 'week6')
        self.assertEqual([d['path'] for d in docs], ['docs/commit.md'])
        (self.root / '.git/secret.md').write_text('metadata')
        (self.root / 'alias.md').symlink_to('.git/secret.md')
        self.assertNotIn('alias.md', [d['path'] for d in r.corpus(self.root)])
        with tempfile.TemporaryDirectory() as directory:
            plain = Path(directory)
            (plain / 'README.md').write_text('plain')
            self.assertEqual(r.corpus(plain)[0]['text'], 'plain')

    def test_corpus_snapshot_stdin(self):
        r.build(self.root, self.db, Fake())
        snapshot = r.corpus(self.root)
        completed = subprocess.run([
            'python3', '-I', str(Path(r.__file__)), '--corpus-stdin',
            '--root', str(self.root), '--database', str(self.db), 'status'
        ], input=json.dumps(snapshot), text=True, capture_output=True, check=True)
        self.assertFalse(json.loads(completed.stdout)['stale'])

if __name__ == '__main__':
    unittest.main()
