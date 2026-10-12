#!/usr/bin/env python3
"""Staged local scorer experiments. Run after make test && make build; resumable."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.request
from score_bench_metrics import metrics, eligible

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT/'results/task3'
BINARY = ROOT/'advent-agent'


def write_once(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists() and path.read_text() != text:
        raise RuntimeError(f'Refusing to overwrite different {path}')
    if not path.exists():
        path.write_text(text)


def dump(path, value):
    temp = path.with_suffix(path.suffix+'.tmp')
    temp.write_text(json.dumps(value, indent=2)+'\n')
    temp.replace(path)


def api(name):
    try:
        with urllib.request.urlopen('http://127.0.0.1:11434/api/'+name, timeout=5) as r:
            return json.load(r)
    except Exception as e:
        return {'error':str(e)}


def reset_model():
    model='hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M'
    request=urllib.request.Request('http://127.0.0.1:11434/api/generate',
        data=json.dumps({'model':model,'keep_alive':0}).encode(),headers={'Content-Type':'application/json'})
    with urllib.request.urlopen(request,timeout=60) as response:
        result=json.load(response)
    deadline=time.monotonic()+30
    while time.monotonic()<deadline:
        state=api('ps')
        if 'error' in state:raise RuntimeError(state['error'])
        if not any(m.get('name')==model for m in state.get('models',[])):
            return result
        time.sleep(1)
    raise RuntimeError('Ollama model did not unload; refusing uncontrolled measurement')


def sample():
    value = {'time':time.time(),'ollama':api('ps')}
    for key,cmd in [('swap',['sysctl','vm.swapusage']),('vm',['vm_stat']),
                    ('processes',['ps','-axo','pid=,rss=,comm='])]:
        p = subprocess.run(cmd,text=True,capture_output=True)
        text = p.stdout.strip()
        if key=='processes':
            text='\n'.join(s for s in text.splitlines() if 'ollama' in s.lower() or 'advent-agent' in s)
        value[key]=text or p.stderr.strip()
    return value


def profile(name, scorer):
    profile_name = 'task3-'+name if OUT == ROOT/'results/task3' else 'task3-'+hashlib.sha256(json.dumps(scorer,sort_keys=True).encode()).hexdigest()[:8]+'-'+name
    base = (ROOT/'data/task2-config/advent-agent/profiles/local-rag/config.toml').read_text()
    base = base.replace(str(ROOT/'data/task2-state/advent-agent/local-rag'), str(ROOT/'data/task3-state/advent-agent'/profile_name))
    if name=='spot':
        base=base.replace('request_timeout = "10m"','request_timeout = "90s"')
        if 'request_timeout = "90s"' not in base:
            raise RuntimeError('Could not set the answer spot-check deadline')
    if scorer:
        base+='\n[rag.scorer]\n'+''.join(f'{k} = {json.dumps(v)}\n' for k,v in scorer.items())
    path=ROOT/'data/task3-config/advent-agent/profiles'/profile_name/'config.toml'
    write_once(path,base)
    write_once(OUT/'configs'/(name+'.toml'),base)
    return profile_name


def run(name, scorer, stage, pass_num=1, questions=''):
    target=OUT/f'{stage}-{name}-{pass_num}.json'
    envfile=target.with_suffix('.environment.json')
    prof=profile(name,scorer)
    env=dict(os.environ,XDG_CONFIG_HOME=str(ROOT/'data/task3-config'),XDG_STATE_HOME=str(ROOT/'data/task3-state'))
    env.pop('DEEPSEEK_API_KEY',None)
    if stage in ('answers','spot'):
        suite=OUT/'spot-suite.json' if stage=='spot' else ROOT.parent/'week4/rag-grounding-suite.json'
        cmd=[str(BINARY),'--profile',prof,'rag','compare','--matrix','repeated','--runs','1',
             '--rag-rerank','--suite',str(suite),'--output',str(target)]
    else:
        cmd=[str(BINARY),'--profile',prof,'rag','score-bench','--input',str(OUT/'candidates.json'),
             '--runs','1','--output',str(target)]
        if questions:cmd+=['--questions',questions]
        if stage=='bounded':
            cmd+=['--baseline',str(OUT/'screen-baseline-1.json'),'--timeout-factor','0.6','--warmup-timeout','60s']
    digest=hashlib.sha256(BINARY.read_bytes()).hexdigest()
    if envfile.exists():
        capture=json.loads(envfile.read_text())
        if capture['binary_sha256']!=digest or capture['command']!=cmd:
            raise RuntimeError('Binary/command changed; use a new output directory')
        if 'exit_code' in capture:
            return target
    else:
        capture={'binary_sha256':digest,'command':cmd,'version':api('version'),'tags':api('tags'),'samples':[]}
    capture['cache_policy']='unload model before configuration, then recorded warmup (scoring only)'
    capture['reset']=reset_model()
    capture['samples'].append(sample());dump(envfile,capture)
    print('START',target.name,scorer,flush=True)
    with target.with_suffix('.log').open('a') as log:
        child=subprocess.Popen(cmd,cwd=ROOT,env=env,stdout=log,stderr=log)
        try:
            while child.poll() is None:
                try:child.wait(timeout=5)
                except subprocess.TimeoutExpired:pass
                capture['samples'].append(sample());dump(envfile,capture)
        except BaseException:
            child.send_signal(2);child.wait(timeout=30);dump(envfile,capture);raise
    capture['exit_code']=child.returncode;dump(envfile,capture)
    print('END',target.name,'exit',child.returncode,flush=True)
    return target


def read_metrics(path, labels):
    try:
        return metrics(json.loads(path.read_text()),labels)
    except (ValueError, KeyError) as e:
        return {'errors':1,'reason':str(e),'median_seconds':None}


def screen():
    labels=json.loads((OUT/'labels.json').read_text())
    questions='q01,q13,q19,q23'
    records=[]
    base=read_metrics(run('baseline',{},'screen',questions=questions),labels)
    if base['errors']:raise RuntimeError(f'Baseline failed: {base}')
    records.append({'name':'baseline','scorer':{},'metrics':base})
    best=records[0]
    def test(name,scorer):
        nonlocal best
        m=read_metrics(run(name,scorer,'screen',questions=questions),labels)
        record={'name':name,'scorer':scorer,'metrics':m};records.append(record)
        if not m['errors'] and eligible(m,base) and m['median_seconds']<best['metrics']['median_seconds']:
            best=record
        dump(OUT/'screening.json',{'records':records,'selected':best,'baseline':records[0]})
        print('METRICS',name,json.dumps(m), 'best',best['name'],flush=True)
    test('temperature0',dict(best['scorer'],temperature=0))
    test('compact',dict(best['scorer'],prompt_version='compact-v1'))
    batching_base=dict(best['scorer'])
    for size in (4,5):test(f'batch{size}',dict(batching_base,batch_size=size))
    context_base=dict(best['scorer'])
    for context in (4096,8192):test(f'context{context}',dict(context_base,context_window_tokens=context))
    test('output',dict(best['scorer'],output_tokens=32+32*best['scorer'].get('batch_size',2)))


def choose_finalist(screening):
    """Always compare a real change, even when screening retained baseline."""
    selected=screening['selected']
    if selected['name']!='baseline':
        return selected
    baseline=screening['baseline']['metrics']
    candidates=[r for r in screening['records'] if r['name']!='baseline'
                and not r['metrics']['errors'] and eligible(r['metrics'],baseline)]
    if not candidates:
        raise RuntimeError('No modified configuration passed screening quality; no finalist to compare')
    return min(candidates,key=lambda r:r['metrics']['median_seconds'])


def screen_extra():
    labels=json.loads((OUT/'labels.json').read_text())
    screening=json.loads((OUT/'screening.json').read_text())
    scorer=dict(temperature=0,prompt_version='compact-v2',batch_size=4,context_window_tokens=8192,output_tokens=64)
    result=read_metrics(run('ordered',scorer,'screen',questions='q01,q13,q19,q23'),labels)
    record={'name':'ordered','scorer':scorer,'metrics':result}
    screening['records']=[r for r in screening['records'] if r['name']!='ordered']+[record]
    baseline=screening['baseline']['metrics']
    if not result['errors'] and eligible(result,baseline) and result['median_seconds']<screening['selected']['metrics']['median_seconds']:
        screening['selected']=record
    screening['finalist']=choose_finalist(screening)
    screening['additional_experiment']='Combined compact ordered output, temperature zero, batch four and 8192 context after individual variants did not improve latency.'
    dump(OUT/'screening.json',screening)
    print('EXTRA',json.dumps(record),'FINALIST',screening['finalist']['name'],flush=True)


def repeated(stage):
    screening=json.loads((OUT/'screening.json').read_text())
    selected=choose_finalist(screening)
    screening['finalist']=selected
    dump(OUT/'screening.json',screening)
    # Repeat baseline versus the fastest quality-eligible modified configuration.
    arms=[('baseline',{}),('finalist',selected['scorer'])]
    for n in range(1,4):
        for name,scorer in (arms if n%2 else list(reversed(arms))):
            run(name,scorer,stage,n)


def main():
    global OUT
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('stage',choices=['screen','extra','bounded','spot','final','answers','all'])
    p.add_argument('--output-dir',type=Path,default=OUT,help='separate experiment directory containing candidates.json and labels.json')
    args=p.parse_args()
    OUT=args.output_dir.resolve()
    if args.stage in ('screen','all'):screen()
    if args.stage in ('extra','all'):screen_extra()
    if args.stage in ('bounded','all'):
        screening=json.loads((OUT/'screening.json').read_text())
        run('finalist',choose_finalist(screening)['scorer'],'bounded',questions='q01,q13,q19,q23')
    if args.stage=='spot':
        labels=json.loads((OUT/'labels.json').read_text())
        b=read_metrics(OUT/'screen-baseline-1.json',labels)
        f=read_metrics(OUT/'bounded-finalist-1.json',labels)
        if not eligible(f,b) or b['median_seconds']/f['median_seconds']<2:
            raise RuntimeError('Bounded scoring gate failed; skipping answer generation')
        suite=json.loads((ROOT.parent/'week4/rag-grounding-suite.json').read_text())
        suite['questions']=[q for q in suite['questions'] if q['id'] in ('q01','q23')]
        suite['selection']='Two bounded answer spot checks: q01 supported, q23 unsupported. Not a full-suite comparison.'
        write_once(OUT/'spot-suite.json',json.dumps(suite,indent=2)+'\n')
        screening=json.loads((OUT/'screening.json').read_text())
        run('spot',choose_finalist(screening)['scorer'],'spot')
    # The expensive repeated stages now require explicit selection.
    if args.stage=='final':repeated('final')
    if args.stage=='answers':repeated('answers')


if __name__=='__main__':main()
