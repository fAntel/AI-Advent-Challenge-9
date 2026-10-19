"""Shared local support-demo helpers; no real credentials are printed."""
import json
import os
from pathlib import Path
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
URL = 'http://127.0.0.1:8065'


def api(method, path, payload=None, token=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    data = None if payload is None else json.dumps(payload).encode()
    request = urllib.request.Request(URL + '/api/v4' + path, data=data,
                                     headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read()
            return (json.loads(body) if body else None), response.headers
    except urllib.error.HTTPError as error:
        raise RuntimeError(f'Mattermost {method} {path}: HTTP {error.code}') from None


def private_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = path.with_suffix(path.suffix + '.tmp')
    with os.fdopen(os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600), 'w') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')
    temporary.chmod(0o600)
    os.replace(temporary, path)
