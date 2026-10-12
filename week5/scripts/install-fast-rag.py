#!/usr/bin/env python3
"""Install the measured winner only after scoring and reviewed-answer gates pass."""
import json
from pathlib import Path
import subprocess
import sys

ROOT=Path(__file__).resolve().parents[1]


def main():
    subprocess.run([sys.executable,str(ROOT/'scripts/render-score-bench.py')],cwd=ROOT,check=True)
    summary=json.loads((ROOT/'results/task3/summary.json').read_text())
    if not summary.get('adopt'):
        raise SystemExit('No qualifying winner: baseline profiles preserved. See results/task3/comparison.md.')
    text=(ROOT/'results/task3/configs/finalist.toml').read_text().replace('task3-finalist','local-rag-fast')
    text=text.replace('[rag]\n','[rag]\nrerank = true\n')
    target=ROOT/'data/task3-config/advent-agent/profiles/local-rag-fast/config.toml'
    target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists() and target.read_text()!=text:
        raise SystemExit(f'Existing profile differs; preserving {target}')
    if not target.exists():
        with target.open('x') as f:f.write(text)
        target.chmod(0o600)
    print(target)


if __name__=='__main__':main()
