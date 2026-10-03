"""Render independently authored reviews; never infer judgments from answers."""
import argparse
import hashlib
import json
import os
from pathlib import Path


def answer_hash(answer):
    return hashlib.sha256(json.dumps(answer, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()


def render(answers, review, suite, output_dir=Path("reports/task1")):
    judgments = {r['question_id'] + '/' + r['mode']: r for r in review.get('assessments', [])}
    questions = {q['id']: q for q in suite['questions']}
    lines = ['# RAG answer comparison', '',
             'Ten independent pairs, DeepSeek generation only. Judgments were authored separately by the implementing assistant against saved source sections. This is illustrative, not a broad reliability claim.', '',
             f"Index SHA-256: `{answers['index_sha256']}`", '',
             f"Settings: `{json.dumps(answers['settings'], sort_keys=True)}`", '']
    lines += ['Selection: ' + suite.get('selection', ''), '', '| Question | Why selected |', '| --- | --- |']
    for q in suite['questions']:
        lines += [f"| {q['id']} | {q.get('selection_reason', '')} |"]
    lines += ['']
    current = all((r := judgments.get(a['question_id'] + '/' + a['mode'])) and r.get('answer_sha256') == answer_hash(a) for a in answers['answers']) and len(answers['answers']) == 2 * len(questions)
    if current:
        lines += [review.get('summary', ''), '', '| Question | RAG outcome | My comparison |', '| --- | --- | --- |']
        for pair in review.get('pairs', []):
            lines += [f"| {pair['question_id']} | {pair['outcome']} | {pair['evidence']} |"]
        lines += ['']
    if not current:
        lines += ['UNREVIEWED: one or more answers are absent or do not have a matching independent review. Paired summary withheld.', '']
    lines += ['## Method and limits', '', review.get('basis', 'Answers have not been reviewed.'), '',
              f'Each question ran independently through the rebuilt `week4/advent-agent rag compare` path and the normal DeepSeek adapter. Both arms received identical base instructions, `{answers["settings"]["model"]}`, {answers["settings"]["reasoning"]} reasoning and no temperature override. Only RAG received evidence from `week4/.venv/bin/kotlin-index`, using {answers["settings"]["rag_strategy"]} retrieval, {answers["settings"]["rag_limit"]} results and a {answers["settings"]["rag_budget"]:,}-character whole-chunk budget. There was no session history, memory, project instruction, lifecycle or MCP context; expected facts were withheld. Mode order alternated. Content was assessed before assigning paired outcomes. Reviews are bound to the entire saved answer record by SHA-256.', '',
              'The saved corpus is the reference standard, not a claim about live documentation today. Source pages can contain omissions or inconsistencies. Completeness checks include some useful caveats beyond the literal short question; missing such a caveat is distinguished from an incorrect core answer. No examples were compiled. Latencies below measure provider calls only and do not include local retrieval.', '']
    total_cost = sum(a['call'].get('cost_usd', 0) for a in answers['answers'])
    lines += [f'Estimated generation cost: ${total_cost:.6f}. Embedding work excluded.', '', '| Mode | Answers | Provider seconds | Total tokens | Estimated USD |', '| --- | ---: | ---: | ---: | ---: |']
    for mode in ('plain', 'rag'):
        rows = [a for a in answers['answers'] if a['mode'] == mode]
        lines += [f"| {mode} | {len(rows)} | {sum(a['call'].get('duration', 0) for a in rows)/1e9:.2f} | {sum(a['call'].get('usage', {}).get('total_tokens', 0) for a in rows)} | {sum(a['call'].get('cost_usd', 0) for a in rows):.6f} |"]
    lines += ['', 'Observed cost and latency differences are not controlled performance estimates; answer lengths, reasoning and cache usage vary.', '', '## Answers and individual reviews', '']
    for a in answers['answers']:
        q = questions[a['question_id']]
        lines += [f"## {a['question_id']} — {a['mode']}", '', q['query'], '', a['answer'], '']
        e = a.get('evidence')
        if e:
            lines += ['Retrieved references:', '']
            for i, c in enumerate(e['chunks'], 1):
                lines += [f"- [{i}] [{c['title']}]({c['source']}), normalized lines {c['line_start']}–{c['line_end']}; {' / '.join(c['sections'] or [])}"]
            lines += ['']
        r = judgments.get(a['question_id'] + '/' + a['mode'])
        if not r or r.get('answer_sha256') != answer_hash(a):
            lines += ['**UNREVIEWED**', '']
            continue
        lines += [f"Review source: [{q['source_review']['excerpt']}]({os.path.relpath(q['source_review']['excerpt'], output_dir)})", '', '| Expected fact | Assessment | Evidence |', '| --- | --- | --- |']
        facts = {f['id']: f['text'] for f in q['expected_facts']}
        for f in r['facts']:
            lines += ['| ' + ' | '.join(str(v).replace('|', '\\|').replace('\n', ' ') for v in (facts[f['id']], f['status'], f['evidence'])) + ' |']
        lines += ['', 'Unsupported/material claims: ' + r['unsupported_claims'], '', 'Citation review: ' + r['citations'], '']
    return '\n'.join(lines) + '\n'

if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--answers', type=Path, default=Path('reports/task1/answers.json'))
    p.add_argument('--review', type=Path, default=Path('reports/task1/review.json'))
    p.add_argument('--suite', type=Path, default=Path('rag-suite.json'))
    p.add_argument('--output', type=Path, default=Path('reports/task1/comparison.md'))
    a = p.parse_args()
    a.output.write_text(render(json.loads(a.answers.read_text()), json.loads(a.review.read_text()) if a.review.exists() else {}, json.loads(a.suite.read_text()), a.output.parent))
