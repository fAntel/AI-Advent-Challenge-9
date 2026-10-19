import json
import tempfile
import threading
import unittest
import urllib.error
import urllib.request
from http.server import ThreadingHTTPServer
from pathlib import Path

from mattermost_simulator import BOT, CHANNEL, REVIEWER, Simulator, handler


class SimulatorTests(unittest.TestCase):
    def test_thread_reactions_auth_and_persistence(self):
        with tempfile.TemporaryDirectory() as directory:
            sim = Simulator(Path(directory))
            server = ThreadingHTTPServer(('127.0.0.1', 0), handler(sim))
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            base = f'http://127.0.0.1:{server.server_port}/api/v4'

            def request(path, payload=None, token='simulated-reviewer-token', method=None):
                data = None if payload is None else json.dumps(payload).encode()
                req = urllib.request.Request(base + path, data=data, method=method,
                        headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
                with urllib.request.urlopen(req) as response:
                    return json.load(response)

            try:
                self.assertEqual(request('/users/me')['id'], REVIEWER)
                self.assertEqual(request('/users/me', token='simulated-bot-token')['id'], BOT)
                with self.assertRaises(urllib.error.HTTPError):
                    request('/users/me', token='wrong')
                root = request('/posts', {'channel_id': CHANNEL, 'message': 'problem'})
                reply = request('/posts', {'channel_id': CHANNEL, 'root_id': root['id'],
                                'message': 'analysis', 'props': {'support_publication': 'stable'}},
                                token='simulated-bot-token')
                posts = request('/posts/' + root['id'] + '/thread')['posts']
                self.assertEqual(len(posts), 2)
                self.assertEqual(posts[reply['id']]['props']['support_publication'], 'stable')
                request('/reactions', {'user_id': REVIEWER, 'post_id': reply['id'], 'emoji_name': 'white_check_mark'})
                self.assertEqual(len(request('/posts/' + reply['id'] + '/reactions')), 1)
                request('/users/' + REVIEWER + '/posts/' + reply['id'] + '/reactions/white_check_mark', method='DELETE')
                self.assertEqual(request('/posts/' + reply['id'] + '/reactions'), [])
                self.assertEqual(len(Simulator(Path(directory)).data['posts']), 2)
            finally:
                server.shutdown()
                server.server_close()
                thread.join()


if __name__ == '__main__':
    unittest.main()
