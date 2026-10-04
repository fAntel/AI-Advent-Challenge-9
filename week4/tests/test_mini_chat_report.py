import copy
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from render_mini_chat import reviewed, turn_hash


class MiniChatReportTests(unittest.TestCase):
    def test_changed_memory_or_evidence_invalidates_review(self):
        turn = {'memory_after': {'goal': 'browser'}, 'evidence': {'chunks': []}}
        review = {'turn_sha256': turn_hash(turn), 'goal_and_constraints': 'yes',
                  'semantic_support': 'N/A', 'appropriate_response': True, 'notes': 'Reviewed.'}
        self.assertTrue(reviewed(turn, review))
        changed = copy.deepcopy(turn)
        changed['memory_after']['goal'] = 'server'
        self.assertFalse(reviewed(changed, review))
        self.assertFalse(reviewed(turn, None))
        self.assertFalse(reviewed(turn, dict(review, semantic_support='')))

    def test_ten_of_twelve_turns_does_not_count_as_complete(self):
        from render_mini_chat import render
        sessions, assessments = [], []
        for dialog in range(2):
            turns = []
            for n in range(10):
                turn = {'id': f'{dialog}-{n}', 'state': 'completed', 'question': 'q',
                        'answer': 'clarification', 'memory_after': {}, 'conflicts': ['clarify']}
                turns.append(turn)
                assessments.append({'turn_id': turn['id'], 'turn_sha256': turn_hash(turn),
                                    'goal_and_constraints': 'yes', 'semantic_support': 'N/A',
                                    'appropriate_response': True, 'notes': 'Reviewed.'})
            sessions.append({'id': str(dialog), 'turns': turns, 'memory': {'goal': {'value': 'goal'}}})
        run = {'suite_sha256': 'same', 'sessions': sessions}
        suite = {'dialogs': [{'questions': ['q'] * 12}, {'questions': ['q'] * 12}]}
        result = render(run, {'assessments': assessments}, suite, 'same')
        self.assertIn('semantic quality totals withheld', result)
        self.assertNotIn('Goal/constraint adherence: 20/20', result)
