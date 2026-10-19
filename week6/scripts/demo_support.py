#!/usr/bin/env python3
"""Capture a real local Mattermost + DeepSeek support demo after test/build.

Run mattermost_simulator.py first with --simulated. This script posts fixture
escalations and a reviewer confirmation to that isolated local instance only.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

import support_demo_common as environment
from support_demo_common import ROOT, api


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True)
    parser.add_argument('--simulated', action='store_true', help='use the local API simulator instead of Mattermost')
    parser.add_argument('--directory', type=Path, help='demo profile directory')
    args = parser.parse_args()
    output = Path(args.output)
    if output.exists():
        raise RuntimeError('Refusing to overwrite demo report')
    directory = args.directory or ROOT / ('data/support-simulator' if args.simulated else 'data/mattermost-demo')
    STATE = directory.resolve() / 'state'
    auth = json.loads((STATE / 'credentials.json').read_text())
    if args.simulated:
        if auth.get('transport') != 'simulated' or not auth['url'].startswith('http://127.0.0.1:'):
            raise RuntimeError('Expected a local simulated transport profile')
        environment.URL = auth['url']
    elif auth['url'] != environment.URL:
        raise RuntimeError('Demo only supports its isolated loopback server')
    config = STATE.parent / 'support.toml'
    env = dict(os.environ, MATTERMOST_TOKEN=auth['bot_token'])
    bot = ROOT / 'support-bot'
    config_command = [str(bot), '--config', str(config), '--once']
    report = {'transport': 'simulated Mattermost REST API' if args.simulated else 'local Mattermost',
              'artifact': str(bot), 'artifact_sha256': digest(bot),
              'mcp_sha256': digest(ROOT / 'mattermost-mcp'), 'scenarios': [], 'checks': {}}
    fixture = ROOT / 'testdata/support/catalog/accounts.py'
    before = digest(fixture)

    def cycle():
        subprocess.run(config_command, env=env, check=True, timeout=400)

    def state():
        return json.loads((STATE / 'support.json').read_text())

    def scenario(name, question, root=''):
        post, _ = api('POST', '/posts', {'channel_id': auth['channel_id'], 'root_id': root, 'message': question}, auth['admin_token'])
        cycle()
        job = next(j for j in state()['jobs'] if j['id'] == post['id'])
        deadline = time.monotonic() + 900
        while job['status'] not in ('sent', 'cancelled') and time.monotonic() < deadline:
            wait = max(1, (job.get('next_attempt', 0) - time.time() * 1000) / 1000)
            time.sleep(min(wait, 30))
            cycle()
            job = next(j for j in state()['jobs'] if j['id'] == post['id'])
        report['scenarios'].append({'name': name, 'question': question, 'job': job})
        if job['status'] != 'sent' or not job['operation'].get('answer'):
            raise RuntimeError(f'{name}: analysis not successfully published')
        return job

    try:
        # Establish new-message cursors before posting fixture questions.
        cycle()
        scenario('documentation', 'In the week6 project, is it expected that ordinary file questions work without an Ollama index? Please check the documentation.')
        bug = scenario('workaround', 'In the catalog demo, Alex is registered as alex@example.test. Looking up Alex@Example.Test returns no account. Explain why using the code and suggest a workaround without changing files.')
        scenario('insufficient_evidence', 'A user says the screen turned purple yesterday. We have no app version, screenshots, steps, or logs. Can you determine why? Do not guess.')
        api('POST', '/reactions', {'user_id': auth['admin_id'], 'post_id': bug['post_id'], 'emoji_name': 'white_check_mark'}, auth['admin_token'])
        cycle()
        report['checks']['human_confirmation_saved'] = bug['post_id'] in state()['cases']
        repeated = scenario('similar_confirmed_case', 'In the catalog demo, account lookup fails when I type ALEX@EXAMPLE.TEST. Is there a previous confirmed case, and does its workaround still apply to the current code?')
        report['checks']['confirmed_case_retrieved'] = any('Human-confirmed support case' in e['text'] for e in repeated['operation']['evidence'])
        report['checks']['current_code_read_for_reuse'] = any(t['name'] == 'read_file' for t in repeated['operation'].get('tools', []))
        # Ordinary conversation produces no job; an explicit mention does.
        before_jobs = len(state()['jobs'])
        api('POST', '/posts', {'channel_id': auth['channel_id'], 'root_id': bug['root_id'], 'message': 'Thanks, we will discuss this internally.'}, auth['admin_token'])
        cycle()
        report['checks']['ordinary_reply_ignored'] = len(state()['jobs']) == before_jobs
        followup = scenario('mentioned_followup', '@week6-support Does the lowercase-input workaround require a code change? Use the current thread context.', bug['root_id'])
        report['checks']['mention_followup_in_same_thread'] = followup['root_id'] == bug['root_id']
        api('DELETE', '/users/' + auth['admin_id'] + '/posts/' + bug['post_id'] + '/reactions/white_check_mark', token=auth['admin_token'])
        cycle()
        report['checks']['confirmation_revoked'] = bug['post_id'] not in state()['cases']
    except Exception as error:
        report['error'] = str(error)
        raise
    finally:
        report['checks']['source_unchanged'] = digest(fixture) == before
        report['checks']['no_write_tools_executed'] = all(
            t['name'] != 'write_file' for s in report['scenarios'] for t in s['job']['operation'].get('tools', []))
        report['completed_at'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
        output.parent.mkdir(parents=True, exist_ok=True)
        with output.open('x') as stream:
            json.dump(report, stream, indent=2, ensure_ascii=False)
            stream.write('\n')
    if not all(report['checks'].values()):
        raise RuntimeError('One or more acceptance checks failed; inspect report')
    print(f'Support demo passed: {output}')


if __name__ == '__main__':
    main()
