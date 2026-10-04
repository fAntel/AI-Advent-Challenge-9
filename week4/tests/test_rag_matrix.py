import copy
import unittest
from scripts.render_rag_matrix import answer_hash, matched, render, summarize


class MatrixReviewTests(unittest.TestCase):
    def fixture(self):
        suite = {'questions': [{'id': 'q', 'query': 'out of corpus', 'unanswerable': True, 'expected_facts': []}]}
        prep = {'calls': [{'purpose': 'rag_score', 'call': {'cost_usd': 0.1}}], 'candidates': [], 'retrieval_duration': 100, 'search_query': 'query'}
        run = {'index_sha256': 'fp', 'settings': {'model': 'flash', 'reasoning': 'none', 'rag_candidates': 20, 'rag_limit': 5, 'rag_threshold': 2, 'rag_budget': 6000},
               'modes': [{'name': str(i)} for i in range(8)], 'preparations': {'shared': prep}, 'answers': []}
        review = {'summary': 'Reviewed summary', 'assessments': []}
        for mode in run['modes']:
            a = {'question_id': 'q', 'mode': mode['name'], 'answer': 'insufficient corpus evidence', 'call': {'cost_usd': 0.01}, 'evidence': {'preparation': prep, 'chunks': [], 'decisions': []}}
            run['answers'].append(a)
            review['assessments'].append({'question_id': 'q', 'mode': mode['name'], 'answer_sha256': answer_hash(a), 'facts': [], 'material_error': False, 'citations': 'none', 'honest_insufficiency': True, 'notes': 'Acknowledges missing corpus support.'})
        return run, review, suite

    def test_shared_cost_and_complete_controls(self):
        run, review, suite = self.fixture()
        stats = summarize(run, review, suite)
        self.assertTrue(stats['review_complete'])
        self.assertAlmostEqual(stats['actual_cost_usd'], 0.18)
        self.assertEqual(stats['actual_calls'], 9)
        self.assertEqual(stats['modes']['0']['honest_controls'], 1)
        self.assertIn('Reviewed summary', render(run, review, suite))

    def test_missing_or_stale_review_withholds_summary(self):
        for alteration in ('changed', 'missing', 'duplicate', 'invalid'):
            run, review, suite = self.fixture()
            if alteration == 'changed':
                run['answers'][0]['answer'] = 'changed'
            elif alteration == 'missing':
                run['answers'].pop()
            elif alteration == 'duplicate':
                run['answers'].append(copy.deepcopy(run['answers'][0]))
            else:
                review['assessments'][0].pop('honest_insufficiency')
            rendered = render(run, review, suite)
            self.assertIn('UNREVIEWED', rendered)
            self.assertNotIn('Reviewed summary', rendered)
            self.assertNotIn('## Answer quality', rendered)

    def test_section_paths_and_document_identity(self):
        q = {'relevant': [{'document_id': 'doc', 'section': 'Nested topic'}]}
        self.assertTrue(matched({'document_id': 'doc', 'sections': ['Title > Nested topic > Detail']}, q))
        self.assertFalse(matched({'document_id': 'other', 'sections': ['Title > Nested topic']}, q))
        self.assertFalse(matched({'document_id': 'doc', 'sections': ['Title > Nested topic extension']}, q))
