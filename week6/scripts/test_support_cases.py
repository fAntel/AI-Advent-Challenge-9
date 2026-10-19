import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import repo_index as r


class Fake:
    identity = {'model': 'fake', 'digest': 'fixed'}

    def embed(self, text):
        return r.normalize([1, text.lower().count('email'), text.lower().count('lowercase')])


class SupportCaseIndexTests(unittest.TestCase):
    def test_virtual_confirmed_corpus_retrieval_and_revocation(self):
        text = '# Confirmed case\n\nEmail lookup needs lowercase input.\n'
        docs = [{'path': 'http://localhost:8065/_redirect/pl/case1',
                 'text': text, 'hash': r.digest(text)}]
        with tempfile.TemporaryDirectory() as directory:
            db = Path(directory) / 'cases.sqlite'
            with patch.object(r, 'CORPUS_SNAPSHOT', docs):
                r.build(directory, db, Fake())
                results = r.search(directory, db, Fake(), 'lowercase email')
                self.assertEqual(results[0]['source'], docs[0]['path'])
                self.assertEqual(results[0]['text'], text)
            # Removing the confirmed case makes the old index unusable.
            with patch.object(r, 'CORPUS_SNAPSHOT', []):
                self.assertTrue(r.status(directory, db)['stale'])
                with self.assertRaisesRegex(ValueError, 'stale'):
                    r.search(directory, db, Fake(), 'email')


if __name__ == '__main__':
    unittest.main()
