#!/usr/bin/env python3
"""Render Task3 measurements; semantic totals require hash-bound answer reviews."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import statistics
from score_bench_metrics import metrics, eligible, ranking_effect

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'results/task3'


def fingerprint(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()


def merge(arm):
    runs=[json.loads((OUT/f'final-{arm}-{n}.json').read_text()) for n in range(1,4)]
    run=dict(runs[0]);run['passes']=3;run['trials']=[]
    for n,r in enumerate(runs,1):
        if any(r[k]!=runs[0][k] for k in ('input_sha256','settings','scorer','context_window','ollama','questions')):
            raise ValueError('Final benchmark inputs differ')
        run['trials'].extend(dict(t,pass_number=n) for t in r['trials'])
    return run


def valid_review(review, trial):
    controls = ('q21','q22','q23')
    expected = 0 if trial['question_id'] in controls else 3
    if not isinstance(review,dict) or review.get('sha256') != fingerprint(trial):
        return False
    if (review.get('support') not in ('yes','partial','no','N/A')
            or not isinstance(review.get('correct_unknown'),bool)
            or not isinstance(review.get('notes'),str) or not review['notes'].strip()
            or not isinstance(review.get('facts'),list) or len(review['facts']) != expected
            or any(v not in ('yes','partial','no') for v in review['facts'])):
        return False
    result=trial.get('answer',{}).get('result',{})
    if trial.get('error') and (review['support']!='N/A' or review['correct_unknown'] or any(v!='no' for v in review['facts'])):
        return False
    if review['correct_unknown'] and (trial['question_id'] not in controls or result.get('status')!='unknown'):
        return False
    return True


def pct(value):
    return 'N/A' if value is None else f'{100*value:.1f}%'



def render_bounded(screen, labels):
    baseline=json.loads((OUT/'screen-baseline-1.json').read_text())
    finalist=json.loads((OUT/'bounded-finalist-1.json').read_text())
    if finalist['baseline_sha256']!=hashlib.sha256((OUT/'screen-baseline-1.json').read_bytes()).hexdigest():
        raise ValueError('Baseline changed after deadline calculation')
    if baseline['questions']!=finalist['questions']:
        raise ValueError('Mismatched bounded questions')
    digests=[]
    for name in ('screen-baseline-1','bounded-finalist-1'):
        capture=json.loads((OUT/(name+'.environment.json')).read_text())
        if capture.get('cache_policy')!='unload model before configuration, then recorded warmup (scoring only)':
            raise ValueError('Mismatched cache policy')
        digests.append({m['digest'] for m in capture['tags']['models'] if m['name']=='hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M'})
    if len(digests[0])!=1 or digests[0]!=digests[1]:
        raise ValueError('Missing or changed model digest')
    b,f=metrics(baseline,labels),metrics(finalist,labels)
    speedup=b['median_seconds']/f['median_seconds'] if f['median_seconds'] else None
    qualifies=eligible(f,b) and speedup is not None and speedup>=2
    summary={'method':'one bounded pass, recorded controlled baseline', 'baseline':b,'finalist':f,
             'speedup':speedup,'scoring_qualifies':qualifies,'adopt':False,
             'reason':'Bounded scoring validation; full answer quality is not established.'}
    lines=['# Task3 — Bounded local scorer optimization','',
        'The long repeated experiment was stopped at the user’s request. This report reuses the controlled four-question baseline and runs the ordered-score finalist once, with a separate deadline of 60% of each baseline duration. Warmup is capped at 60 seconds and excluded. Timeouts are terminal failures, receive zero delivered recall, and cannot qualify by being fast.', '',
        'Spark Q4_K_M weights and quantization are unchanged. Both arms use the same frozen passages and score sorting. Labels were authored by the implementing assistant before tuning; these four examples were also used for selection, so this is a development check, not a held-out evaluation.', '',
        '## Settings','', '```json',json.dumps(screen['finalist']['scorer'],indent=2),'```','',
        '## Bounded validation','', '| Question | Baseline | Deadline | Finalist | Result |', '| --- | ---: | ---: | ---: | --- |']
    reference={t['question_id']:t for t in baseline['trials']}
    for t in finalist['trials']:
        lines.append(f"| {t['question_id']} | {reference[t['question_id']]['duration']/1e9:.2f} s | {t['budget']/1e9:.2f} s | {t['duration']/1e9:.2f} s | {t.get('error','Completed')} |")
    lines+=['','| Measurement | Baseline | Finalist |','| --- | ---: | ---: |']
    for key,title in [('median_seconds','Successful-trial median seconds'),('recall','Filter recall'),('precision','Filter precision'),('packed_recall','Packed evidence recall'),('ndcg5','nDCG@5'),('false_positive_controls','Retained chunks on unsupported control'),('errors','Failed trials'),('prompt_tokens','Recorded input tokens'),('output_tokens','Recorded output tokens'),('calls','Recorded calls')]:
        values=[pct(m[key]) if key in ('recall','precision','packed_recall','ndcg5') else str(round(m[key],2)) if m[key] is not None else 'N/A' for m in (b,f)]
        lines.append(f'| {title} | {values[0]} | {values[1]} |')
    lines+=['',f"Median scoring speedup: **{speedup:.2f}×**." if speedup is not None and not f['errors'] else 'No overall speedup claim: one or more trials failed.', '', f"Scoring gate: **{'passed' if qualifies else 'not met'}**. The target remains 2× median speedup and no more than a five-percentage-point drop in filter or packed recall, with no failures. The 0.6 deadline is a permissive cutoff (1.67×), not the success target. Successful-only medians must not be interpreted as an overall speedup when other trials fail.", '',
        'Answer generation settings are unchanged, but evidence can change. Answer-quality equivalence has not been established; the existing default profile remains in place. The cancelled three-pass run and older uncontrolled-cache pilot are excluded. Token counts on failed trials cover only recorded calls, not necessarily all interrupted server work.', '',
        '## Screening','', '| Variant | Median seconds | Filter recall | Packed recall | Errors |','| --- | ---: | ---: | ---: | ---: |']
    for r in screen['records']:
        m=r['metrics']; sec=m.get('median_seconds')
        lines.append(f"| {r['name']} | {sec:.2f} | {pct(m['recall'])} | {pct(m['packed_recall'])} | {m['errors']} |" if sec else f"| {r['name']} | Failed | — | — | {m['errors']} |")
    lines+=['', 'The ordered variant’s screening median was approximately 2× faster, with equal packed recall and higher filtering recall, but lower filtering precision. Screening selected the candidate; bounded validation determines whether it meets the runtime constraint.', '',
        '## Resources','', '| Run | Peak sampled model allocation | Peak sampled Ollama RSS | System swap range |','| --- | ---: | ---: | ---: |']
    for name in ('screen-baseline-1','bounded-finalist-1'):
        capture=json.loads((OUT/(name+'.environment.json')).read_text()); allocations=[];rss=[];swap=[]
        for sample in capture['samples']:
            allocations.append(sum(m.get('size',0) for m in sample.get('ollama',{}).get('models',[])))
            rss.append(sum(int(s.split()[1])*1024 for s in sample.get('processes','').splitlines() if 'ollama' in s.lower() and len(s.split())>=2))
            match=re.search(r'used = ([\d.]+)([MG])',sample.get('swap',''))
            if match:swap.append(float(match[1])*(1024 if match[2]=='G' else 1))
        swap_text=f'{min(swap):.1f}–{max(swap):.1f} MiB' if swap else 'Unavailable'
        lines.append(f'| {name} | {max(allocations,default=0)/2**20:.1f} MiB | {max(rss,default=0)/2**20:.1f} MiB | {swap_text} |')
    lines+=['', 'Five-second samples include warmup and can miss peaks. Allocation and RSS are distinct and must not be added. Swap is system-wide. Both runs unload the model before warmup; separate executions still vary with system load and cache state.', '',
        '## Reproduction','', 'After `make test` and `make build`, use `python3 scripts/run-score-bench.py bounded`, then `python3 scripts/render-score-bench.py`. Existing completed captures are immutable; use a fresh output directory to run a rebuilt binary. The runner invokes `/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent` with isolated workspace profiles. The CLI accepts `--baseline`, `--timeout-factor 0.6`, and `--warmup-timeout 60s`; the saved baseline hash binds the per-question budgets.', '']
    spot_path=OUT/'spot-spot-1.json'
    if spot_path.exists():
        spot=json.loads(spot_path.read_text())
        historic=json.loads((ROOT/'results/task2/answers.json').read_text())
        review_path=OUT/'spot-review.json'
        reviews=json.loads(review_path.read_text()) if review_path.exists() else {}
        lines+=['## Answer spot checks','', 'Two full-pipeline trials, each capped at 90 seconds. Baseline answers below are historical Task2 first-pass outputs, not newly generated under controlled timing. These checks cannot establish full-suite equivalence.', '',
                '| Question | Arm | Result | Expected facts fully covered | Review |','| --- | --- | --- | ---: | --- |']
        stats=[]
        for arm, trials in [('historical baseline',[t for t in historic['trials'] if t['pass']==1 and t['question_id'] in ('q01','q23')]),('finalist',spot['trials'])]:
            for t in trials:
                key=arm+'/'+t['question_id'];review=reviews.get(key)
                bound=valid_review(review,t)
                result=t.get('answer',{}).get('result') or {}
                status='Failed: '+t['error'] if t.get('error') else result.get('status','Incomplete')
                facts=str(review['facts'].count('yes'))+'/3' if bound and t['question_id']=='q01' else 'N/A'
                notes=review['notes'] if bound else 'Not reviewed'
                lines.append(f"| {t['question_id']} | {arm} | {status} | {facts} | {notes} |")
                stats.append({'arm':arm,'question_id':t['question_id'],'status':status,'review_bound':bound})
        summary['answer_spot_checks']=stats
        lines+=['', 'Failed or unreviewed outputs receive no delivered-answer quality credit. Semantic reviews are hash-bound to the saved trials. The default profile is unchanged.', '']
    (OUT/'comparison.md').write_text('\n'.join(lines))
    (OUT/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')


def main():
    global OUT
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-dir',type=Path,default=OUT)
    OUT=parser.parse_args().output_dir.resolve()
    labels=json.loads((OUT/'labels.json').read_text())
    if hashlib.sha256((OUT/'candidates.json').read_bytes()).hexdigest()!=labels['input_sha256']:
        raise ValueError('Frozen candidates changed after labelling')
    screen=json.loads((OUT/'screening.json').read_text())
    if (OUT/'bounded-finalist-1.json').exists():
        render_bounded(screen,labels)
        return
    model_digests=set()
    for capture_path in OUT.glob('*.environment.json'):
        capture=json.loads(capture_path.read_text())
        if capture.get('cache_policy')!='unload model before configuration, then recorded warmup (scoring only)':
            raise ValueError('Mixed cache policies in final measurements')
        for model in capture.get('tags',{}).get('models',[]):
            if model.get('name')=='hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M':
                model_digests.add(model['digest'])
    if len(model_digests)!=1:
        raise ValueError('Missing or changed scoring model digest')

    lines=['# Task3 — Local relevance scorer optimization','',
           'Spark Q4_K_M is fixed. No quantization experiment or cloud calls. The first Task2 pass supplies the frozen candidates; labels were authored before tuning by the implementing assistant, not an independent human reviewer.',
           '', 'Model digest: `'+next(iter(model_digests))+'`.', '', '## Screening', '', '| Variant | Median scoring | Filter recall | Packed recall | Errors |', '| --- | ---: | ---: | ---: | ---: |']
    for r in screen['records']:
        m=r['metrics']; seconds=m.get('median_seconds')
        lines.append(f"| {r['name']} | {seconds:.2f} s | {pct(m['recall'])} | {pct(m['packed_recall'])} | {m['errors']} |" if seconds else f"| {r['name']} | Failed | — | — | {m['errors']} |")
    for record in screen['records']:
        if record['metrics'].get('errors'):
            path=OUT/f"screen-{record['name']}-1.json"
            if path.exists():
                raw=json.loads(path.read_text())
                reasons=[raw.get('warmup',{}).get('error','')]+[t.get('error','') for t in raw.get('trials',[])]
                for reason in sorted(set(filter(None,reasons))):
                    lines+=['',f"Rejected `{record['name']}`: {reason}"]
    lines+=['', 'Screening uses q01, q13, q19 and q23 once per variant, excluding warmup. It is noisy and selection-biased; the repeated comparison is the adoption test.', '',
            'Finalist scorer (or current screening selection if final testing has not started):','', '```json',json.dumps(screen.get('finalist',screen['selected'])['scorer'],indent=2),'```','']
    complete=all((OUT/f'final-{a}-{n}.json').exists() for a in ('baseline','finalist') for n in range(1,4))
    summary={}
    if complete:
        try:
            runs={a:merge(a) for a in ('baseline','finalist')}
            summary={a:metrics(r,labels) for a,r in runs.items()}
        except ValueError:
            complete=False
    if complete:
        b,f=summary['baseline'],summary['finalist']; speedup=(b['median_seconds'] or 0)/f['median_seconds'] if f['median_seconds'] else 0
        scoring_ok=eligible(f,b) and speedup>=2 and runs['baseline']['scorer']!=runs['finalist']['scorer']
        summary.update(speedup=speedup,scoring_qualifies=scoring_ok)
        effects={a:ranking_effect(r,labels) for a,r in runs.items()}
        summary['ranking_effect']=effects
        lines+=['## Repeated scoring comparison','', '| Measurement | Baseline | Finalist |','| --- | ---: | ---: |']
        for key,title in [('median_seconds','Median scoring (seconds)'),('recall','Filter recall'),('precision','Filter precision'),('packed_recall','Packed evidence recall'),('ndcg5','nDCG@5'),('false_positive_controls','Retained chunks on unsupported controls'),('errors','Failed trials'),('varying_pairs','Query/chunk pairs with varying scores'),('prompt_tokens','Input tokens'),('output_tokens','Output tokens'),('calls','Scoring calls'),('prompt_seconds','Total prompt evaluation (seconds)'),('generation_seconds','Total token generation (seconds)'),('load_seconds','Total recorded model loading (seconds)')]:
            values=[pct(summary[a][key]) if key in ('recall','precision','packed_recall','ndcg5') else str(round(summary[a][key],2)) if summary[a][key] is not None else 'N/A' for a in ('baseline','finalist')]
            lines.append(f'| {title} | {values[0]} | {values[1]} |')
        lines+=['',f'Scoring speedup: **{speedup:.2f}×**. Scoring adoption gate: **{"passed" if scoring_ok else "not met"}**.',
                '', 'Recall denominators include every labelled relevant candidate; packed recall measures those retained after the five-chunk limit and 6,000-character budget. Failed trials receive zero delivered recall and are excluded from successful-latency medians. nDCG@5 uses graded labels and stable score sorting before filtering. Both arms enable reranking. Each configuration starts after an explicit model unload, followed by a recorded warmup. Warmup is excluded from all scoring totals. The earlier uncontrolled-cache pilot is retained separately and excluded.']
        lines+=['', '| Question | Baseline median | Finalist median | Baseline filter recall | Finalist filter recall |', '| --- | ---: | ---: | ---: | ---: |']
        for question in runs['baseline']['questions']:
            qm={}
            for arm,run in runs.items():
                subset=dict(run,questions=[question],trials=[t for t in run['trials'] if t['question_id']==question])
                qm[arm]=metrics(subset,labels)
            times=[f"{qm[a]['median_seconds']:.2f} s" if qm[a]['median_seconds'] else 'Failed' for a in ('baseline','finalist')]
            recalls=[pct(qm[a]['recall']) if qm[a]['relevant'] else 'No relevant candidates' for a in ('baseline','finalist')]
            lines.append(f"| {question} | {times[0]} | {times[1]} | {recalls[0]} | {recalls[1]} |")
        lines+=['', 'Sorting-only audit (same scores, no new model calls):']
        for a,e in effects.items():
            lines.append(f"- {a}: packed recall {pct(e['original_order_packed_recall'])} in retrieval order → {pct(e['reranked_packed_recall'])} with score sorting; {e['changed_evidence_sets']} changed evidence sets.")
    else:
        lines+=['Repeated scoring comparison is incomplete; no adoption claim is available.']
    review_path=OUT/'answer-review.json';reviews=json.loads(review_path.read_text()) if review_path.exists() else {}
    answer_stats={}; all_reviewed=True
    lines+=['','## Full-pipeline comparison','','| Arm | Valid first attempts | Median trial | Correct unknown controls | Fully quote-supported factual answers | Expected facts fully covered |','| --- | ---: | ---: | ---: | ---: | ---: |']
    for arm in ('baseline','finalist'):
        trials=[]
        for n in range(1,4):
            path=OUT/f'answers-{arm}-{n}.json'
            if path.exists():
                for t in json.loads(path.read_text())['trials']:trials.append((f'{arm}/{n}/{t["question_id"]}',t))
        if len(trials)!=30 or not all(t['completed'] for _,t in trials):
            all_reviewed=False;lines.append(f'| {arm} | Incomplete | — | — | — | — |');continue
        valid=[t for _,t in trials if not t.get('error')]
        bound=[]
        for key,t in trials:
            review=reviews.get(key)
            if not valid_review(review,t):all_reviewed=False
            else:bound.append(review)
        factual=sum(t.get('answer',{}).get('result',{}).get('status')=='answered' for t in valid)
        controls=sum(t['question_id'] in ('q21','q22','q23') and t.get('answer',{}).get('result',{}).get('status')=='answered' for t in valid)
        st={'valid':len(valid),'median_seconds':statistics.median(t['duration']/1e9 for _,t in trials),'accepted_factual_controls':controls,'bound_reviews':len(bound)}
        answer_stats[arm]=st
        fully=sum(r.get('support')=='yes' for r in bound);facts=sum(r.get('facts',[]).count('yes') for r in bound);unknown=sum(r.get('correct_unknown',False) for r in bound)
        reviewed=len(bound)==30
        lines.append(f'| {arm} | {len(valid)}/30 | {st["median_seconds"]:.2f} s | {str(unknown)+"/9" if reviewed else "Unreviewed"} | {str(fully)+"/"+str(factual) if reviewed else "Unreviewed"} | {str(facts)+"/63" if reviewed else "Unreviewed"} |')
    summary['answers']=answer_stats
    summary['adopt']=bool(complete and summary.get('scoring_qualifies') and all_reviewed and len(answer_stats)==2 and answer_stats['finalist']['accepted_factual_controls']<=answer_stats['baseline']['accepted_factual_controls'])
    lines+=['','Rejected outputs receive no delivered-answer credit. Semantic reviews are hash-bound to saved trials; quote validation alone is insufficient. Finalist adoption: **'+('eligible' if summary['adopt'] else 'not established')+'**.','', '## Resources','', '| Arm | Peak sampled model allocation | Peak sampled Ollama RSS | Swap range |','| --- | ---: | ---: | ---: |']
    for arm in ('baseline','finalist'):
        allocations=[];rss=[];swap=[]
        for path in sorted(OUT.glob(f'final-{arm}-*.environment.json')):
            for sample in json.loads(path.read_text())['samples']:
                allocations.append(sum(m.get('size',0) for m in sample.get('ollama',{}).get('models',[])))
                rss.append(sum(int(s.split()[1])*1024 for s in sample.get('processes','').splitlines() if 'ollama' in s.lower() and len(s.split())>=2))
                match=re.search(r'used = ([\d.]+)([MG])',sample.get('swap',''))
                if match:swap.append(float(match[1])*(1024 if match[2]=='G' else 1))
        if allocations:lines.append(f'| {arm} | {max(allocations)/2**20:.1f} MiB | {max(rss)/2**20:.1f} MiB | {min(swap):.1f}–{max(swap):.1f} MiB |' if swap else f'| {arm} | {max(allocations)/2**20:.1f} MiB | {max(rss)/2**20:.1f} MiB | Unavailable |')
    lines+=['','Allocation and RSS are different measurements and must not be added. Five-second samples can miss peaks. Swap and VM statistics are system-wide, not attributable solely to this task. Runs share the same host and are not thermally controlled. Full-pipeline timings include context changes, model loading, retrieval, and validation.','', '## Reproduction','', 'Run `make test` and `make build`, then `python3 scripts/prepare-score-bench.py` and `python3 scripts/run-score-bench.py all`. The runner uses the absolute Week5 `advent-agent` binary, workspace-only profiles, and removes the cloud key from child environments. It refuses to resume measurements with a changed binary. Review saved answers before running this renderer.','']
    (OUT/'comparison.md').write_text('\n'.join(lines))
    (OUT/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')


if __name__=='__main__':main()
