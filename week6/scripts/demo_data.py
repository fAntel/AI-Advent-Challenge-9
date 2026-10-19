#!/usr/bin/env python3
"""Run the Week6 data MCP demo with the SDK test account, without displaying credentials."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

from test_data_mcp import MCP, payload

ROOT = Path(__file__).resolve().parents[1]
APP = ROOT / 'data-mcp/build/package/healbe-data-mcp.app/Contents/MacOS/healbe-data-mcp'


def credentials(sdk):
    source = sdk / 'serverApi/src/commonTest/kotlin/com/healbe/healbesdk/serverApi/AccessIdSetter.kt'
    text = source.read_text()
    email = re.search(r'const\s+val\s+TEST_EMAIL\s*=\s*"([^"\n]+)"', text)
    password = re.search(r'\.authUser\(TEST_EMAIL,\s*"([^"\n]+)"\)', text)
    if not email or not password:
        raise RuntimeError('Cannot identify the SDK test account; supply HEALBE_LOGIN/HEALBE_PASSWORD instead.')
    return email.group(1), password.group(1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--sdk', type=Path, default=Path.home()/'AndroidStudioProjects/healbe-sdk-android')
    p.add_argument('--output', type=Path, default=ROOT/'results/task4-data-demo.json')
    p.add_argument('--probe', action='store_true', help='Authenticate and read only; no model calls or uploads')
    p.add_argument('--chat', action='store_true', help='Interactive advent-agent with the private demo account configured')
    p.add_argument('--prompt', help='Override the scenario for a fresh capture')
    args = p.parse_args()
    if not APP.exists(): raise SystemExit('Build data-mcp packageApp first.')
    if not args.probe and not args.chat and args.output.exists(): raise SystemExit('Refusing to overwrite capture.')
    login, password = (os.getenv('HEALBE_LOGIN'), os.getenv('HEALBE_PASSWORD'))
    if not login or not password: login, password = credentials(args.sdk)
    env = dict(os.environ, HEALBE_LOGIN=login, HEALBE_PASSWORD=password)
    env.pop('HEALBE_ENDPOINT',None)  # This demo explicitly uses the real SDK default server.
    env.pop('JAVA_HOME',None)  # Exercise the bundled runtime.
    def redact(text): return text.replace(password,'[redacted]').replace(login,'[test-account]')
    now = datetime.now(timezone.utc)
    start = (now-timedelta(days=30)).replace(hour=9,minute=17,second=now.second,microsecond=0)
    start_s = int(start.timestamp())
    if args.probe:
        m = MCP(env, APP)
        try:
            result = payload(m.call('download_weights_by_measurement_time', {'fromTime':start_s,'limit':1}))
            print('Bundled-runtime authentication and weight download succeeded. Available records:',result['total'])
        finally: m.close()
        return
    # Build freshness protects the video against demonstrating stale Go/JVM artifacts.
    newest_go = max(f.stat().st_mtime for f in list(ROOT.glob('*.go'))+list((ROOT/'cmd').rglob('*.go')))
    newest_kotlin = max(f.stat().st_mtime for f in (ROOT/'data-mcp/src').rglob('*.kt'))
    if any((ROOT/n).stat().st_mtime < newest_go for n in ('advent-agent','advent-agentd')) or APP.stat().st_mtime < newest_kotlin:
        raise SystemExit('Artifacts are stale; rebuild before the demo.')
    prompt = args.prompt or (
        f'Use the privately configured test account. Create synthetic test fixtures: exactly 30 daily weigh-ins, '
        f'one per day starting at Unix second {start_s} ({start.isoformat()}), about 80 kg initially and '
        'about 80.7 kg finally, with plausible small daily variation. Include additionalData.muscleMass '
        'starting around 34 kg and ending around 34.4 kg, with small variation. These are synthetic values, '
        'not a medical prediction. Use source HEALBE. First read existing weights; preserve all existing '
        'records and shift the seconds within each day if any timestamp collides. Generate the values yourself '
        'and use the weight batch tool to upload them. Then download and verify all 30 measurement times, '
        'weights and muscleMass values. If verification differs, report it; do not blindly replay uploads. '
        'Do not read project files or ask for credentials. Finish with a concise summary and evidence.'
    )
    with tempfile.TemporaryDirectory(prefix='w6-data-',dir='/tmp') as tmp:
        tmp = Path(tmp).resolve(); state = tmp/'state'; project = tmp/'project'; project.mkdir()
        config = tmp/'config.toml'
        config.write_text('state = '+json.dumps(str(state))+'\ndata_mcp = '+json.dumps(str(APP))+'\ntimeout_seconds = 600\n')
        cli = [str(ROOT/'advent-agent'),'--config',str(config),'--project',str(project)]
        try:
            if args.chat:
                print('Test account configured privately. Example prompt:\n'+prompt+'\n',flush=True)
                subprocess.run(cli,env=env,check=True)
                return
            report = {'at':now.isoformat(),'library':'com.healbe:server-api-jvm:1.6.0','prompt':prompt,'passed':False,
                      'artifacts':{str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest() for f in [ROOT/'advent-agent',ROOT/'advent-agentd',APP]}}
            print('Running DeepSeek → advent-agent → bundled JVM MCP → real HEALBE server…',flush=True)
            r = subprocess.run(cli+['-p',prompt],env=env,capture_output=True,text=True,timeout=660)
            print(redact(r.stdout),flush=True)
            report.update(returncode=r.returncode,stdout=redact(r.stdout),stderr=redact(r.stderr))
            sessions = list((state/'sessions').glob('*.json'))
            if sessions:
                session = json.loads(sessions[0].read_text())
                report['session'] = session
                generated = []
                for op in session['operations']:
                    for tool in op.get('tools',[]):
                        if tool['name'] == 'healbe_add_batch_weight': generated.extend(json.loads(tool['arguments'])['data'])
                        elif tool['name'] == 'healbe_add_weight': generated.append(json.loads(tool['arguments'])['data'])
                report['generated_count'] = len(generated)
                if generated:
                    m = MCP(env,APP)
                    try:
                        downloaded = []; offset = 0
                        while True:
                            page = payload(m.call('download_weights_by_measurement_time',{'fromTime':min(w['measurementTime'] for w in generated),'limit':50,'offset':offset}))
                            downloaded.extend(page['records'])
                            if not page['has_more']: break
                            offset = page['next_offset']
                            if offset>10000: raise RuntimeError('Read-back limit exceeded')
                        by_time = {w['measurementTime']:w for w in downloaded}
                        checks=[]
                        for w in generated:
                            got=by_time.get(w['measurementTime'],{})
                            mass=got.get('additionalData',{}).get('muscleMass')
                            expected=w.get('additionalData',{}).get('muscleMass')
                            checks.append({'measurementTime':w['measurementTime'],'weight_matches':abs(got.get('weight',-1000)-w['weight'])<0.011,
                                           'muscle_matches':mass is not None and expected is not None and abs(mass-expected)<0.011})
                        report['readback_checks']=checks
                        report['passed']=r.returncode==0 and len(generated)==30 and len(by_time.keys() & {w['measurementTime'] for w in generated})==30 and all(c['weight_matches'] and c['muscle_matches'] for c in checks) and all(op.get('answer',{}).get('status')=='answered' for op in session['operations'])
                    finally: m.close()
            args.output.parent.mkdir(parents=True,exist_ok=True)
            args.output.write_text(redact(json.dumps(report,indent=2,ensure_ascii=False))+'\n')
            print('Capture:',args.output,'Verified:',report['passed'])
            if not report['passed']: raise SystemExit(1)
        finally:
            subprocess.run([str(ROOT/'advent-agentd'),'--config',str(config),'stop'],env=env,capture_output=True,timeout=15)

if __name__=='__main__': main()
