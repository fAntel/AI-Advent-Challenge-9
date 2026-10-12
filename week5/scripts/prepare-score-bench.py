#!/usr/bin/env python3
"""Freeze Task2 first-pass candidates and pre-tuning relevance labels."""
import argparse
import hashlib
import json
from pathlib import Path


def save_once(path, value):
    text = json.dumps(value, indent=2) + '\n'
    if path.exists():
        if path.read_text() != text:
            raise SystemExit(f'Refusing to overwrite different {path}')
    else:
        path.write_text(text)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-dir', type=Path, default=Path('results/task3'))
    out = parser.parse_args().output_dir
    out.mkdir(parents=True, exist_ok=True)
    src = Path('results/task2/answers.json').read_bytes()
    source = json.loads(src)
    frozen = {k: source[k] for k in ['version', 'suite_sha256', 'index_sha256', 'settings', 'ollama', 'context_window']}
    frozen.update(passes=1, trials=[])
    for t in source['trials']:
        if t['pass'] != 1:
            continue
        p = t['preparation']
        frozen['trials'].append({'pass': 1, 'question_id': t['question_id'], 'preparation': {
            'query': p['query'], 'retrieved': True, 'index_sha256': p['index_sha256'], 'candidates': p['candidates']}})
    save_once(out / 'candidates.json', frozen)
    scores = {
        'q01': [2,0,3,0,3,0,0,0,0,0,0,0,0,1,0,2,0,2,0,0],
        'q13': [3,1,0,0,1,1,1,0,1,1,0,0,1,1,1,1,1,1,1,1],
        'q14': [3,3,0,0,2,0,1,2,0,1,0,0,3,1,1,0,1,1,1,0],
        'q16': [3,0,0,2,0,0,0,3,0,0,2,2,3,2,0,0,2,0,0,0],
        'q18': [3,1,1,1,1,3,1,1,0,3,1,1,0,3,0,0,0,0,0,0],
        'q19': [3,3,2,0,1,0,2,2,0,0,0,0,0,0,0,1,0,0,0,0],
        'q20': [3,3,1,1,1,0,1,1,1,1,1,1,1,1,0,0,0,0,1,1],
        'q21': [0]*20, 'q22': [0]*20, 'q23': [0]*20,
    }
    labels = {
        'version': 'relevance-labels-v1',
        'reviewer': 'Codex manual review before Task3 tuning; not independent human ground truth',
        'source_sha256': hashlib.sha256(src).hexdigest(),
        'input_sha256': hashlib.sha256((out/'candidates.json').read_bytes()).hexdigest(),
        'rubric': {'0': 'unrelated or incompatible context/platform/version/direction',
                   '1': 'related topic without answer support', '2': 'useful partial answer support', '3': 'direct answer support'},
        'notes': [
            'Reviewed complete text for relevant/ambiguous candidates; used source and section context for clear negatives.',
            'Headings alone and links to examples do not supply answer evidence.',
            'Release-specific questions require release evidence in the text or source metadata.',
            'q14 Objective-C callbacks differ from direct Swift export.',
            'q16 Kotlin exports to JavaScript are the opposite interoperability direction.',
            'q22 Kotlin 2.x passages cannot establish Kotlin 3.0 changes.'],
        'questions': {},
    }
    for t in frozen['trials']:
        q = t['question_id']
        labels['questions'][q] = {c['id']: scores[q][i] for i, c in enumerate(t['preparation']['candidates'])}
    save_once(out/'labels.json', labels)


if __name__ == '__main__':
    main()
