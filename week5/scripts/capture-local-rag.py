#!/usr/bin/env python3
"""Capture a rebuilt local RAG evaluation with outbound networking limited to loopback."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.request

POLICY = '(version 1)(allow default)(deny network*)(allow network-outbound (remote ip "localhost:*"))(allow network* (local unix-socket) (remote unix-socket))'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--suite', default='../week4/rag-grounding-suite.json')
    parser.add_argument('--output', default='results/task2/answers.json')
    parser.add_argument('--runs', type=int, default=3)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    binary = root / 'advent-agent'
    env = dict(os.environ)
    env.pop('DEEPSEEK_API_KEY', None)
    env['XDG_CONFIG_HOME'] = str(root / 'data/task2-config')
    env['XDG_STATE_HOME'] = str(root / 'data/task2-state')
    # Avoid a proxy influencing even loopback requests.
    for key in list(env):
        if key.lower() in ('http_proxy', 'https_proxy', 'all_proxy'):
            env.pop(key)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    def api(path, body=None):
        data = None if body is None else json.dumps(body).encode()
        with opener.open(urllib.request.Request('http://127.0.0.1:11434/api/' + path, data=data, headers={'Content-Type':'application/json'}), timeout=30) as response:
            return json.load(response)
    def command(argv):
        result = subprocess.run(argv, env=env, capture_output=True, text=True)
        return {'argv':argv, 'exit_code':result.returncode, 'stdout':result.stdout, 'stderr':result.stderr}
    metadata_path = Path(args.output).with_suffix('.environment.json')
    if metadata_path.exists():
        raise SystemExit(f'Preserving {metadata_path}; resume using advent-agent directly.')
    metadata = {'recorded_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),
                'artifact':str(binary), 'artifact_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),
                'cloud_key_passed':False, 'network_policy':POLICY, 'version':api('version'), 'models':{}}
    for model in ['embeddinggemma:300m', 'hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M']:
        info = api('show', {'model':model})
        if info.get('remote_model') or info.get('remote_host'):
            raise SystemExit(f'Refusing remote model: {model}')
        metadata['models'][model] = {key:info.get(key) for key in ('details','model_info','remote_model','remote_host')}
    metadata['tags'] = api('tags')
    metadata['memory_before'] = command(['vm_stat'])
    metadata['swap_before'] = command(['sysctl','vm.swapusage'])
    metadata['ps_before'] = api('ps')
    proof = command(['/usr/bin/sandbox-exec','-p',POLICY,sys.executable,'-c',
                     'import socket; s=socket.socket(); s.settimeout(2); s.connect(("1.1.1.1",80))'])
    metadata['external_network_probe'] = proof
    if proof['exit_code'] == 0 or 'Operation not permitted' not in proof['stderr']:
        raise SystemExit('External-network denial was not verified: ' + str(proof))
    argv = ['/usr/bin/sandbox-exec','-p',POLICY,str(binary),'--profile','local-rag','rag','compare',
            '--matrix','repeated','--suite',str(Path(args.suite).resolve()),'--output',str(Path(args.output).resolve()),'--runs',str(args.runs)]
    metadata['argv'] = argv
    metadata_path.parent.mkdir(parents=True, exist_ok=True)
    metadata_path.write_text(json.dumps(metadata, indent=2) + '\n')
    # Stream progress, while the CLI saves each trial independently.
    result = subprocess.run(argv, env=env, cwd=root)
    metadata['exit_code'] = result.returncode
    metadata['ps_after'] = api('ps')
    metadata['memory_after'] = command(['vm_stat'])
    metadata['swap_after'] = command(['sysctl','vm.swapusage'])
    metadata_path.write_text(json.dumps(metadata, indent=2) + '\n')
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()
