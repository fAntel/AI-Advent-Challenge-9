#!/usr/bin/env python3
"""Verify the VPS chat service through an already-open SSH tunnel; save evidence."""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import re
import statistics
import time
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def completed_chat(record):
    try:
        choice = record['response']['choices'][0]
        return (record['status'] == 200 and choice['finish_reason'] == 'stop'
                and choice['message']['role'] == 'assistant'
                and bool(choice['message']['content'].strip()))
    except (KeyError, IndexError, TypeError, AttributeError):
        return False


def context_rejected(record):
    # A gateway 413, rate limit, generic bad JSON, or HTML error is not proof.
    response = record.get('response')
    if record.get('status') != 400 or not isinstance(response, dict):
        return False
    error = response.get('error')
    if not isinstance(error, dict):
        return False
    message = str(error.get('message', '')).lower()
    return ('context' in message and
            any(word in message for word in ('exceed', 'too long', 'too large', 'larger')))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', default='http://127.0.0.1:18080')
    parser.add_argument('--model', default='private-smollm')
    parser.add_argument('--requests', type=int, default=10)
    parser.add_argument('--output', default='results/task4/verification.json')
    args = parser.parse_args()
    url = urllib.parse.urlsplit(args.base_url)
    if (url.scheme != 'http' or url.hostname not in ('127.0.0.1', 'localhost', '::1')
            or url.username or url.password or url.path not in ('', '/')
            or url.query or url.fragment):
        parser.error('Use a loopback HTTP SSH-tunnel URL without credentials or paths.')
    if not 2 <= args.requests <= 100:
        parser.error('--requests must be between 2 and 100')
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    report = {
        'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'script_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        'base_url': args.base_url, 'model': args.model,
        'requested_stability_requests': args.requests,
        'status': 'running', 'requests': [], 'checks': {},
        'notes': ['No retries or cloud fallback.',
                  '11-second pauses before normal chat calls respect the shared rate limit.',
                  'Client sends conversation history; server-side session storage is not tested.',
                  'This short sequential test is not a load test or an uptime guarantee.',
                  'No server memory or restart measurements are available through this API.'],
    }
    with output.open('x') as handle:
        json.dump(report, handle, indent=2)

    def save():
        temporary = output.with_suffix(output.suffix + '.tmp')
        temporary.write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')
        temporary.replace(output)

    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(name, path, payload=None, raw=None, paced=False):
        if paced:
            time.sleep(11)
        body = raw if raw is not None else (
            json.dumps(payload).encode() if payload is not None else None)
        record = {'name': name, 'path': path, 'request': payload,
                  'request_bytes': len(body) if body is not None else 0}
        if raw is not None:
            record['raw_request'] = raw.decode()
        start = time.monotonic()
        req = urllib.request.Request(args.base_url.rstrip('/') + path, data=body,
                                     headers={'Content-Type': 'application/json'} if body else {})
        try:
            try:
                response = opener.open(req, timeout=180)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                record['status'] = response.code
                record['headers'] = dict(response.headers.items())
                text = response.read(2_000_001).decode('utf-8', errors='replace')
                record['response_text'] = text
                try:
                    record['response'] = json.loads(text)
                except ValueError:
                    record['response'] = None
        except (OSError, urllib.error.URLError) as error:
            record.update(status=None, error=str(error))
        record['elapsed_seconds'] = round(time.monotonic() - start, 3)
        report['requests'].append(record)
        save()
        print(f"{name}: HTTP {record['status']}, {record['elapsed_seconds']:.2f}s", flush=True)
        return record

    def check(name, passed):
        report['checks'][name] = bool(passed)
        save()

    def chat(name, messages, max_tokens=32):
        return request(name, '/v1/chat/completions', payload={
            'model': args.model, 'messages': messages, 'max_tokens': max_tokens,
            'temperature': 0, 'stream': False}, paced=True)

    try:
        health = request('health_before', '/health')
        check('health_before', health['status'] == 200 and health.get('response') == {'status': 'ok'})
        models = request('models', '/v1/models')
        data = (models.get('response') or {}).get('data', [])
        check('model_available', models['status'] == 200 and any(
            item.get('id') == args.model for item in data))
        if not all(report['checks'].values()):
            raise RuntimeError('Health or model discovery failed; generation skipped.')

        messages = [{'role': 'user', 'content': 'My name is Mira. Reply with a short greeting.'}]
        first = chat('chat_first', messages)
        check('chat_first_completed', completed_chat(first))
        if completed_chat(first):
            messages.append(first['response']['choices'][0]['message'])
            messages.append({'role': 'user', 'content': 'What is my name? Reply with only my name.'})
            second = chat('chat_followup', messages)
            check('chat_followup_completed', completed_chat(second))
            answer = (second.get('response') or {}).get('choices', [{}])[0].get('message', {}).get('content', '')
            check('chat_remembers_name', completed_chat(second) and re.search(r'\bMira\b', answer, re.I))
        else:
            check('chat_followup_completed', False)
            check('chat_remembers_name', False)

        prompts = ['Reply with exactly one word: ready.',
                   'What is 2 plus 3? Answer with one number.',
                   'Name one primary color. Answer with one word.',
                   'What is the opposite of hot? Answer with one word.',
                   'Name one animal. Answer with one word.']
        stability = []
        for index in range(args.requests):
            stability.append(chat(f'stability_{index + 1:02}', [
                {'role': 'user', 'content': prompts[index % len(prompts)]}]))
        check('sequential_requests_completed', all(completed_chat(r) for r in stability))
        report['stability'] = {
            'attempted': len(stability),
            'completed': sum(completed_chat(r) for r in stability),
            'median_seconds': statistics.median(r['elapsed_seconds'] for r in stability),
            'max_seconds': max(r['elapsed_seconds'] for r in stability),
        }
        save()

        oversized = chat('oversized_context', [
            {'role': 'user', 'content': 'hello ' * 2000}], max_tokens=8)
        check('oversized_context_rejected', context_rejected(oversized))

        malformed = request('rate_first', '/v1/chat/completions', raw=b'{', paced=True)
        limited = request('rate_second', '/v1/chat/completions', raw=b'{')
        check('rate_limit', malformed['status'] == 400 and limited['status'] == 429)

        body = chat('oversized_body', [{'role': 'user', 'content': 'x' * 17000}], max_tokens=8)
        check('oversized_body_rejected', body['status'] == 413)
        recovery = chat('recovery', [{'role': 'user', 'content': 'Reply with one word: ready.'}])
        check('chat_after_rejections', completed_chat(recovery))
        health = request('health_after', '/health')
        check('health_after', health['status'] == 200 and health.get('response') == {'status': 'ok'})
        report['status'] = 'passed' if all(report['checks'].values()) else 'failed'
    except (Exception, KeyboardInterrupt) as error:
        report.update(status='failed', error=f'{type(error).__name__}: {error}')
    finally:
        report['finished_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        save()
    print(json.dumps({'status': report['status'], 'checks': report['checks'],
                      'stability': report.get('stability'), 'output': str(output)}, indent=2))
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
