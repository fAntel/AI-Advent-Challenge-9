#!/usr/bin/env python3
"""Capture the rebuilt Week6 CLI against live providers; no silent retries."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
from datetime import datetime, timezone


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--output', default='results/task0-demo.json')
    args = p.parse_args()
    base = Path(__file__).resolve().parents[1]
    output = Path(args.output).resolve()
    if output.exists():
        raise SystemExit('Refusing to overwrite ' + str(output))
    output.parent.mkdir(parents=True, exist_ok=True)
    report = {'at': datetime.now(timezone.utc).isoformat(), 'artifacts': {}, 'steps': []}
    for name in ('advent-agent', 'advent-agentd', 'git-mcp'):
        path = base / name
        report['artifacts'][str(path)] = hashlib.sha256(path.read_bytes()).hexdigest()
    def save():
        output.write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')
    def run(name, command, stdin=None):
        completed = subprocess.run([str(x) for x in command], cwd=base, input=stdin, text=True, capture_output=True, timeout=600)
        report['steps'].append({'name': name, 'command': [str(x) for x in command], 'returncode': completed.returncode, 'stdout': completed.stdout, 'stderr': completed.stderr})
        save()
        print(name + ': ' + str(completed.returncode), flush=True)
        if completed.returncode:
            if report.get('session'):
                state_path = base / 'data/state/sessions' / (report['session'] + '.json')
                report['operations'] = json.loads(state_path.read_text())['operations']
                save()
            raise SystemExit('Failed ' + name + '; evidence saved to ' + str(output))
        return completed.stdout
    # Ensure every request reaches the rebuilt sibling daemon, not an older process.
    import time
    socket = base / 'data/state/agent.sock'
    if socket.exists():
        run('initial daemon stop', [base / 'advent-agentd', 'stop'])
        for _ in range(100):
            if not socket.exists():
                break
            time.sleep(.05)
        else:
            raise SystemExit('Week6 daemon did not stop')
    run('index status', [base / 'advent-agent', 'rag', 'status'])
    text = run('bare help', [base / 'advent-agent'], '/help\n/exit\n')
    session = re.search(r'Session: ([0-9a-f]{32})', text)[1]
    report['session'] = session
    questions = [
        ('structure', '/help How are weeks and tasks organized in this repository?'),
        ('earlier week', '/help How can I register and inspect MCP servers in Week3?'),
        ('commit convention', '/help What commit message format should I use for Week6 Task0?'),
        ('branch', 'Which branch am I on?'),
        ('unknown', 'What is this project\'s undocumented production SLA?'),
    ]
    for name, question in questions:
        run(name, [base / 'advent-agent', '-p', question, 'resume', session])
    run('daemon stop', [base / 'advent-agentd', 'stop'])
    # Wait for the owned daemon to release its socket before auto-start/resume.
    import time
    socket = base / 'data/state/agent.sock'
    for _ in range(100):
        if not socket.exists():
            break
        time.sleep(.05)
    run('resume after restart', [base / 'advent-agent', '-p', 'Which branch is checked out now?', 'resume', session])
    state = json.loads((base / 'data/state/sessions' / (session + '.json')).read_text())
    report['operations'] = state['operations']
    report['summary'] = state['summary']
    report['passed'] = all(op.get('answer') and not op.get('error') for op in state['operations'])
    report['unknown_control'] = state['operations'][4]['answer']['status'] == 'unknown'
    report['answerable_controls'] = all(state['operations'][i]['answer']['status'] == 'answered' for i in (0, 1, 2, 3, 5))
    report['passed'] = report['passed'] and report['unknown_control'] and report['answerable_controls']
    save()
    if not report['passed']:
        raise SystemExit('Live acceptance checks failed; inspect ' + str(output))
    print('Saved ' + str(output))

if __name__ == '__main__':
    main()
