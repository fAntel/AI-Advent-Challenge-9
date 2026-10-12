#!/usr/bin/env python3
"""Capture a live grounded answer, unknown response, and daemon restart/resume."""
import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
import tomllib


class UnixConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__('localhost', timeout=10)
        self.path = path
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', default='results/task2/demo.json')
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    output = Path(args.output).resolve()
    if output.exists():
        raise SystemExit('Choose a new output path; existing capture preserved.')
    env = dict(os.environ)
    env.pop('DEEPSEEK_API_KEY', None)
    env.setdefault('XDG_CONFIG_HOME', str(root / 'data/task2-config'))
    env.setdefault('XDG_STATE_HOME', str(root / 'data/task2-state'))
    cfg = tomllib.loads((Path(env['XDG_CONFIG_HOME']) / 'advent-agent/profiles/local-rag/config.toml').read_text())
    sock = Path(cfg['daemon']['socket_path']).expanduser()
    binary, daemon = root / 'advent-agent', root / 'advent-agentd'
    report = {'artifact':str(binary), 'artifact_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),
              'profile':'local-rag', 'cloud_key_passed':False, 'commands':[], 'snapshots':[]}
    output.parent.mkdir(parents=True, exist_ok=True)
    def save():
        output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    def api(path):
        c = UnixConnection(str(sock))
        try:
            c.request('GET', '/v1/' + path, headers={'X-Agent-Profile':'local-rag'})
            response = c.getresponse()
            if response.status != 200:
                raise RuntimeError(f'Daemon HTTP {response.status}')
            return json.loads(response.read())
        finally:
            c.close()
    with tempfile.TemporaryDirectory(prefix='week5-local-rag-') as cwd:
        report['project_directory'] = cwd
        def run(argv, text=''):
            start = time.monotonic()
            result = subprocess.run(list(map(str, argv)), input=text, env=env, cwd=cwd, text=True, capture_output=True, timeout=1200)
            report['commands'].append({'argv':list(map(str, argv)), 'stdin':text, 'stdout':result.stdout,
                                       'stderr':result.stderr, 'exit_code':result.returncode, 'elapsed_seconds':time.monotonic()-start})
            save()
            print(result.stdout, end='', flush=True)
            if result.returncode:
                raise RuntimeError(result.stderr)
            return result
        first = run([binary, '--profile', 'local-rag', '--rag', '-p', 'When did the common Kotlin UUID API become stable? Answer briefly and use one short exact quote.', '--stats'])
        match = re.search(r'Session: ([a-f0-9]+)', first.stderr)
        if not match:
            raise RuntimeError('Missing session ID')
        session = match[1]
        report['session_id'] = session
        def snapshot():
            state = api('sessions/' + session)
            report['snapshots'].append(state)
            save()
            if state['operation']['state'] != 'completed':
                raise RuntimeError('Operation did not complete')
            return state
        snapshot()
        report['capabilities'] = api('capabilities')
        run([binary, '--profile', 'local-rag', 'resume', session], 'What is the internal production API endpoint for my company payroll service?\n/exit\n')
        unknown = snapshot()
        if unknown['operation']['rag_result']['status'] != 'unknown':
            raise RuntimeError('Unsupported question was not unknown')
        run([daemon, '--profile', 'local-rag', 'stop'])
        deadline = time.monotonic() + 10
        while sock.exists() and time.monotonic() < deadline:
            time.sleep(.05)
        run([binary, '--profile', 'local-rag', 'resume', session], 'When did the common Kotlin UUID API become stable? Answer briefly and use one short exact quote.\n/exit\n')
        snapshot()
        report['status'] = 'completed'
        save()
        run([daemon, '--profile', 'local-rag', 'stop'])


if __name__ == '__main__':
    main()
