import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('verify_private_llm', Path(__file__).with_name('verify-private-llm.py'))
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)


class VerificationTest(unittest.TestCase):
    def test_truncated_or_empty_chats_do_not_pass(self):
        record = {'status': 200, 'response': {'choices': [{
            'finish_reason': 'stop', 'message': {'role': 'assistant', 'content': 'Ready.'}}]}}
        self.assertTrue(verify.completed_chat(record))
        record['response']['choices'][0]['finish_reason'] = 'length'
        self.assertFalse(verify.completed_chat(record))
        record['response']['choices'][0]['finish_reason'] = 'stop'
        record['response']['choices'][0]['message']['content'] = ' '
        self.assertFalse(verify.completed_chat(record))

    def test_transport_and_malformed_responses_fail(self):
        for record in ({'status': 429}, {'status': None}, {'status': 200, 'response': None},
                       {'status': 200, 'response': {'choices': []}}):
            self.assertFalse(verify.completed_chat(record))
            self.assertFalse(verify.context_rejected(record))

    def test_context_error_must_be_specific(self):
        record = {'status': 400, 'response': {'error': {'message': 'request exceeds the available context size'}}}
        self.assertTrue(verify.context_rejected(record))
        for code in (200, 413, 429, 500):
            record['status'] = code
            self.assertFalse(verify.context_rejected(record))
        record['status'] = 400
        record['response']['error']['message'] = 'invalid JSON'
        self.assertFalse(verify.context_rejected(record))


if __name__ == '__main__':
    unittest.main()
