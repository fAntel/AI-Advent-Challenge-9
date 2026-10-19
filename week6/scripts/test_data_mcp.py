"""Offline integration: real JVM/SDK/stdio MCP against a local HTTP fixture.
Run after data-mcp installDist; absent binaries skip this optional suite.
"""
import json
import os
from pathlib import Path
import subprocess
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parents[1]
EXE = Path(os.environ.get('DATA_MCP_EXE', ROOT / 'data-mcp/build/install/healbe-data-mcp/bin/healbe-data-mcp'))

class MCP:
    def __init__(self, env=None, executable=EXE):
        self.p = subprocess.Popen([str(executable)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, text=True, env=env)
        self.seq = 0
        self.request('initialize', {'protocolVersion':'2024-11-05', 'capabilities':{}, 'clientInfo':{'name':'test','version':'1'}})
        self.p.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n')
        self.p.stdin.flush()
    def request(self, method, params):
        self.seq += 1
        self.p.stdin.write(json.dumps({'jsonrpc':'2.0','id':self.seq,'method':method,'params':params})+'\n')
        self.p.stdin.flush()
        # read with a deadline so broken protocol/server cannot hang the test suite.
        import select
        if not select.select([self.p.stdout], [], [], 40)[0]:
            self.p.kill()
            raise TimeoutError('MCP response timeout')
        raw = self.p.stdout.readline()
        if not raw: raise RuntimeError('MCP exited: '+self.p.stderr.read()[:1000])
        return json.loads(raw)['result']
    def call(self, name, args): return self.request('tools/call', {'name':name, 'arguments':args})
    def close(self):
        self.p.stdin.close()
        try: self.p.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.p.kill(); self.p.wait()
        errors = self.p.stderr.read()
        self.p.stdout.close(); self.p.stderr.close()
        return errors

def payload(r):
    assert not r.get('isError'), r
    return json.loads(r['content'][0]['text'])

@unittest.skipUnless(EXE.exists(), 'Build data-mcp installDist first')
class MCPIntegration(unittest.TestCase):
    def setUp(self):
        self.requests = []
        self.weights = []
        self.fail_weight = None
        self.reject_enrichment = False
        owner = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_GET(self):
                from urllib.parse import urlparse, parse_qs
                owner.requests.append({'request':'download', 'query':parse_qs(urlparse(self.path).query), 'access_id':self.headers.get('accessid')})
                data = json.dumps({'groups':[], 'count':0, 'total':0}).encode()
                self.send_response(200); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(data))); self.end_headers(); self.wfile.write(data)
            def do_POST(self):
                b = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                owner.requests.append(b)
                code = b.get('request')
                r = {'response_code':0,'request':code,'timestamp':1700000000}
                if code == 1:
                    if b.get('password') == 'wrong': r['response_code'] = 1
                    else: r['access_id'] = 'token-'+b['email']
                elif code == 78:
                    if b['weight'] == owner.fail_weight: r['response_code'] = 1
                    else:
                        r['id'] = len(owner.weights)+1
                        owner.weights.append({'id':r['id'], 'ts':b['timestamp'], 'weight':b['weight'], 'source_id':b['source_id']})
                elif code == 79:
                    if owner.reject_enrichment: r['response_code'] = 1
                    else:
                        w = next(x for x in owner.weights if x['id']==b['id'])
                        w.update({k:v for k,v in b.items() if k in ('weight','height','additional_data','outer_id','device_id')})
                        r['last_update'] = 1700000000
                elif code == 77:
                    r['items'] = [w for w in owner.weights if w['ts'] >= b.get('from_t',0)]
                else: r.update({'groups':[], 'count':0,'total':0,'limit':50})
                data = json.dumps(r).encode()
                self.send_response(200); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(data))); self.end_headers(); self.wfile.write(data)
        self.server = ThreadingHTTPServer(('127.0.0.1',0),Handler)
        self.thread = threading.Thread(target=self.server.serve_forever,daemon=True); self.thread.start()
        env = dict(os.environ, HEALBE_ENDPOINT=f'http://127.0.0.1:{self.server.server_port}/', HEALBE_LOGIN='fixture-a', HEALBE_PASSWORD='fixture-secret')
        self.mcp = MCP(env)
    def tearDown(self):
        stderr = self.mcp.close()
        self.server.shutdown(); self.server.server_close(); self.thread.join()
        self.assertNotIn('fixture-secret',stderr)
        self.assertNotIn('token-fixture',stderr)
    def test_discovery(self):
        ts = self.mcp.request('tools/list',{})['tools']
        self.assertEqual(len(ts),25)
        self.assertEqual(len({t['name'] for t in ts}),25)
        self.assertFalse(self.requests)
    def test_auth_failure_does_not_write_or_leak(self):
        r = self.mcp.call('add_weight',{'login':'fixture-b','password':'wrong','data':{'weight':80,'measurementTime':1700000000}})
        self.assertTrue(r['isError']); self.assertEqual([b['request'] for b in self.requests],[1])
        self.assertNotIn('wrong',json.dumps(r)); self.assertNotIn('token-',json.dumps(r))
    def test_batch_stops_after_failure(self):
        self.fail_weight = 82
        r = payload(self.mcp.call('add_batch_weight', {'data':[{'weight':w,'measurementTime':1700000000+i} for i,w in enumerate([81,82,83])]}))
        self.assertEqual(r['accepted'],1); self.assertEqual(r['attempted'],2)
        self.assertEqual(len(self.weights),1)
    def test_roundtrip_with_composition_and_pagination(self):
        w = {'weight':80.1,'measurementTime':1700000000,'height':180,'additionalData':{'muscleMass':34.2}}
        r = payload(self.mcp.call('add_weight',{'data':w})); self.assertEqual(r['accepted'],1)
        rows = payload(self.mcp.call('download_weights_by_measurement_time',{'fromTime':1699999999,'limit':1}))
        self.assertEqual(rows['records'][0]['additionalData']['muscleMass'],34.2)
        self.assertEqual(rows['records'][0]['weight'],80.1)
        self.assertFalse(rows['has_more'])
        w['id'] = rows['records'][0]['id']; w['weight'] = 80.3
        self.assertEqual(payload(self.mcp.call('update_weight',{'data':w}))['accepted'],1)
    def test_enrichment_failure_is_visible_on_readback(self):
        self.reject_enrichment = True
        r = payload(self.mcp.call('add_weight',{'data':{'weight':80,'measurementTime':1700000000,'additionalData':{'muscleMass':34}}}))
        self.assertEqual(r['accepted'],1)
        rows = payload(self.mcp.call('download_weights_by_measurement_time',{'fromTime':0}))
        self.assertIsNone(rows['records'][0].get('additionalData'))
    def test_accounts_are_isolated(self):
        for account in ('fixture-a','fixture-b'):
            payload(self.mcp.call('download_weights_by_measurement_time',{'login':account,'password':'fixture-secret','fromTime':0}))
        reads = [b for b in self.requests if b['request']==77]
        self.assertEqual([b['access_id'] for b in reads],['token-fixture-a','token-fixture-b'])
    def test_invalid_input_never_authenticates(self):
        for args in ({'data':{'weight':80,'measurementTime':1700000000000}}, {'data':{'weight':80,'measurementTime':1700000000},'resync':True}):
            self.assertTrue(self.mcp.call('add_weight',args)['isError'])
        self.assertFalse(self.requests)
    def test_dataservice_forwarding_and_resync_false(self):
        payload(self.mcp.call('download_pressures',{'fromTime':1700000000,'limit':10,'sensorId':'fixture'}))
        r = self.mcp.call('upload_pressures',{'data':[{'timestamp':1700000000,'utc_offset':0,'sensor_id':'fixture','recordIndex':1,'systolic_pressure':120,'diastolic_pressure':80,'heart_rate':65}]})
        self.assertEqual(payload(r)['count'],1)
        writes = [b for b in self.requests if b['request']!=1]
        self.assertEqual(len(writes),2)
        self.assertEqual(writes[0]['query']['from_t'], ['1700000000'])
        self.assertEqual(writes[0]['query']['limit'], ['10'])
        for group in writes[-1]['groups']:
            self.assertFalse(group.get('resync',False))

if __name__ == '__main__': unittest.main()
