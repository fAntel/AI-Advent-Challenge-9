"""Render the eight-mode experiment; judgments must be authored separately."""
import argparse
import hashlib
import json
from pathlib import Path


def answer_hash(answer):
    return hashlib.sha256(json.dumps(answer, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()


def matched(chunk, question):
    # Exact section-path labels: a retrieval proxy, not a factual-quality judgment.
    return any(chunk['document_id'] == label['document_id'] and
               label['section'].casefold() in [part.strip().casefold() for s in chunk.get('sections', []) for part in s.split(' > ')]
               for label in question.get('relevant', []))


def calls_for(answer):
    return [c['call'] for c in answer['evidence']['preparation']['calls']] + [answer['call']]


def valid_review(answer, review, question):
    if not review or review.get('answer_sha256') != answer_hash(answer):
        return False
    expected = {f['id'] for f in question.get('expected_facts', [])}
    facts = review.get('facts', [])
    return (len(facts) == len(expected) and {f['id'] for f in facts} == expected
            and all(f.get('status') in ('correct', 'partial', 'missing', 'incorrect') and f.get('evidence') for f in facts)
            and isinstance(review.get('material_error'), bool)
            and review.get('citations') in ('supported', 'mixed', 'unsupported', 'none')
            and bool(review.get('notes'))
            and (not question.get('unanswerable') or isinstance(review.get('honest_insufficiency'), bool)))


def summarize(run, review, suite):
    questions = {q['id']: q for q in suite['questions']}
    assessments = {a['question_id'] + '/' + a['mode']: a for a in review.get('assessments', [])}
    expected = {(q, m['name']) for q in questions for m in run['modes']}
    actual = {(a['question_id'], a['mode']) for a in run['answers']}
    complete = expected == actual and len(run['answers']) == len(expected)
    complete = complete and all(valid_review(a, assessments.get(a['question_id'] + '/' + a['mode']), questions[a['question_id']]) for a in run['answers'])
    modes = {}
    for mode in run['modes']:
        rows = [a for a in run['answers'] if a['mode'] == mode['name']]
        answerable = [a for a in rows if not questions[a['question_id']].get('unanswerable')]
        controls = [a for a in rows if questions[a['question_id']].get('unanswerable')]
        stages = {}
        for purpose in ('rag_rewrite', 'retrieval', 'rag_score', 'answer'):
            calls = ([a['call'] for a in rows] if purpose == 'answer' else
                     [c['call'] for a in rows for c in a['evidence']['preparation']['calls'] if c['purpose'] == purpose])
            stages[purpose] = {'seconds': (sum(a['evidence']['preparation']['retrieval_duration'] for a in rows) if purpose == 'retrieval' else sum(c.get('duration', 0) for c in calls)) / 1e9,
                               'tokens': sum(c.get('usage', {}).get('total_tokens', 0) for c in calls),
                               'cost_usd': sum(c.get('cost_usd', 0) for c in calls)}
        metrics = {'answers': len(rows), 'stages': stages,
                   'candidate_hits': sum(any(matched(c, questions[a['question_id']]) for c in a['evidence']['preparation']['candidates']) for a in answerable),
                   'packed_hits': sum(any(matched(c, questions[a['question_id']]) for c in a['evidence']['chunks']) for a in answerable),
                   'empty_controls': sum(not a['evidence']['chunks'] for a in controls),
                   'filtered_chunks': sum(d['status'] == 'filtered' for a in rows for d in a['evidence']['decisions']),
                   'designated_chunks_filtered': sum(matched(c, questions[a['question_id']]) for a in answerable for c in a['evidence']['preparation']['candidates'] if any(d['id'] == c['id'] and d['status'] == 'filtered' for d in a['evidence']['decisions']))}
        if complete:
            reviews = [assessments[a['question_id'] + '/' + a['mode']] for a in rows]
            metrics.update(correct_facts=sum(f['status'] == 'correct' for r in reviews for f in r['facts']),
                           partial_facts=sum(f['status'] == 'partial' for r in reviews for f in r['facts']),
                           total_facts=sum(len(r['facts']) for r in reviews),
                           material_errors=sum(r['material_error'] for r in reviews),
                           supported_citations=sum(r['citations'] == 'supported' for r in reviews),
                           problematic_citations=sum(r['citations'] in ('mixed', 'unsupported') for r in reviews),
                           honest_controls=sum(assessments[a['question_id'] + '/' + a['mode']]['honest_insufficiency'] for a in controls))
        modes[mode['name']] = metrics
    actual_calls = [a['call'] for a in run['answers']] + [c['call'] for p in run['preparations'].values() for c in p['calls']]
    return {'review_complete': complete, 'modes': modes,
            'actual_calls': len(actual_calls), 'actual_cost_usd': sum(c.get('cost_usd', 0) for c in actual_calls),
            'actual_tokens': sum(c.get('usage', {}).get('total_tokens', 0) for c in actual_calls)}


def render(run, review, suite):
    stats = summarize(run, review, suite)
    questions = {q['id']: q for q in suite['questions']}
    assessments = {r['question_id'] + '/' + r['mode']: r for r in review.get('assessments', [])}
    settings = run['settings']
    n_answerable = sum(not q.get('unanswerable') for q in suite['questions'])
    n_controls = len(suite['questions']) - n_answerable
    lines = ['# Task 3: eight-mode RAG comparison', '',
             f"{len(suite['questions'])} questions × {len(run['modes'])} modes. Model `{settings['model']}`, reasoning `{settings['reasoning']}`. Index SHA-256: `{run['index_sha256']}`.", '',
             f"Candidate K={settings['rag_candidates']}; final K={settings['rag_limit']}; minimum relevance={settings['rag_threshold']}; evidence budget={settings['rag_budget']} characters.", '',
             '## Findings', '', review.get('summary', '') if stats['review_complete'] else '**UNREVIEWED**: answers or hash-bound reviews are incomplete; quality summary withheld.', '',
             '## Retrieval', '',
             f'| Mode | Candidate section hits / {n_answerable} | Packed section hits / {n_answerable} | Empty controls / {n_controls} | Chunks filtered | Designated-section chunks filtered |',
             '| --- | ---: | ---: | ---: | ---: | ---: |']
    for mode, m in stats['modes'].items():
        lines.append(f"| {mode} | {m['candidate_hits']} | {m['packed_hits']} | {m['empty_controls']} | {m['filtered_chunks']} | {m['designated_chunks_filtered']} |")
    if stats['review_complete']:
        lines += ['', '## Answer quality', '', f'| Mode | Correct facts | Partial facts | Answers with material errors | Supported citation sets | Problematic citation sets | Honest controls / {n_controls} |', '| --- | ---: | ---: | ---: | ---: | ---: | ---: |']
        for mode, m in stats['modes'].items():
            lines.append(f"| {mode} | {m['correct_facts']}/{m['total_facts']} | {m['partial_facts']} | {m['material_errors']} | {m['supported_citations']} | {m['problematic_citations']} | {m['honest_controls']} |")
    check = review.get('external_check')
    if stats['review_complete'] and check:
        lines += ['', 'Post-generation external spot-check: ' + check['purpose'], '',
                  check['findings'] + f" [Official Debezium documentation]({check['source']}) (accessed {check['accessed']}).", '']
    lines += ['', '## Cost and latency', '',
              f"Actual recorded calls (shared preparation counted once): {stats['actual_calls']}; tokens: {stats['actual_tokens']:,}; estimated cost: ${stats['actual_cost_usd']:.6f}.", '',
              'Below, each mode includes its full preparation cost as if run independently. These rows must not be summed to obtain experiment cost. Local retrieval includes subprocess startup, index checks and embedding work; it has no hosted-token charge. Provider pricing uses the repository adapter estimates.', '',
              '| Mode | Stage | Seconds | Tokens | Estimated USD |', '| --- | --- | ---: | ---: | ---: |']
    for mode, m in stats['modes'].items():
        for stage, v in m['stages'].items():
            lines.append(f"| {mode} | {stage} | {v['seconds']:.2f} | {v['tokens']} | {v['cost_usd']:.6f} |")
    lines += ['', '## Method and limits', '',
              'One run per question and mode; no significance or general reliability claim. All modes use the same corpus, base answering instructions and original question. Mode order rotates. Rewrites and candidate pools are shared within each query condition; scoring results are shared across applicable modes. Auxiliary calls always use Flash with reasoning disabled. No temperature override is sent. Expected facts and relevance labels never enter generation or scoring prompts.', '',
              'The rubric, cutoff and K values were fixed before generation. Flash relevance scores are ordinal judgments, not calibrated probabilities or independent ground truth. Retrieval metrics use exact designated section-path labels, which are not exhaustive and do not guarantee answer support. Filtered designated chunks can be irrelevant to the specific question despite sharing its section. A corpus miss does not imply the model cannot answer from general knowledge.', '',
              'Reviews were authored separately by the implementing assistant against the saved source excerpts and full answers, not by the Flash scorer. Correct, partial, missing and incorrect facts are distinguished. Citation judgments check the actual packed evidence. Controls pass when corpus insufficiency is honestly acknowledged without invented support; general knowledge may be provided if distinguished from evidence. Missing expected caveats are not automatically material errors. No code examples were compiled. Provider timings vary with caching and service load.', '',
              '## Answers and independent reviews', '']
    for a in run['answers']:
        q = questions[a['question_id']]
        prep = a['evidence']['preparation']
        lines += [f"### {a['question_id']} — {a['mode']}", '', q['query'], '', f"Search query: {prep['search_query']}", '', a['answer'], '']
        for i, c in enumerate(a['evidence']['chunks'], 1):
            lines.append(f"- [{i}] [{c['title']}]({c['source']}), lines {c['line_start']}–{c['line_end']}; {' / '.join(c.get('sections', []))}")
        lines.append('')
        r = assessments.get(a['question_id'] + '/' + a['mode'])
        if not valid_review(a, r, q):
            lines += ['**UNREVIEWED**', '']; continue
        for f in r['facts']:
            lines.append(f"- {f['id']}: **{f['status']}** — {f['evidence']}")
        lines += ['', f"Material error: {r['material_error']}. Citations: {r['citations']}. {r['notes']}", '']
        if q.get('source_review'):
            lines += [f"Reference excerpt: [{q['source_review']['excerpt']}](../../{q['source_review']['excerpt']}).", '']
    return '\n'.join(lines).rstrip() + '\n'


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--answers', type=Path, default=Path('reports/task3/answers.json'))
    p.add_argument('--review', type=Path, default=Path('reports/task3/review.json'))
    p.add_argument('--suite', type=Path, default=Path('rag-task3-suite.json'))
    p.add_argument('--output', type=Path, default=Path('reports/task3/comparison.md'))
    a = p.parse_args()
    run, suite = json.loads(a.answers.read_text()), json.loads(a.suite.read_text())
    review = json.loads(a.review.read_text()) if a.review.exists() else {}
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_text(render(run, review, suite))
    a.output.with_suffix('.json').write_text(json.dumps(summarize(run, review, suite), indent=2) + '\n')


if __name__ == '__main__':
    main()
