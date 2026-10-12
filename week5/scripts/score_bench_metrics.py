"""Label-bound relevance metrics; failed trials receive zero delivered recall."""
import hashlib
import math
import statistics


def median(values):
    return statistics.median(values) if values else None


def metrics(run, labels):
    if run['input_sha256'] != labels['input_sha256']:
        raise ValueError('Stale relevance labels')
    trials = run['trials']
    expected = run['passes'] * len(run['questions'])
    if len(trials) != expected or not all(t['completed'] for t in trials):
        raise ValueError('Incomplete benchmark')
    totals = dict(relevant=0, retained=0, selected_relevant=0, selected=0, packed_relevant=0,
                  false_positive_controls=0, errors=0, prompt_tokens=0, output_tokens=0, calls=0,
                  prompt_seconds=0, generation_seconds=0, load_seconds=0)
    ndcgs, seconds = [], []
    observations = {}
    for t in trials:
        gold = labels['questions'][t['question_id']]
        relevant = {k for k, v in gold.items() if v >= 2}
        totals['relevant'] += len(relevant)
        p = t.get('preparation') or {}
        for c in p.get('calls', []):
            totals['calls'] += 1
            totals['prompt_seconds'] += c['call'].get('prompt_duration',0)/1e9
            totals['generation_seconds'] += c['call'].get('eval_duration',0)/1e9
            totals['load_seconds'] += c['call'].get('load_duration',0)/1e9
            u = c['call'].get('usage', {})
            totals['prompt_tokens'] += u.get('prompt_tokens', 0)
            totals['output_tokens'] += u.get('completion_tokens', 0)
        if t.get('error'):
            totals['errors'] += 1
            if relevant:
                ndcgs.append(0)
            continue
        scores = p['scores']
        if set(scores) != set(gold):
            raise ValueError('Candidate labels do not match scoring output')
        kept = {k for k, v in scores.items() if v >= 2}
        packed = {c['id'] for c in t['evidence']['chunks']}
        totals['retained'] += len(kept & relevant)
        totals['selected_relevant'] += len(kept & relevant)
        totals['selected'] += len(kept)
        totals['packed_relevant'] += len(packed & relevant)
        if not relevant:
            totals['false_positive_controls'] += len(kept)
        ranked = sorted(p['candidates'], key=lambda c: -scores[c['id']])[:5]
        def dcg(values):
            return sum((2**v-1)/math.log2(i+2) for i,v in enumerate(values))
        ideal = dcg(sorted(gold.values(), reverse=True)[:5])
        if ideal:
            ndcgs.append(dcg([gold[c['id']] for c in ranked])/ideal)
        seconds.append(t['duration']/1e9)
        for k,v in scores.items():
            observations.setdefault((t['question_id'],k), []).append(v)
    return dict(totals, trials=len(trials), median_seconds=median(seconds),
                recall=totals['retained']/totals['relevant'] if totals['relevant'] else 1,
                precision=totals['selected_relevant']/totals['selected'] if totals['selected'] else None,
                packed_recall=totals['packed_relevant']/totals['relevant'] if totals['relevant'] else 1,
                ndcg5=statistics.mean(ndcgs) if ndcgs else None,
                varying_pairs=sum(len(set(v))>1 for v in observations.values()))


def eligible(candidate, baseline):
    return (candidate['errors']==0 and candidate['recall'] >= baseline['recall']-0.05-1e-9
            and candidate['packed_recall'] >= baseline['packed_recall']-0.05-1e-9)


def packed_ids(preparation, rerank=True, limit=5, budget=6000):
    """Offline selection audit; renderer verifies it against Go's saved evidence."""
    chunks=preparation['candidates']
    if rerank:
        chunks=sorted(chunks,key=lambda c:-preparation['scores'][c['id']])
    chunks=[c for c in chunks if preparation['scores'][c['id']]>=2][:limit]
    packed=[];used=0
    for c in chunks:
        version='unknown' if c.get('release_version') is None else c['release_version']
        ref=(f"[{len(packed)+1}] {c['title']} — {' / '.join(c.get('sections') or [])}\n"
             f"{c['source']} (normalized source lines {c['line_start']}-{c['line_end']}; release {version}; availability {c['availability']}; chunk_id: {c['id']})\n")
        size=len(ref+c['text']+'\n\n')
        if used+size<=budget:
            packed.append(c['id']);used+=size
    return packed


def ranking_effect(run,labels):
    relevant=ranked=original=changed=0
    for t in run['trials']:
        gold=labels['questions'][t['question_id']]
        relevant+=sum(v>=2 for v in gold.values())
        if t.get('error'):continue
        p=t['preparation'];actual=[c['id'] for c in t['evidence']['chunks']]
        computed=packed_ids(p)
        if computed!=actual:
            raise ValueError('Offline selection audit differs from production evidence')
        old=packed_ids(p,rerank=False)
        ranked+=sum(gold[k]>=2 for k in actual)
        original+=sum(gold[k]>=2 for k in old)
        changed+=set(old)!=set(actual)
    return {'reranked_packed_recall':ranked/relevant,'original_order_packed_recall':original/relevant,'changed_evidence_sets':changed}
