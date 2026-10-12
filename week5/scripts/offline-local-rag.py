#!/usr/bin/env python3
"""Run the CLI restart/resume demo with both Ollama and clients denied external networking."""
import argparse
import datetime
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import time
import tomllib
import urllib.request

POLICY = '(version 1)(allow default)(deny network*)(allow network-outbound (remote ip "localhost:*"))(allow network-inbound (local ip "localhost:*"))(allow network-bind (local ip "localhost:*"))(allow network* (local unix-socket) (remote unix-socket))'


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--output', default='results/task2/offline-demo.json')
    args = p.parse_args()
    root = Path(__file__).resolve().parent.parent
    output = Path(args.output).resolve()
    metadata_path = output.with_suffix('.environment.json')
    if output.exists() or metadata_path.exists():
        raise SystemExit('Choose a new output path; existing capture preserved.')
    env = dict(os.environ)
    env.pop('DEEPSEEK_API_KEY', None)
    for key in list(env):
        if key.lower() in ('http_proxy', 'https_proxy', 'all_proxy'):
            env.pop(key)
    host = 'http://127.0.0.1:11435'
    with socket.socket() as probe:
        if probe.connect_ex(('127.0.0.1', 11435)) == 0:
            raise SystemExit('Port 11435 is occupied; refusing to use an unrelated server.')
    env['OLLAMA_HOST'] = host
    env['OLLAMA_NO_CLOUD'] = '1'
    # Pin the original embedding variant used by the Week4 index. New servers
    # otherwise select a different runner/weight variant from the same tag.
    source_models = Path(env.get('OLLAMA_MODELS', str(Path.home() / '.ollama/models')))
    pinned_models = root / 'data/offline-models'
    pinned_models.mkdir(parents=True, exist_ok=True)
    if not (pinned_models / 'blobs').exists():
        (pinned_models / 'blobs').symlink_to(source_models / 'blobs', target_is_directory=True)
    for relative in ['registry.ollama.ai/library/embeddinggemma/300m', 'hf.co/XHToken/Spark-X2.5-4B-GGUF/Q4_K_M']:
        target = pinned_models / 'manifests' / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        source = source_models / 'manifests' / relative
        if relative.endswith('embeddinggemma/300m'):
            source = source_models / 'blobs/sha256-3a2d262bdb75dd103c532a4cf9595dc033fd47a2710a061f7a364c7cbb475c9c'
        target.write_bytes(source.read_bytes())
    env['OLLAMA_MODELS'] = str(pinned_models)
    env['OLLAMA_NOPRUNE'] = '1'
    env['XDG_CONFIG_HOME'] = str(root / 'data/task2-offline-config')
    env['XDG_STATE_HOME'] = str(root / 'data/offline-state')
    config = Path(env['XDG_CONFIG_HOME']) / 'advent-agent/profiles/local-rag/config.toml'
    if not config.exists():
        subprocess.run([sys.executable, str(root / 'scripts/install-local-rag.py'), '--context', '16384'], env=env, check=True)
        config.write_text(config.read_text().replace('http://127.0.0.1:11434', host))
    existing = tomllib.loads(config.read_text())
    if existing['ollama']['endpoint'] != host or existing['agent']['context_window_tokens'] != 16384:
        raise SystemExit('Existing offline profile differs; preserving it.')
    sandbox = ['/usr/bin/sandbox-exec', '-p', POLICY]
    probe = subprocess.run(sandbox + [sys.executable, '-c', 'import socket; s=socket.socket(); s.settimeout(2); s.connect(("1.1.1.1",80))'], env=env, text=True, capture_output=True)
    if probe.returncode == 0 or 'Operation not permitted' not in probe.stderr:
        raise SystemExit('External-network denial not verified: ' + probe.stderr)
    metadata = {'created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'network_policy':POLICY,
                'external_probe_error':probe.stderr, 'endpoint':host, 'server_and_clients_sandboxed':True}
    output.parent.mkdir(parents=True, exist_ok=True)
    metadata_path.write_text(json.dumps(metadata, indent=2) + '\n')
    log_path = root / 'data/offline-ollama.log'
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    def api(path, body=None):
        data = None if body is None else json.dumps(body).encode()
        with opener.open(urllib.request.Request(host + '/api/' + path, data=data, headers={'Content-Type':'application/json'}), timeout=10) as response:
            return json.load(response)
    with log_path.open('w') as log:
        server = subprocess.Popen(sandbox + [shutil.which('ollama'), 'serve'], env=env, stdout=log, stderr=subprocess.STDOUT)
        metadata['server_pid'] = server.pid
        try:
            deadline = time.monotonic() + 30
            while True:
                try:
                    metadata['version'] = api('version')
                    break
                except OSError:
                    if server.poll() is not None or time.monotonic() >= deadline:
                        raise RuntimeError(f'Isolated Ollama did not start; inspect {log_path}')
                    time.sleep(.2)
            for model in ['embeddinggemma:300m','hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M']:
                show = api('show', {'model':model})
                if show.get('remote_model') or show.get('remote_host'):
                    raise RuntimeError('Remote model refused')
            metadata['tags'] = api('tags')
            result = subprocess.run(sandbox + [sys.executable, str(root / 'scripts/demo-local-rag.py'), '--output', str(output)], env=env, cwd=root)
            metadata['exit_code'] = result.returncode
            metadata['ps_after'] = api('ps')
        finally:
            subprocess.run([str(root / 'advent-agentd'), '--profile', 'local-rag', 'stop'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            server.terminate()
            try:
                server.wait(timeout=20)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            metadata_path.write_text(json.dumps(metadata, indent=2) + '\n')
    raise SystemExit(metadata.get('exit_code', 1))


if __name__ == '__main__':
    main()
