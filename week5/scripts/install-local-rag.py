#!/usr/bin/env python3
"""Copy Week4's index and install an isolated local-rag profile (Python 3.11+)."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as source:
        for block in iter(lambda: source.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--context', type=int, choices=[8192, 16384], default=16384)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    week4 = root.parent / 'week4'
    indexer = week4 / '.venv/bin/kotlin-index'
    source = week4 / 'data/index.sqlite'
    if not indexer.is_file() or not source.is_file():
        raise SystemExit('Build the Week4 index and install its Python environment first.')
    database = root / 'data/index.sqlite'
    database.parent.mkdir(parents=True, exist_ok=True)
    expected = digest(source)
    if database.exists():
        if digest(database) != expected:
            raise SystemExit('Existing Week5 index differs; preserving it.')
    else:
        with tempfile.NamedTemporaryFile(dir=database.parent, delete=False) as temporary:
            temp = Path(temporary.name)
        try:
            shutil.copyfile(source, temp)
            if digest(temp) != expected or digest(source) != expected:
                raise SystemExit('Week4 index changed during copy.')
            # Exclusive destination creation: never replace an existing index.
            os.link(temp, database)
        finally:
            temp.unlink()
    def xdg(name, fallback):
        value = Path(os.environ.get(name, str(Path.home() / fallback)))
        return value if value.is_absolute() else Path.home() / fallback
    state = xdg('XDG_STATE_HOME', '.local/state') / 'advent-agent/local-rag'
    target = xdg('XDG_CONFIG_HOME', '.config') / 'advent-agent/profiles/local-rag/config.toml'
    text = (root / 'config.local-rag.example.toml').read_text()
    replacements = {'@INDEXER@':str(root / 'scripts/local-kotlin-index'), '@DATABASE@':str(database), '@STATE@':str(state)}
    for key, value in replacements.items():
        text = text.replace(key, value.replace('\\', '\\\\').replace('"', '\\"'))
    text = text.replace('context_window_tokens = 16384', f'context_window_tokens = {args.context}')
    if target.exists():
        if target.read_text() != text:
            raise SystemExit(f'Existing profile preserved: {target}; review changes manually.')
    else:
        target.parent.mkdir(parents=True, exist_ok=True)
        with target.open('x') as output:
            output.write(text)
        target.chmod(0o600)
    print(f'Installed {target}\nIndex SHA-256: {expected}')


if __name__ == '__main__':
    main()
