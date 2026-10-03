import json
import tempfile
import unittest
import urllib.error
from pathlib import Path
from unittest.mock import patch

import numpy as np

from kotlin_index.core import Embedder, build, chunk, compatible, digest, extract, rank, read_index
from kotlin_index.evaluate import metrics, targets, union_length

CONFIG = dict(fixed_size=80, overlap=10, minimum=40, maximum=120, threshold=0.65)


class FakeEmbedder:
    identity = dict(model='fake', digest='abc', format='test')
    calls = 0

    def embed(self, texts):
        self.calls += len(texts)
        return [np.array([1., 0.], dtype='<f4') for _ in texts]


def document(html):
    text, blocks = extract(html)
    return dict(id='doc', title='Test', text=text, blocks=blocks, text_hash=digest(text), url='https://example.com', platforms=['common'])


class IndexTests(unittest.TestCase):
    def test_extraction(self):
        doc = document('<nav>outside</nav><article><h1 id="title">Title</h1><h2 id="usage">Usage</h2><p>Use <code>x</code>.</p><ul><li>one</li><li>two</li></ul><div class="code-block">fun x() {\n  println(1)\n}</div><table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table><button>Copy</button></article>')
        self.assertNotIn('outside', doc['text'])
        self.assertNotIn('Copy', doc['text'])
        self.assertIn('  println(1)', doc['text'])
        self.assertIn('A | B', doc['text'])
        self.assertEqual(doc['blocks'][-1]['section'], ['Title', 'Usage'])
        self.assertEqual(doc['blocks'][-1]['anchor'], 'usage')
        for block in doc['blocks']:
            self.assertEqual(doc['text'][block['start']:block['end']], block['text'])

    def test_nested_list_code(self):
        doc = document('<article><h1>Example</h1><ol><li><p>First step</p><div class="code-block">fun x() {\n  run()\n}</div></li></ol></article>')
        code = [b for b in doc['blocks'] if b['kind'] == 'pre']
        self.assertEqual(len(code), 1)
        self.assertIn('  run()', code[0]['text'])

    def test_fixed_coverage_overlap_and_ids(self):
        doc = document('<article><p>' + 'abcdef ' * 60 + '</p></article>')
        chunks = chunk(doc, 'fixed', None, CONFIG)
        self.assertEqual(chunks, chunk(doc, 'fixed', None, CONFIG))
        self.assertEqual(chunks[0]['start'], 0)
        self.assertEqual(chunks[-1]['end'], len(doc['text']))
        for left, right in zip(chunks, chunks[1:]):
            self.assertEqual(left['end'] - right['start'], 10)
        self.assertEqual(len({c['id'] for c in chunks}), len(chunks))

    def test_semantic_code_preservation_and_oversize(self):
        doc = document('<article><h1>Title</h1><p>Explanation.</p><pre>fun x() {\n  println(1)\n}</pre><h2>Other</h2><pre>' + '  println(2)\n' * 30 + '</pre></article>')
        chunks = chunk(doc, 'semantic', FakeEmbedder(), CONFIG)
        self.assertIn('Explanation.', chunks[0]['text'])
        self.assertIn('println(1)', chunks[0]['text'])
        self.assertFalse(chunks[0]['split_code'])
        self.assertTrue(any(c['split_code'] for c in chunks))
        self.assertTrue(all(len(c['text']) <= 120 for c in chunks))
        for block in doc['blocks']:
            self.assertEqual(union_length([(max(c['start'], block['start']), min(c['end'], block['end'])) for c in chunks if min(c['end'], block['end']) > max(c['start'], block['start'])]), len(block['text']))

    def test_semantic_topic_boundary(self):
        doc = document('<article><p>' + 'a ' * 30 + '</p><p>' + 'b ' * 20 + '</p></article>')
        model = FakeEmbedder()
        model.embed = lambda texts: [np.array([1., 0.]), np.array([0., 1.])]
        self.assertEqual(len(chunk(doc, 'semantic', model, CONFIG)), 2)

    def test_metrics_deduplicate_and_budget(self):
        rows = [dict(document_id='doc', start=0, end=80, text='x'*80), dict(document_id='doc', start=20, end=100, text='x'*80)]
        result = metrics(rows, [('doc', 0, 100)], budget=160)
        self.assertEqual(result['coverage_at_6000_chars'], 1)
        self.assertEqual(result['hit_at_5'], 1)
        self.assertEqual(metrics(rows, [('doc', 0, 100)], budget=50)['coverage_at_6000_chars'], 0.5)
        self.assertIsNone(metrics(rows, []))
        self.assertEqual(metrics(rows, [('other', 0, 100)])['reciprocal_rank'], 0)

    def test_labels_and_ranking(self):
        doc = document('<article><h1>Title</h1><p>hello</p></article>')
        self.assertTrue(targets({'relevant': [{'document_id': 'doc', 'section': 'Title'}]}, {'doc': doc}))
        with self.assertRaises(ValueError):
            targets({'relevant': [{'document_id': 'doc', 'section': 'Missing'}]}, {'doc': doc})
        rows = [(dict(id='a', strategy='fixed', document_id='doc'), np.array([1.,0.])), (dict(id='b', strategy='fixed', document_id='doc'), np.array([0.,1.]))]
        self.assertEqual(rank(rows, np.array([0.,1.]), 'fixed')[0]['id'], 'b')
        self.assertEqual(rank(rows, np.array([0.,1.]), 'semantic'), [])
        with self.assertRaises(ValueError):
            rank(rows, np.array([0.,1.,0.]), 'fixed')

    def test_database_roundtrip_and_failure_preserves_index(self):
        doc = document('<article><h1>Title</h1><p>some content</p></article>')
        with tempfile.TemporaryDirectory() as directory, patch('kotlin_index.core.corpus_stats', return_value={'unique_prose_words': 15000}):
            path = Path(directory) / 'index.sqlite'
            build([doc], path, FakeEmbedder(), ['fixed'], CONFIG)
            meta, docs, rows = read_index(path)
            self.assertEqual(docs['doc'], doc)
            self.assertEqual(len(rows), 1)
            self.assertEqual(meta['dimensions'], 2)
            before = path.read_bytes()
            model = FakeEmbedder()
            model.embed = lambda _: (_ for _ in ()).throw(ValueError('failure'))
            with self.assertRaises(ValueError):
                build([doc], path, model, ['fixed'], CONFIG)
            self.assertEqual(path.read_bytes(), before)
            compatible(meta, FakeEmbedder())
            meta['embedding']['digest'] = 'different'
            with self.assertRaises(ValueError):
                compatible(meta, FakeEmbedder())

    def test_unavailable_ollama_has_actionable_error(self):
        with tempfile.TemporaryDirectory() as directory, patch(
            'kotlin_index.core.request_json',
            side_effect=urllib.error.URLError('Connection refused'),
        ):
            with self.assertRaisesRegex(ValueError, 'brew services start ollama'):
                Embedder('http://localhost:11434', 'fake', Path(directory) / 'cache.sqlite')

    def test_embedding_validation_and_cache(self):
        def response(url, body=None):
            if url.endswith('/api/tags'):
                return {'models': [{'name': 'fake', 'digest': 'abc'}]}
            if url.endswith('/api/version'):
                return {'version': 'test'}
            self.assertFalse(body['truncate'])
            return {'embeddings': [[3.,4.]]}
        with tempfile.TemporaryDirectory() as directory, patch('kotlin_index.core.request_json', side_effect=response):
            model = Embedder('http://localhost', 'fake', Path(directory) / 'cache.sqlite')
            np.testing.assert_allclose(model.embed(['test'])[0], [0.6,0.8])
            model.embed(['test'])
            self.assertEqual(model.calls, 1)
            with patch('kotlin_index.core.request_json', return_value={'embeddings': [[0.,0.]]}):
                with self.assertRaises(ValueError):
                    model.embed(['new'])
            model.cache.close()


if __name__ == '__main__':
    unittest.main()
