#!/usr/bin/env python3
"""Capture two goal-driven file scenarios using freshly rebuilt Week6 binaries."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', default='results/task3-demo.json')
    args = parser.parse_args()
    base = Path(__file__).resolve().parents[1]
    output = Path(args.output).resolve()
    if output.exists():
        raise SystemExit('Refusing to overwrite ' + str(output))
    binaries = [base / name for name in ('advent-agent', 'advent-agentd', 'git-mcp')]
    newest_source = max(p.stat().st_mtime for p in list(base.glob('*.go')) + list((base / 'cmd').rglob('*.go')) + [base / 'go.mod'])
    if any(not p.exists() or p.stat().st_mtime < newest_source for p in binaries):
        raise SystemExit('Binaries are missing/stale; run make test and make build first.')
    report = {'at': datetime.now(timezone.utc).isoformat(), 'artifacts': {str(p): digest(p) for p in binaries}, 'steps': [], 'passed': False}
    output.parent.mkdir(parents=True, exist_ok=True)
    # Short paths keep the private Unix socket below macOS's socket-path limit.
    with tempfile.TemporaryDirectory(prefix='w6-files-', dir='/tmp') as temporary:
        temporary = Path(temporary).resolve()
        project = temporary / 'project'
        shutil.copytree(base / 'testdata/task3', project)
        subprocess.run(['git', 'init', '-q', str(project)], check=True)
        outside = temporary / 'outside.txt'
        outside.write_text('outside sentinel\n')
        sentinels = {outside: digest(outside), project / '.git/config': digest(project / '.git/config')}
        sources = {p.name: digest(p) for p in project.glob('*.go')}
        report['before'] = {p.name: p.read_text() for p in project.iterdir() if p.is_file()}
        state = temporary / 'state'
        config = temporary / 'config.toml'
        config.write_text('state = ' + json.dumps(str(state)) + '\n')
        session = None

        def save():
            if session:
                path = state / 'sessions' / (session + '.json')
                if path.exists():
                    report['session'] = json.loads(path.read_text())
            report['after'] = {p.name: p.read_text() for p in project.iterdir() if p.is_file()}
            output.write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')

        def run(name, command):
            nonlocal session
            result = subprocess.run([str(x) for x in command], cwd=project, text=True, capture_output=True, timeout=600)
            report['steps'].append({'name': name, 'command': [str(x) for x in command], 'returncode': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr})
            match = re.search(r'Session: ([0-9a-f]{32})', result.stdout)
            if match:
                session = match[1]
            save()
            print(name + ': ' + str(result.returncode), flush=True)
            if result.returncode:
                raise RuntimeError('Failed ' + name + '; see ' + str(output))

        cli = [base / 'advent-agent', '--config', config]
        try:
            run('usage search', cli + ['-p', 'Find every definition and call of the catalog lookup API. Report file paths and line numbers.'])
            run('documentation update', cli + ['-p', 'Update the project README to match the current implementation, including lookup behavior and its callers. Save the changes.', 'resume', session])
            operations = report['session']['operations']
            docs = (project / 'README.md').read_text()
            read_paths = {json.loads(t['arguments']).get('path') for op in operations for t in op.get('tools', []) if t['name'] == 'read_file' and not t.get('error')}
            report['checks'] = {
                'both_answers': all(op.get('answer', {}).get('status') == 'answered' and not op.get('error') for op in operations),
                'usage_search': any(t['name'] == 'search_files' and not t.get('error') and 'catalog.go' in t.get('result', '') and 'service.go' in t.get('result', '') for t in operations[0].get('tools', [])),
                'three_files_read': {'catalog.go', 'service.go', 'README.md'} <= read_paths,
                'documentation_saved': any(c['path'] == 'README.md' and c['status'] == 'saved' and c['diff'] for c in operations[1].get('changes', [])),
                'documented_behavior': all(word in docs for word in ('unknown', 'Featured', 'Missing', '42')),
                'sources_unchanged': all(digest(project / name) == value for name, value in sources.items()),
                'boundaries_unchanged': all(digest(p) == value for p, value in sentinels.items()),
            }
            report['passed'] = all(report['checks'].values())
            if not report['passed']:
                raise RuntimeError('Acceptance checks failed; see ' + str(output))
        finally:
            save()
            if (state / 'agent.sock').exists():
                subprocess.run([str(base / 'advent-agentd'), '--config', str(config), 'stop'], capture_output=True, timeout=10)
                for _ in range(100):
                    if not (state / 'agent.sock').exists():
                        break
                    time.sleep(.05)
    print('Saved ' + str(output))


if __name__ == '__main__':
    main()
