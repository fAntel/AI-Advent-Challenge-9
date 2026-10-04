"""Render saved mini-chat dialogs and separately authored, hash-bound reviews."""
import argparse
import hashlib
import json
from pathlib import Path

from render_rag_grounding import evidence_checks


def turn_hash(turn):
    return hashlib.sha256(json.dumps(turn, sort_keys=True, ensure_ascii=False,
                                     separators=(',', ':')).encode()).hexdigest()


def reviewed(turn, assessment):
    return bool(assessment and assessment.get('turn_sha256') == turn_hash(turn)
                and assessment.get('goal_and_constraints') in ('yes', 'partial', 'no')
                and assessment.get('semantic_support') in ('yes', 'partial', 'no', 'N/A')
                and isinstance(assessment.get('appropriate_response'), bool)
                and assessment.get('notes'))


def render(run, review, suite, suite_hash):
    assessments = {r['turn_id']: r for r in review.get('assessments', [])}
    turns = [t for s in run['sessions'] for t in s['turns']]
    complete = (run['suite_sha256'] == suite_hash
                and len(run['sessions']) == len(suite['dialogs']) == 2
                and all(len(s['turns']) == len(d['questions'])
                        for s, d in zip(run['sessions'], suite['dialogs']))
                and all(t['state'] == 'completed' and reviewed(t, assessments.get(t['id']))
                        for t in turns))
    lines = ['# Task4: mini-chat with RAG and task memory', '',
             'A two-dialog evaluation using the standalone Go CLI, local Kotlin index and DeepSeek. '
             'Each answer receives four recent exchanges plus persistent task memory and the question log. '
             'These are illustrative checks, not a general reliability guarantee.', '',
             f"Suite SHA-256: `{run['suite_sha256']}`.", '',
             'Reviews below were authored separately against the saved evidence and bound to complete turn hashes. '
             'Exact quotations establish provenance, not semantic entailment.', '']
    for note in review.get('run_notes', []):
        lines += [note, '']
    factual = [t for t in turns if t.get('result', {}).get('status') == 'answered']
    source_valid = lambda t: evidence_checks(t)['sources'] == 'yes' and evidence_checks(t)['exact']
    lines += [f"Completed turns: {sum(t['state'] == 'completed' for t in turns)}/{len(turns)}. "
              f"Factual answers: {len(factual)}; source and quote provenance valid: {sum(source_valid(t) for t in factual)}/{len(factual)}. "
              f"Conflict clarifications: {sum(bool(t.get('conflicts')) for t in turns)}. "
              f"Unknown responses: {sum(t.get('result', {}).get('status') == 'unknown' for t in turns)}.", '']
    if complete:
        lines += [f"Goal/constraint adherence: {sum(assessments[t['id']]['goal_and_constraints'] == 'yes' for t in turns)}/{len(turns)}. "
                  f"Factual answers fully supported by displayed quotes: {sum(assessments[t['id']]['semantic_support'] == 'yes' for t in factual)}/{len(factual)}. "
                  f"Appropriate responses: {sum(assessments[t['id']]['appropriate_response'] for t in turns)}/{len(turns)}.", '']
    else:
        lines += ['**UNREVIEWED or incomplete: semantic quality totals withheld.**', '']
    calls = [c for t in turns for c in t.get('calls', [])]
    lines += [f"Recorded API attempts: {len(calls)}; tokens: {sum(c['response']['Call']['usage']['total_tokens'] for c in calls):,}; "
              f"estimated provider cost: ${sum(c['response']['Call']['cost_usd'] for c in calls):.6f}. "
              f"Failed API attempts retained: {sum(bool(c.get('error')) for c in calls)}. "
              'Embedding cost is not included.', '']
    for i, session in enumerate(run['sessions'], 1):
        lines += [f'## Dialog {i}', '', f"Session: `{session['id']}`", '',
                  f"Final goal: {session['memory']['goal']['value']}", '',
                  '| Turn | Response | Goal/constraints | Support |', '| --- | --- | --- | --- |']
        for n, t in enumerate(session['turns'], 1):
            r = assessments.get(t['id'])
            valid = reviewed(t, r)
            status = 'clarification' if t.get('conflicts') else t.get('result', {}).get('status', t['state'])
            lines += [f"| {n} | {status} | {r['goal_and_constraints'] if valid else 'UNREVIEWED'} | {r['semantic_support'] if valid else 'UNREVIEWED'} |"]
        for n, t in enumerate(session['turns'], 1):
            r = assessments.get(t['id'])
            lines += ['', f'### Turn {n}', '', t['question'], '', t.get('answer', t.get('error', '')), '',
                      'Review: ' + (r['notes'] if reviewed(t, r) else 'UNREVIEWED'), '',
                      'Saved task memory:', '', '```json', json.dumps(t['memory_after'], ensure_ascii=False, indent=2), '```']
    return '\n'.join(line.rstrip() for line in '\n'.join(lines).splitlines()) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--answers', default='reports/task4/answers.json')
    parser.add_argument('--review', default='reports/task4/review.json')
    parser.add_argument('--suite', default='mini-chat-suite.json')
    parser.add_argument('--output', default='reports/task4/dialogs.md')
    args = parser.parse_args()
    review = json.loads(Path(args.review).read_text()) if Path(args.review).exists() else {}
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    suite_bytes = Path(args.suite).read_bytes()
    output.write_text(render(json.loads(Path(args.answers).read_text()), review,
                             json.loads(suite_bytes), hashlib.sha256(suite_bytes).hexdigest()))


if __name__ == '__main__':
    main()
