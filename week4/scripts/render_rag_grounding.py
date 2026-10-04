"""Render grounding evidence checks and separately authored, hash-bound reviews."""
import argparse
import json
from pathlib import Path

from render_rag_matrix import answer_hash


def evidence_checks(answer):
    result = answer['result']
    chunks = {c['id']: c for c in answer['evidence']['chunks']}
    sources = result['sources']
    quotes = result['quotes']
    if result['status'] == 'unknown':
        return {'sources': 'N/A', 'quotes': 'N/A', 'exact': not sources and not quotes,
                'unknown_valid': not result['answer'] and bool(result['clarification'].strip())}
    exact = bool(quotes) and all(q['chunk_id'] in chunks and q['text'].strip()
                                and q['text'] in chunks[q['chunk_id']]['text'] for q in quotes)
    provenance = bool(sources) and all(s['chunk_id'] in chunks and
                                     all(s[k] == chunks[s['chunk_id']][k] for k in ('source', 'title', 'sections'))
                                     for s in sources)
    return {'sources': 'yes' if provenance else 'NO', 'quotes': 'yes' if quotes else 'NO',
            'exact': bool(exact), 'unknown_valid': False}


def reviewed(answer, assessment):
    return bool(assessment and assessment.get('answer_sha256') == answer_hash(answer)
                and assessment.get('meaning_matches_quotes') in ('yes', 'partial', 'no', 'N/A')
                and isinstance(assessment.get('appropriate_unknown'), bool)
                and assessment.get('notes'))


def render(run, review, suite):
    questions = {q['id']: q for q in suite['questions']}
    assessments = {r['question_id']: r for r in review.get('assessments', [])}
    answers = run['answers']
    complete = len(answers) == len(questions) and {a['question_id'] for a in answers} == set(questions)
    complete = complete and all(reviewed(a, assessments.get(a['question_id'])) for a in answers)
    lines = ['# Quotes, sources and unknown responses', '',
             'Ten questions: seven answerable from the saved corpus and three out-of-corpus controls. '
             f"Model `{run['settings']['model']}`, reasoning `{run['settings']['reasoning']}`; "
             f"relevance threshold {run['settings']['rag_threshold']}/3. "
             f"Rewrite={run['modes'][0]['rewrite']}, rerank={run['modes'][0]['rerank']}. "
             'Scores are ordinal judgments, not probabilities.', '',
             'Reviews are authored separately against the saved chunks and bound to complete answer hashes. '
             'Exact substring validation establishes quote provenance, not semantic agreement. '
             'This is one illustrative run; it does not establish general reliability.', '',
             f"Index SHA-256: `{run['index_sha256']}`. Suite SHA-256: `{run['suite_sha256']}`.", '']
    if not complete:
        lines += ['**UNREVIEWED or incomplete: quality totals withheld.**', '']
    else:
        supported = [a for a in answers if a['result']['status'] == 'answered']
        controls = [a for a in answers if questions[a['question_id']].get('unanswerable')]
        false_refusals = [a for a in answers if a['result']['status'] == 'unknown' and not questions[a['question_id']].get('unanswerable')]
        lines += [f"{len(supported)} factual answers; sources present in {sum(evidence_checks(a)['sources'] == 'yes' for a in supported)}/{len(supported)}, "
                  f"quotes present in {sum(evidence_checks(a)['quotes'] == 'yes' for a in supported)}/{len(supported)}, "
                  f"exact fragments in {sum(evidence_checks(a)['exact'] for a in supported)}/{len(supported)}.", '',
                  f"Meaning fully supported by quotes: {sum(assessments[a['question_id']]['meaning_matches_quotes'] == 'yes' for a in supported)}/{len(supported)}. "
                  f"Appropriate unknown controls: {sum(assessments[a['question_id']]['appropriate_unknown'] for a in controls)}/{len(controls)}. "
                  f"Answerable questions refused: {len(false_refusals)}.", '']
    lines += ['| Question | Result | Sources | Quotes | Exact fragments | Meaning agrees |',
              '| --- | --- | --- | --- | --- | --- |']
    for a in answers:
        checks = evidence_checks(a)
        r = assessments.get(a['question_id'])
        meaning = r['meaning_matches_quotes'] if reviewed(a, r) else 'UNREVIEWED'
        exact = 'N/A' if a['result']['status'] == 'unknown' else ('yes' if checks['exact'] else 'NO')
        lines.append(f"| {a['question_id']} | {a['result']['status']} | {checks['sources']} | {checks['quotes']} | {exact} | {meaning} |")
    calls = [c['call'] for p in run['preparations'].values() for c in p['calls']]
    calls += [a['call'] for a in answers if not a.get('answer_skipped')]
    calls += [f['record']['call'] for f in run.get('failed_attempts', [])]
    lines += ['', f"Recorded API attempts: {len(calls)}; tokens: {sum(c.get('usage', {}).get('total_tokens', 0) for c in calls):,}; "
              f"estimated cost: ${sum(c.get('cost_usd', 0) for c in calls):.6f}. "
              f"Skipped answer calls: {sum(bool(a.get('answer_skipped')) for a in answers)}. "
              f"Failed attempts retained: {len(run.get('failed_attempts', [])) + sum(bool(c.get('error')) for p in run['preparations'].values() for c in p['calls'])}.", '',
              'Unknown responses legitimately have no sources or quotes. They must contain no factual answer and must request clarification. '
              'API failures and malformed answers are errors, not evidence of corpus insufficiency.', '']
    for a in answers:
        lines += [f"## {a['question_id']}", '', questions[a['question_id']]['query'], '', a['answer'], '']
        r = assessments.get(a['question_id'])
        lines += ['Review: ' + (r['notes'] if reviewed(a, r) else 'UNREVIEWED'), '']
    return '\n'.join(lines).replace('\n> \n', '\n>\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--answers', default='reports/grounding/answers.json')
    parser.add_argument('--review', default='reports/grounding/review.json')
    parser.add_argument('--suite', default='rag-grounding-suite.json')
    parser.add_argument('--output', default='reports/grounding/comparison.md')
    args = parser.parse_args()
    review = json.loads(Path(args.review).read_text()) if Path(args.review).exists() else {}
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(render(json.loads(Path(args.answers).read_text()), review, json.loads(Path(args.suite).read_text())))


if __name__ == '__main__':
    main()
