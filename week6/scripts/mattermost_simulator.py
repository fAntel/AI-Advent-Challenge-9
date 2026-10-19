#!/usr/bin/env python3
"""Local Mattermost REST transport simulator, explicitly not a Mattermost server.

The real support-bot and stdio MCP adapter connect without special code paths.
The browser UI represents a reviewer and can send questions and confirm answers.
Only the small API subset needed by this demo is implemented.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

from support_demo_common import ROOT, private_json

CHANNEL = 'aaaaaaaaaaaaaaaaaaaaaaaaaa'
REVIEWER = 'cccccccccccccccccccccccccc'
BOT = 'dddddddddddddddddddddddddd'
USERS = {REVIEWER: {'id': REVIEWER, 'username': 'reviewer', 'is_bot': False},
         BOT: {'id': BOT, 'username': 'week6-support', 'is_bot': True}}
TOKENS = {'simulated-reviewer-token': REVIEWER, 'simulated-bot-token': BOT}

PAGE = '''<!doctype html><html><head><meta charset="utf-8"><title>Support bot demo</title>
<style>
body{font:16px system-ui;background:#f3f5f8;color:#17253c;margin:0}header{background:#17253c;color:white;padding:22px 6vw}header h1{margin:0;font-size:24px}.badge{color:#fbd38d;font-size:14px}main{max-width:1000px;margin:24px auto;padding:0 18px}.card{background:white;border:1px solid #dce2eb;border-radius:12px;padding:20px;margin:18px 0}.reply{margin:16px 0 0 28px;border-left:3px solid #577be8;padding:14px;background:#f7f9ff}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:14px/1.6 system-ui;margin:12px 0}small{color:#607087}textarea{box-sizing:border-box;width:100%;height:90px;border:1px solid #b9c5d5;border-radius:7px;padding:10px;font:inherit}button{background:#3659ba;border:0;border-radius:7px;padding:9px 15px;color:white;cursor:pointer;margin:8px 8px 0 0}button.secondary{background:#e9edf6;color:#243657}.confirmed{color:#166534;font-weight:600}#status{color:#607087;font-size:14px}.empty{color:#607087;padding:30px}
</style></head><body><header><h1>Support investigation assistant</h1><div class="badge">SIMULATED Mattermost transport · Real bot + MCP + DeepSeek + RAG</div></header><main>
<div class="card"><b>New support escalation</b><p id="status">The bot polls every 15 seconds. Analysis can take a minute.</p><textarea id="question" placeholder="Describe the problem..."></textarea><button onclick="send()">Send escalation</button><button class="secondary" onclick="example()">Use account-lookup example</button></div><div id="threads"></div></main>
<script>
const channel='aaaaaaaaaaaaaaaaaaaaaaaaaa',reviewer='cccccccccccccccccccccccccc';
async function api(path,method='GET',body){let r=await fetch('/api/v4'+path,{method,headers:{'Content-Type':'application/json','Authorization':'Bearer simulated-reviewer-token'},body:body?JSON.stringify(body):undefined});if(!r.ok)throw Error('HTTP '+r.status);return r.json()}
function example(){document.getElementById('question').value='In the catalog demo, Alex is registered as alex@example.test. Looking up Alex@Example.Test returns no account. Explain the code and suggest a workaround without changing files.'}
async function send(root,message){const q=document.getElementById('question');message=message||q.value;if(!message.trim())return;await api('/posts','POST',{channel_id:channel,root_id:root||'',message});if(!root)q.value='';await refresh()}
async function confirm(post,on){if(on)await api('/reactions','POST',{user_id:reviewer,post_id:post,emoji_name:'white_check_mark'});else await api('/users/'+reviewer+'/posts/'+post+'/reactions/white_check_mark','DELETE');await refresh()}
function node(tag,text,cls){let n=document.createElement(tag);if(text)n.textContent=text;if(cls)n.className=cls;return n}
async function refresh(){try{let list=await api('/channels/'+channel+'/posts?per_page=200&page=0');let posts=Object.values(list.posts).sort((a,b)=>a.create_at-b.create_at);let host=document.getElementById('threads');host.replaceChildren();for(let root of posts.filter(p=>!p.root_id)){let card=node('div','', 'card');card.append(node('small','Reviewer · '+new Date(root.create_at).toLocaleTimeString()),node('pre',root.message));for(let p of posts.filter(p=>p.root_id===root.id)){let reply=node('div','', 'reply');reply.append(node('small',(p.user_id===reviewer?'Reviewer':'Support assistant')+' · '+new Date(p.create_at).toLocaleTimeString()),node('pre',p.message));let reactions=await api('/posts/'+p.id+'/reactions');let checked=reactions.some(r=>r.user_id===reviewer&&r.emoji_name==='white_check_mark');let b=node('button',checked?'✓ Confirmed — revoke':'Confirm resolution',checked?'secondary confirmed':'secondary');b.onclick=()=>confirm(p.id,!checked);reply.append(b);card.append(reply)}let b=node('button','Ask a follow-up','secondary');b.onclick=()=>{let text=prompt('Follow-up for the bot');if(text)send(root.id,'@week6-support '+text)};card.append(b);host.append(card)}if(!posts.length)host.append(node('div','No escalations yet. Send one above or run the scripted demo.','empty'))}catch(e){document.getElementById('status').textContent='Simulator connection: '+e.message}}
refresh();setInterval(refresh,4000);
</script></body></html>'''


class Simulator:
    def __init__(self, state):
        self.file = state / 'simulator.json'
        self.lock = threading.RLock()
        self.data = json.loads(self.file.read_text()) if self.file.exists() else {'posts': {}, 'reactions': []}

    def save(self):
        private_json(self.file, self.data)


def handler(sim):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def reply(self, value, status=200):
            raw = json.dumps(value).encode()
            self.send_response(status)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_GET(self):
            self.dispatch()

        def do_POST(self):
            self.dispatch()

        def do_DELETE(self):
            self.dispatch()

        def dispatch(self):
            parsed = urlsplit(self.path)
            if parsed.path == '/' and self.command == 'GET':
                raw = PAGE.encode(); self.send_response(200)
                self.send_header('Content-Type', 'text/html; charset=utf-8')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers(); self.wfile.write(raw); return
            token = self.headers.get('Authorization', '').removeprefix('Bearer ')
            user = TOKENS.get(token)
            if not user:
                return self.reply({'error': 'unauthorized'}, 401)
            try:
                length = int(self.headers.get('Content-Length', 0))
                if length > 200000:
                    return self.reply({'error': 'body too large'}, 413)
                body = json.loads(self.rfile.read(length)) if length else {}
                with sim.lock:
                    self.route(parsed, body, user)
            except (ValueError, KeyError, TypeError):
                self.reply({'error': 'invalid request'}, 400)

        def route(self, parsed, body, user):
            path = parsed.path.removeprefix('/api/v4')
            parts = path.strip('/').split('/')
            posts = sim.data['posts']
            method = self.command
            if method == 'GET' and path == '/system/ping':
                return self.reply({'status': 'OK'})
            if method == 'GET' and len(parts) == 2 and parts[0] == 'users':
                return self.reply(USERS[user if parts[1] == 'me' else parts[1]])
            if method == 'GET' and path == '/channels/' + CHANNEL + '/posts':
                query = parse_qs(parsed.query)
                page = max(0, int(query.get('page', ['0'])[0]))
                count = min(200, max(1, int(query.get('per_page', ['60'])[0])))
                rows = sorted(posts.values(), key=lambda p: (p['create_at'], p['id']), reverse=True)
                rows = rows[page*count:(page+1)*count]
                return self.reply({'order': [p['id'] for p in rows], 'posts': {p['id']: p for p in rows}})
            if method == 'GET' and parts[0] == 'posts' and len(parts) >= 2:
                post = posts.get(parts[1])
                if not post:
                    return self.reply({'error': 'not found'}, 404)
                if len(parts) == 2:
                    return self.reply(post)
                if parts[2] == 'thread':
                    rows = [p for p in posts.values() if p['id'] == post['id'] or p['root_id'] == post['id']]
                    return self.reply({'order': [p['id'] for p in rows], 'posts': {p['id']: p for p in rows}})
                if parts[2] == 'reactions':
                    return self.reply([r for r in sim.data['reactions'] if r['post_id'] == post['id']])
            if method == 'POST' and path == '/posts':
                if body['channel_id'] != CHANNEL or not body.get('message', '').strip():
                    return self.reply({'error': 'invalid post'}, 400)
                root = body.get('root_id', '')
                if root and (root not in posts or posts[root]['root_id']):
                    return self.reply({'error': 'invalid root'}, 400)
                now = time.time_ns() // 1000000
                post = {'id': secrets.token_hex(13), 'channel_id': CHANNEL, 'root_id': root,
                        'user_id': user, 'message': body['message'], 'type': '',
                        'create_at': now, 'update_at': now, 'delete_at': 0, 'props': body.get('props', {})}
                posts[post['id']] = post; sim.save()
                return self.reply(post, 201)
            if method == 'POST' and path == '/reactions':
                if body['user_id'] != user or body['post_id'] not in posts:
                    return self.reply({'error': 'invalid reaction'}, 400)
                reaction = dict(body, create_at=time.time_ns() // 1000000)
                sim.data['reactions'] = [r for r in sim.data['reactions'] if not (
                    r['user_id'] == user and r['post_id'] == body['post_id'] and r['emoji_name'] == body['emoji_name'])]
                sim.data['reactions'].append(reaction); sim.save()
                return self.reply(reaction, 201)
            if method == 'DELETE' and len(parts) == 6 and parts[0] == 'users' and parts[2] == 'posts' and parts[4] == 'reactions':
                if parts[1] != user:
                    return self.reply({'error': 'forbidden'}, 403)
                sim.data['reactions'] = [r for r in sim.data['reactions'] if not (
                    r['user_id'] == user and r['post_id'] == parts[3] and r['emoji_name'] == parts[5])]
                sim.save(); return self.reply({'status': 'OK'})
            return self.reply({'error': 'unsupported simulator endpoint'}, 404)
    return Handler


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=8765)
    parser.add_argument('--directory', type=Path, default=ROOT / 'data/support-simulator')
    args = parser.parse_args()
    os.umask(0o077)
    directory = args.directory.resolve(); state = directory / 'state'
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    url = f'http://127.0.0.1:{args.port}'
    private_json(state / 'credentials.json', {'url': url, 'transport': 'simulated',
                 'admin_id': REVIEWER, 'bot_id': BOT, 'channel_id': CHANNEL,
                 'admin_token': 'simulated-reviewer-token', 'bot_token': 'simulated-bot-token'})
    config = directory / 'support.toml'; q = json.dumps
    config.write_text(f'''url = {q(url)}
state = {q(str(state))}
poll_seconds = 15
reviewers = [{q(REVIEWER)}]
[[channels]]
id = {q(CHANNEL)}
[channels.projects]
week6 = {q(str(ROOT))}
catalog = {q(str(ROOT / 'testdata/support/catalog'))}
[agent]
state = {q(str(ROOT / 'data/state'))}
''')
    config.chmod(0o600)
    server = ThreadingHTTPServer(('127.0.0.1', args.port), handler(Simulator(state)))
    print(f'Mattermost transport SIMULATOR: {url}\nConfig: {config}', flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == '__main__':
    main()
