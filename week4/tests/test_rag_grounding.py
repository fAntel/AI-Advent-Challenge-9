import copy
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from render_rag_grounding import answer_hash, evidence_checks, render


class GroundingReportTest(unittest.TestCase):
    def fixture(self):
        a = {'question_id': 'q', 'answer': "I don't know. Which version?", 'answer_skipped': True,
             'result': {'status': 'unknown', 'answer': '', 'sources': [], 'quotes': [], 'clarification': 'Which version?'},
             'evidence': {'chunks': []}, 'call': {}}
        run = {'answers': [a], 'preparations': {}, 'settings': {'model': 'flash', 'reasoning': 'none', 'rag_threshold': 2}, 'modes': [{'rewrite': False, 'rerank': False}], 'index_sha256': 'index', 'suite_sha256': 'suite'}
        suite = {'questions': [{'id': 'q', 'query': 'Future version?', 'unanswerable': True}]}
        review = {'assessments': [{'question_id': 'q', 'answer_sha256': answer_hash(a),
                                  'meaning_matches_quotes': 'N/A', 'appropriate_unknown': True, 'notes': 'Correct refusal.'}]}
        return run, review, suite

    def test_unknown_has_no_fictitious_generation_call(self):
        run, review, suite = self.fixture()
        output = render(run, review, suite)
        self.assertIn('Recorded API attempts: 0', output)
        self.assertIn('Appropriate unknown controls: 1/1', output)
        self.assertIn('| N/A | N/A | N/A | N/A |', output)
        self.assertTrue(evidence_checks(run['answers'][0])['unknown_valid'])

    def test_stale_missing_or_incomplete_reviews(self):
        for change in ('stale', 'missing', 'incomplete'):
            run, review, suite = self.fixture()
            if change == 'stale':
                run['answers'][0]['answer'] += ' changed'
            elif change == 'missing':
                review = {}
            else:
                suite['questions'].append({'id': 'other', 'query': 'Another?'})
            output = render(run, review, suite)
            self.assertIn('UNREVIEWED', output)
            self.assertNotIn('Appropriate unknown controls:', output)

    def test_quote_and_source_checks_use_saved_chunks(self):
        run, _, _ = self.fixture()
        a = copy.deepcopy(run['answers'][0])
        chunk = {'id': 'c', 'text': 'Exact αβ fragment.', 'source': 'https://example.org', 'title': 'Guide', 'sections': ['Section']}
        a['evidence']['chunks'] = [chunk]
        a['result'] = {'status': 'answered', 'answer': 'Fact [1].', 'clarification': '',
                       'quotes': [{'chunk_id': 'c', 'text': 'αβ fragment'}],
                       'sources': [{'chunk_id': 'c', 'source': chunk['source'], 'title': chunk['title'], 'sections': chunk['sections']}]}
        self.assertTrue(evidence_checks(a)['exact'])
        a['result']['quotes'][0]['text'] = 'made up'
        a['result']['sources'][0]['source'] = 'https://invented.example'
        self.assertFalse(evidence_checks(a)['exact'])
        self.assertEqual(evidence_checks(a)['sources'], 'NO')
