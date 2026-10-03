import unittest
from scripts.render_rag_review import answer_hash, render

class ReviewTests(unittest.TestCase):
    def test_changed_answer_cannot_inherit_judgment(self):
        answer = {'question_id': 'q', 'mode': 'plain', 'answer': 'old', 'call': {}}
        suite = {'questions': [{'id': 'q', 'query': 'question', 'source_review': {'excerpt': 'source'}, 'expected_facts': []}]}
        review = {'assessments': [{'question_id': 'q', 'mode': 'plain', 'answer_sha256': answer_hash(answer), 'facts': [], 'unsupported_claims': 'none', 'citations': 'n/a'}]}
        run = {'index_sha256': 'hash', 'settings': {'model': 'deepseek-v4-flash', 'reasoning': 'low', 'rag_strategy': 'semantic', 'rag_limit': 5, 'rag_budget': 6000}, 'answers': [answer]}
        self.assertIn('Review source:', render(run, review, suite))
        answer['answer'] = 'new'
        result = render(run, review, suite)
        self.assertIn('**UNREVIEWED**', result)
        self.assertIn('`deepseek-v4-flash`, low reasoning', result)
        self.assertNotIn('deepseek-v4-pro', result)
        self.assertNotIn('Review source:', result)
