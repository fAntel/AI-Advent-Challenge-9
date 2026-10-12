#!/usr/bin/env python3
"""Render Task2 measurements and separately authored, hash-bound semantic reviews."""
import argparse
import hashlib
import json
from pathlib import Path
import statistics


def fingerprint(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()


def timing(values):
    return 'N/A' if not values else f'{statistics.median(values):.2f} s ({min(values):.2f}–{max(values):.2f})'


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--answers', default='results/task2/answers.json')
    p.add_argument('--review', default='results/task2/review.json')
    p.add_argument('--baseline', default='../week4/reports/grounding/answers.json')
    p.add_argument('--output', default='results/task2/comparison.md')
    args = p.parse_args()
    run = json.loads(Path(args.answers).read_text())
    cloud = json.loads(Path(args.baseline).read_text())
    if run['index_sha256'] != cloud['index_sha256'] or run['suite_sha256'] != cloud['suite_sha256']:
        raise SystemExit('Historical suite/index differs; comparison would be misleading.')
    baseline_review_path = Path(args.baseline).with_name('review.json')
    baseline_reviews = json.loads(baseline_review_path.read_text())['assessments']
    baseline_by_id = {r['question_id']:r for r in baseline_reviews}
    baseline_valid = all(baseline_by_id.get(a['question_id'], {}).get('answer_sha256') == fingerprint(a) for a in cloud['answers'])
    reviews = json.loads(Path(args.review).read_text()) if Path(args.review).exists() else {}
    trials = run['trials']
    completed = [t for t in trials if t['completed']]
    successful = [t for t in completed if not t.get('error')]
    def stages(t):
        prep = t.get('preparation', {})
        return (prep.get('retrieval_duration', 0) / 1e9,
                sum(c['call'].get('duration_ns', c['call'].get('duration', 0)) for c in prep.get('calls', [])) / 1e9,
                t.get('answer', {}).get('call', {}).get('duration_ns', t.get('answer', {}).get('call', {}).get('duration', 0)) / 1e9)
    lines = ['# Task2: fully local grounded RAG', '',
             f"Model: `{run['settings']['model']}`. Context: {run['context_window']}; {run['passes']} independent passes.", '',
             f"Index: `{run['index_sha256']}`. Suite: `{run['suite_sha256']}`.", '',
             'Both pipelines retrieve locally, score relevance, filter, and validate citations and exact quotes. Local scoring uses Spark in batches of two; the historical run used DeepSeek Flash in batches of five. Different scores can produce different evidence. Local requests additionally use Ollama JSON schemas and short scoring IDs; both pipelines retain the same final citation/quote validator. This is a pipeline comparison, not an isolated generation benchmark.', '',
             f'Completed: {len(completed)}/{len(cloud["answers"])*run["passes"]} planned trials. Local first-attempt success: **{len(successful)}/{len(completed)}** completed trials. Failures are retained and never silently retried.', '',
             '| Local stage | Median (range) |', '| --- | --- |']
    for i, label in enumerate(['Retrieval', 'Scoring/rewrite', 'Answer generation']):
        vals = [stages(t)[i] for t in completed if stages(t)[i] > 0]
        lines.append(f'| {label} | {timing(vals)} |')
    lines += [f"| Total trial | {timing([t['duration']/1e9 for t in completed])} |", '',
              'Times include observed loading/cache behavior; failed trials can be shorter. A skipped answer is not a fast generation call. No forced cold-start or thermal normalization was performed.', '']
    baseline_preps = cloud['preparations']
    cloud_stages = []
    for answer in cloud['answers']:
        prep = baseline_preps[answer['question_id'] + '/rewrite=false']
        retrieval = prep['retrieval_duration'] / 1e9
        scoring = sum(c['call'].get('duration', 0) for c in prep['calls']) / 1e9
        generation = answer['call'].get('duration', 0) / 1e9
        cloud_stages.append((retrieval, scoring, generation, retrieval + scoring + generation))
    lines += ['| Historical cloud stage | Median (range) |', '| --- | --- |']
    for i, label in enumerate(['Retrieval', 'Scoring', 'Answer generation', 'Sum of recorded stages']):
        lines.append(f"| {label} | {timing([v[i] for v in cloud_stages if v[i] > 0])} |")
    lines += ['', 'Cloud stage sums are not wall-clock trial measurements and exclude unrecorded orchestration time. Measurements were made on different dates.', '']
    bound_reviews = {f"{t['pass']}/{t['question_id']}": reviews[f"{t['pass']}/{t['question_id']}"]
                     for t in completed if reviews.get(f"{t['pass']}/{t['question_id']}", {}).get('sha256') == fingerprint(t)}
    lines += ['Two suite questions were used in development pilots to fix protocol handling. This is an illustrative evaluation, not a held-out benchmark.', '']
    lines += ['Semantic reviews were authored by the implementing assistant against saved sources, separately from model scoring. Fact coverage grades answer text and examples; supporting quotations are assessed separately. Generated examples were not compiled. Rejected outputs are not published answers and are excluded from semantic fact totals, while remaining failures in the completion rate.', '']
    valid = {}
    for t in successful:
        key = f"{t['pass']}/{t['question_id']}"
        review = reviews.get(key)
        if (review and review.get('sha256') == fingerprint(t)
                and review.get('support') in ('yes', 'partial', 'no', 'N/A')
                and isinstance(review.get('correct_unknown'), bool)
                and isinstance(review.get('notes'), str) and review['notes'].strip()
                and isinstance(review.get('facts'), list)
                and len(review['facts']) == (0 if t['question_id'] in ['q21','q22','q23'] else 3)
                and all(f in ('yes', 'partial', 'no') for f in review['facts'])):
            valid[key] = review
    if len(valid) == len(successful) and successful:
        facts = [f for r in valid.values() for f in r.get('facts', [])]
        lines += [f"Reviewed successful trials: {len(valid)}. Expected facts: {facts.count('yes')} full, {facts.count('partial')} partial, {facts.count('no')} absent/incorrect.",
                  f"Fully supported factual answers: {sum(r.get('support') == 'yes' for r in valid.values())}/{sum(t['answer']['result']['status'] == 'answered' for t in successful)}.",
                  f"Correct unknown controls: {sum(r.get('correct_unknown') is True for r in valid.values())}/{sum(t['question_id'] in ['q21','q22','q23'] for t in completed)}.", '']
    else:
        lines += ['Semantic totals withheld: missing or stale reviews. Exact-quote validation alone does not prove correctness.', '']
    calls = [c['call'] for t in completed for c in t.get('preparation', {}).get('calls', [])]
    calls += [t['answer']['call'] for t in completed if t.get('answer') and not t['answer'].get('answer_skipped')]
    lines += [f"Recorded local calls: {len(calls)}; provider tokens: {sum(c.get('usage', {}).get('total_tokens', 0) for c in calls):,}. Generation API charges: $0, excluding hardware and electricity.", '']
    failures = [t for t in completed if t.get('error')]
    lines += [f"Failures: {len(failures)}; output-limit failures: {sum('token limit' in t['error'] for t in failures)}; timeout failures: {sum('deadline' in t['error'].lower() or 'timeout' in t['error'].lower() for t in failures)}.", '', f"Answerable questions refused: {sum(t['question_id'] not in ['q21','q22','q23'] and t['answer']['result']['status']=='unknown' for t in successful)}.", '']
    cloud_failures = len(cloud.get('failed_attempts', [])) + sum(bool(c.get('error')) for prep in cloud['preparations'].values() for c in prep['calls'])
    environment_path = Path(args.answers).with_suffix('.environment.json')
    if environment_path.exists():
        environment = json.loads(environment_path.read_text())
        lines += ['## Resource observations', '', f"Ollama API version: {environment['version']['version']}. Runtime metadata and the loopback-only client network policy are saved in [{environment_path.name}]({environment_path.name}).", '']
        for key, label in [('swap_before', 'Before'), ('swap_after', 'After')]:
            if key in environment:
                lines.append(f"{label}: `{environment[key]['stdout'].strip()}`")
        lines += ['', 'Swap was already in use before the run and increased substantially. These are system-wide snapshots, not an attribution of all memory use to RAG or a peak-memory measurement. The system was not a clean idle-machine benchmark. Query embedding caches were reused across passes.', '']
        for model in environment.get('ps_after', {}).get('models', []):
            lines.append(f"Loaded at finish: `{model['name']}`, {model['size']:,} reported bytes, context {model.get('context_length', 'unknown')}. This excludes other application and OS memory.")
        lines += ['']
    lines += ['## Historical DeepSeek baseline', '',
              f"Saved answers: {len(cloud['answers'])}; failed attempts retained: {cloud_failures}. See [the original review](../../../week4/reports/grounding/comparison.md). Its semantic review is {'hash-verified' if baseline_valid else 'STALE: semantic totals unavailable'}.", '',
              'The baseline has only one pass. Repeated-run cloud stability is unavailable; do not compare its single pass with local variation as equivalent samples.', '',
              '## Per-question stability', '', '| Question | Pass outcomes | Full expected facts per pass |', '| --- | --- | --- |']
    for q in dict.fromkeys(t['question_id'] for t in trials):
        outcomes = ['error' if t.get('error') else t.get('answer', {}).get('result', {}).get('status', 'unfinished') for t in trials if t['question_id'] == q]
        coverage = []
        for t in trials:
            if t['question_id'] != q:
                continue
            review = bound_reviews.get(f"{t['pass']}/{q}")
            coverage.append('invalid' if t.get('error') else ('unreviewed' if not review else str(review.get('facts', []).count('yes'))))
        lines.append(f"| {q} | {', '.join(outcomes)} | {', '.join(coverage)} |")
    lines += ['', '## Evidence selection', '', '| Question | Local eligible candidates per pass | Cloud eligible | Local packed chunks per pass | Cloud packed |', '| --- | --- | --- | --- | --- |']
    for q in dict.fromkeys(t['question_id'] for t in trials):
        selected = [t for t in completed if t['question_id'] == q]
        cp = baseline_preps[q + '/rewrite=false']
        ca = next(a for a in cloud['answers'] if a['question_id'] == q)
        eligible = [str(sum(v >= run['settings']['rag_threshold'] for v in t.get('preparation', {}).get('scores', {}).values())) for t in selected]
        packed = [str(len(t['answer']['evidence']['chunks'])) if t.get('answer') else 'failed preparation' for t in selected]
        lines.append(f"| {q} | {', '.join(eligible)} | {sum(v>=2 for v in cp['scores'].values())} | {', '.join(packed)} | {len(ca['evidence']['chunks'])} |")
    lines += ['', '## Scoring variation', '', '| Question | Changed scores vs pass 1 | Changed filter decisions vs pass 1 |', '| --- | --- | --- |']
    for q in dict.fromkeys(t['question_id'] for t in trials):
        grouped = sorted([t for t in completed if t['question_id'] == q], key=lambda t:t['pass'])
        if len(grouped) < 2:
            continue
        first = grouped[0].get('preparation', {}).get('scores', {})
        changes, filters = [], []
        threshold = run['settings']['rag_threshold']
        for other in grouped[1:]:
            current = other.get('preparation', {}).get('scores', {})
            common = set(first) & set(current)
            changes.append(f"{sum(first[k]!=current[k] for k in common)}/{len(common)}")
            filters.append(f"{sum((first[k]>=threshold)!=(current[k]>=threshold) for k in common)}/{len(common)}")
        lines.append(f"| {q} | {', '.join(changes)} | {', '.join(filters)} |")
    if baseline_valid:
        answered = [a for a in cloud['answers'] if a['result']['status'] == 'answered']
        controls = [a for a in cloud['answers'] if a['question_id'] in ['q21','q22','q23']]
        lines += ['', f"Historical quote support: {sum(baseline_by_id[a['question_id']]['meaning_matches_quotes']=='yes' for a in answered)}/{len(answered)} fully supported factual answers. Correct unknown controls: {sum(baseline_by_id[a['question_id']]['appropriate_unknown'] for a in controls)}/{len(controls)}.", '']
    lines += ['', '## Saved local trials', '']
    for t in trials:
        key = f"{t['pass']}/{t['question_id']}"
        lines += [f'### {key}', '', f"Elapsed: {t['duration']/1e9:.2f} s. Review hash: `{fingerprint(t)}`.", '']
        if t.get('error'):
            lines += ['Error: ' + t['error'], '']
        if t.get('answer'):
            lines += [t['answer'].get('answer') or 'Rejected raw response:\n\n' + t['answer'].get('raw_answer', ''), '']
        if key in bound_reviews:
            lines += ['Review: ' + bound_reviews[key]['notes'], '']
    rendered = '\n'.join(line.rstrip() for line in '\n'.join(lines).splitlines())
    Path(args.output).write_text(rendered.rstrip() + '\n')


if __name__ == '__main__':
    main()
