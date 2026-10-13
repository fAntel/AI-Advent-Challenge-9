#!/usr/bin/env python3
"""Capture two VPS chat turns, including daemon restart and session resume."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', default='results/task4/agent-demo.json')
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.pop('DEEPSEEK_API_KEY', None)
    env['XDG_CONFIG_HOME'] = str(root / 'data/task4-config')
    env['XDG_STATE_HOME'] = str(root / 'data/task4-state')
    state = root / 'data/task4-state/advent-agent/vps'
    binary = root / 'advent-agent'
    daemon = root / 'advent-agentd'
    report = {'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
              'artifact': str(binary), 'artifact_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
              'daemon_sha256': hashlib.sha256(daemon.read_bytes()).hexdigest(),
              'cloud_key_passed': False, 'commands': [], 'snapshots': [], 'status': 'running'}
    with output.open('x') as handle:
        json.dump(report, handle, indent=2)

    def save():
        output.write_text(json.dumps(report, indent=2) + '\n')

    def run(argv, text=''):
        print('Running:', ' '.join(map(str, argv)), flush=True)
        start = time.monotonic()
        result = subprocess.run(argv, input=text, text=True, capture_output=True,
                                cwd=root, env=env, timeout=240)
        report['commands'].append({'argv': list(map(str, argv)), 'stdin': text,
                                   'stdout': result.stdout, 'stderr': result.stderr,
                                   'exit_code': result.returncode,
                                   'elapsed_seconds': time.monotonic() - start})
        save()
        print(result.stdout + result.stderr, flush=True)
        if result.returncode:
            raise RuntimeError('Command failed; see capture')
        return result

    def stop():
        run([daemon, '--profile', 'vps', 'stop'])
        deadline = time.monotonic() + 10
        while (state / 'agent.sock').exists() and time.monotonic() < deadline:
            time.sleep(.1)
        if (state / 'agent.sock').exists():
            raise RuntimeError('VPS daemon did not stop')

    try:
        run(['python3', root / 'scripts/install-local-profile.py', '--profile', 'vps'])
        stop()
        first = run([root / 'scripts/vps-chat.sh', '-p',
                     'My name is Mira. Reply with a short greeting.', '--stats'])
        match = re.search(r'Session: ([a-f0-9]+)', first.stderr)
        if not match:
            raise RuntimeError('No session ID')
        session_id = match.group(1)
        report['session_id'] = session_id

        def snapshot():
            session = json.loads((state / 'profiles/vps/sessions' / session_id / 'session.json').read_text())
            report['snapshots'].append(session)
            save()
            if session['operation']['state'] != 'completed' or session['settings']['provider'] != 'llamacpp':
                raise RuntimeError('Turn did not complete on the private provider')
            return session

        snapshot()
        stop()
        # A restarted daemon has no in-memory pacing timestamp.
        time.sleep(11)
        run([root / 'scripts/vps-chat.sh', 'resume', session_id],
            '/model\nWhat is my name? Reply with only my name.\n/stats\n/exit\n')
        session = snapshot()
        answer = session['messages'][-1]['content']
        if not re.search(r'\bMira\b', answer, re.I):
            raise RuntimeError('Follow-up did not recall Mira')
        report['status'] = 'passed'
    except (Exception, KeyboardInterrupt) as error:
        report.update(status='failed', error=f'{type(error).__name__}: {error}')
    finally:
        report['finished_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        save()
    print(f"{report['status']}: {output}")
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
