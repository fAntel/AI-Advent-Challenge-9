"""Section-level relevance and source-span coverage evaluation."""
import json
from pathlib import Path

import numpy as np

from .core import compatible, query_input, rank, read_index


def targets(question, documents):
    result = []
    for label in question.get('relevant', []):
        doc = documents[label['document_id']]
        wanted = label['section'].casefold()
        blocks = [b for b in doc['blocks'] if any(s.casefold() == wanted for s in b['section'])]
        if not blocks:
            raise ValueError('Evaluation section missing: ' + str(label))
        result.append((label['document_id'], min(b['start'] for b in blocks), max(b['end'] for b in blocks)))
    return result


def overlap(chunk, target):
    doc, start, end = target
    return chunk['document_id'] == doc and min(chunk['end'], end) > max(chunk['start'], start)


def union_length(intervals):
    total, last = 0, -1
    for start, end in sorted(intervals):
        total += max(0, end - max(start, last))
        last = max(last, end)
    return total


def metrics(ranked, relevant, budget=6000):
    if not relevant:
        return None
    top = ranked[:5]
    hits = [any(overlap(c, target) for c in top) for target in relevant]
    first = next((i + 1 for i, c in enumerate(ranked) if any(overlap(c, t) for t in relevant)), None)
    selected, remaining = [], budget
    for chunk in ranked:
        if remaining <= 0:
            break
        take = min(len(chunk['text']), remaining)
        selected.append(dict(chunk, end=chunk['start'] + take))
        remaining -= take
    # Deduplicate both relevance spans and retrieved intersections per document.
    by_document, covered = {}, {}
    for doc, start, end in relevant:
        by_document.setdefault(doc, []).append((start, end))
        for chunk in selected:
            if overlap(chunk, (doc, start, end)):
                covered.setdefault(doc, []).append((max(start, chunk['start']), min(end, chunk['end'])))
    denominator = sum(union_length(spans) for spans in by_document.values())
    return {'hit_at_5': float(any(hits)), 'recall_at_5': sum(hits) / len(hits), 'reciprocal_rank': 0 if first is None else 1 / first, 'coverage_at_6000_chars': sum(union_length(spans) for spans in covered.values()) / denominator}


def compare(index, questions_path, embedder, output):
    meta, docs, rows = read_index(index)
    compatible(meta, embedder)
    questions = json.loads(Path(questions_path).read_text())['questions']
    strategies = sorted({c['strategy'] for c, _ in rows})
    if strategies != ['fixed', 'semantic']:
        raise ValueError('Comparison requires both fixed and semantic indexes')
    report = {'metadata': meta, 'index_bytes': index.stat().st_size, 'strategies': {}, 'questions': []}
    labels = {q['id']: targets(q, docs) for q in questions}
    vectors = embedder.embed([query_input(q['query']) for q in questions])
    for strategy in strategies:
        chunks = [c for c, _ in rows if c['strategy'] == strategy]
        lengths = [len(c['text']) for c in chunks]
        report['strategies'][strategy] = {'chunk_count': len(chunks), 'characters_min': min(lengths), 'characters_median': float(np.median(lengths)), 'characters_p95': float(np.percentile(lengths, 95)), 'characters_max': max(lengths), 'chunks_with_split_code': sum(c['split_code'] for c in chunks), 'vector_bytes': sum(v.nbytes for c, v in rows if c['strategy'] == strategy), **meta['build'][strategy]}
    for question, vector in zip(questions, vectors):
        result = dict(question, results={})
        for strategy in strategies:
            ranked = rank(rows, vector, strategy)
            result['results'][strategy] = {'metrics': metrics(ranked, labels[question['id']]), 'top5': [{'document_id': c['document_id'], 'sections': c['sections'], 'score': c['score'], 'line_start': c['line_start'], 'line_end': c['line_end'], 'preview': c['text'][:240], 'source': docs[c['document_id']]['url']} for c in ranked[:5]]}
        report['questions'].append(result)
    for strategy in strategies:
        scores = [q['results'][strategy]['metrics'] for q in report['questions'] if q['results'][strategy]['metrics'] is not None]
        report['strategies'][strategy]['averages'] = {key: float(np.mean([m[key] for m in scores])) for key in scores[0]}
    output.mkdir(parents=True, exist_ok=True)
    (output / 'comparison.json').write_text(json.dumps(report, indent=2, ensure_ascii=False))
    lines = ['# Chunking comparison', '', 'Corpus: **{documents} documents**, **{unique_prose_words:,} unique prose words**, **{equivalent_pages:.1f} equivalent pages** (500 words/page).'.format(**meta['corpus']), '', 'Model: `' + meta['embedding']['model'] + '`; digest: `' + meta['embedding']['digest'] + '`.', '', '| Metric | Fixed | Semantic |', '| --- | ---: | ---: |']
    for key in ('chunk_count', 'characters_min', 'characters_median', 'characters_p95', 'characters_max', 'chunks_with_split_code', 'vector_bytes', 'seconds', 'embedding_calls'):
        values = [report['strategies'][s][key] for s in ('fixed', 'semantic')]
        lines.append('| ' + key + ' | ' + ' | '.join(f'{v:.2f}' if isinstance(v, float) else str(v) for v in values) + ' |')
    for key in report['strategies']['fixed']['averages']:
        lines.append('| ' + key + ' | ' + ' | '.join(f"{report['strategies'][s]['averages'][key]:.3f}" for s in ('fixed', 'semantic')) + ' |')
    lines += ['', '## Interpretation', '', 'The table reports measured results, without assuming either chunker wins. Relevance means overlap with manually selected source sections; it does not establish correctness of generated code. Both strategies use identical query vectors and exact cosine ranking. Coverage uses the first 6,000 retrieved text characters, truncating the final chunk and deduplicating overlapping source spans. This is a character budget, not a model-token budget.', '', 'Indexing times include boundary detection, cached embedding lookups and database insertion. Semantic runs after fixed and can reuse identical cached inputs; embedding-call counts describe actual calls in this run, not a controlled cold-cache speed benchmark. The full SQLite file contains both strategies and document snapshots; vector bytes provide the directly comparable per-strategy storage.', '', '## Per-question evidence', '', '| Question | Fixed hit@5 | Semantic hit@5 |', '| --- | ---: | ---: |']
    for q in report['questions']:
        values = [q['results'][s]['metrics'] for s in ('fixed', 'semantic')]
        lines.append('| ' + q['query'].replace('|', '\\|') + ' | ' + ' | '.join('unanswerable' if v is None else str(int(v['hit_at_5'])) for v in values) + ' |')
    lines += ['', '## Representative results', '']
    # Include disagreements/failures first, plus successful and out-of-corpus examples.
    ordered = sorted(report['questions'], key=lambda q: (q['results']['fixed']['metrics'] is None, sum((q['results'][s]['metrics'] or {}).get('hit_at_5', 0) for s in strategies)))
    chosen = ordered[:3] + [q for q in ordered if q['results']['fixed']['metrics'] and all(q['results'][s]['metrics']['hit_at_5'] for s in strategies)][:1] + [q for q in ordered if q['results']['fixed']['metrics'] is None]
    for q in {q['id']: q for q in chosen}.values():
        lines += ['### ' + q['query'], '']
        for strategy in strategies:
            top = q['results'][strategy]['top5'][0]
            lines += [f"- **{strategy}** first result: [{top['document_id']}]({top['source']}), lines {top['line_start']}–{top['line_end']}; score {top['score']:.3f}. " + ' / '.join(top['sections'])]
        lines.append('')
    lines += ['Out-of-corpus questions still produce nearest neighbors. No answerability threshold or refusal capability is claimed. Inspect the JSON report for all top-five results and scores.', '']
    (output / 'comparison.md').write_text('\n'.join(lines))
    return report['strategies']
