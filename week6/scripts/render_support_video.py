#!/usr/bin/env python3
"""Make a silent, explicitly labeled replay from a verified support capture.

macOS: uses existing Swift/AppKit and ffmpeg. No audio track is created.
This renders actual recorded excerpts, not a purported live screen recording.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def plain(text):
    return re.sub(r'(?m)^#{1,6}\s*', '', text).replace('**', '').replace('`', '').strip()


def excerpt(text, limit=850):
    text = plain(text)
    if len(text) <= limit:
        return text
    return text[:limit].rsplit(' ', 1)[0] + ' …'


def section(text, title):
    lines = text.splitlines()
    start = None
    for i, line in enumerate(lines):
        cleaned = plain(line).strip(':').lower()
        if cleaned == title.lower():
            start = i + 1
            break
    if start is None:
        return text
    end = len(lines)
    for i in range(start, len(lines)):
        if lines[i].startswith(('## ', '**Evidence', '**Workaround', '**Missing', '**Assessment')):
            end = i; break
    return '\n'.join(lines[start:end]).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--capture', type=Path, default=ROOT / 'results/task2-support-demo-verified.json')
    parser.add_argument('--output', type=Path, default=ROOT / 'results/task2-support-demo.mov')
    args = parser.parse_args()
    output = args.output.resolve()
    mp4 = output.with_suffix('.mp4')
    if output.exists() or mp4.exists():
        raise RuntimeError('Refusing to overwrite an existing video')
    capture = json.loads(args.capture.read_text())
    if not all(capture['checks'].values()):
        raise RuntimeError('Video requires a verified capture with all checks passing')
    jobs = {s['name']: s for s in capture['scenarios']}

    def answer(name):
        a = jobs[name]['job']['operation']['answer']
        return a['answer'] or a['clarification']

    def tools(name):
        rows = jobs[name]['job']['operation']['tools']
        # Show the actual call sequence, compacting adjacent repeats only.
        names = []
        for row in rows:
            name = row['name'] + (' → error' if row.get('error') else '')
            if not names or name != names[-1]:
                names.append(name)
        return '\n'.join(f'{n+1:02d}  {v}' for n, v in enumerate(names[:10]))

    scenes = []
    def add(title, subtitle, question, body, side, badge, narration,
            question_label='ACTUAL SUPPORT QUESTION', body_label='ACTUAL ANSWER · EXCERPT',
            side_label='OBSERVED TOOL CALLS', mono=False):
        scenes.append({'title':title, 'subtitle':subtitle, 'question':question,
                       'body':body, 'side':side, 'badge':badge, 'narration':narration,
                       'questionLabel':question_label, 'bodyLabel':body_label,
                       'sideLabel':side_label, 'mono':mono})

    add('A support assistant that investigates, not edits',
        'AI Advent Challenge #9 · A replay of the verified local demonstration',
        'When support cannot explain a problem, the bot investigates the escalation using documentation, code and previous resolutions.',
        'This video replays real recorded outputs.\n\nReal: DeepSeek, local Ollama embeddings, stdio MCP, code inspection and case retrieval.\n\nSimulated: Mattermost REST transport and reviewer actions.\n\nThe bot has no write or command-execution tools.',
        '5 live scenarios\n8 acceptance checks passed\n37 provider calls\n\nTransport can later be replaced\nwith a real Mattermost server\nusing configuration.',
        'No Mattermost installation needed',
        'This is an annotated replay of a verified support bot demonstration. The Mattermost transport is simulated. DeepSeek, local embeddings, MCP, code inspection, and retrieval are real. The assistant investigates problems without editing the code.',
        question_label='THE WORKFLOW',body_label='WHAT YOU ARE WATCHING',side_label='VERIFIED RUN')

    add('One channel, explicit read-only code access',
        'The same rebuilt Go service and MCP adapter run against either transport.',
        'A new human root message triggers triage. The service fetches the thread through MCP, then asks DeepSeek to investigate.',
        'Channel → configured project roots\n\nThread context → read-only Mattermost MCP\n\nEvidence → documentation RAG + live code + confirmed cases\n\nValidated answer → service-controlled thread reply\n\nFollow-ups run only when someone mentions the bot.',
        'Allowed\nlist_files · search_files\nread_file · rag_search\ngit_branch · case_search\nget_thread\n\nBlocked\nwrite_file · shell execution',
        'Writes rejected in tool dispatch',
        'Each channel maps to explicit local codebase paths. New questions are analyzed automatically. MCP provides thread context. The model can read files and search knowledge, but write calls are rejected. Only the service can publish the answer.',
        question_label='TRIGGER',body_label='DATA FLOW',side_label='CAPABILITIES')

    add('1. Answer a documented behavior question',
        'Expected behavior requires evidence of intent, not just an observation in code.',
        jobs['documentation']['question'],
        excerpt(section(answer('documentation'),'Assessment'),750),
        tools('documentation'), 'Documentation RAG + live files',
        'The first question asks whether file operations should work without an Ollama index. The bot retrieves documentation and checks live files. It answers that this is documented expected behavior, and cites the supporting sources.')

    add('2. Investigate the reported failure',
        'A controlled account-directory fixture makes the investigation reproducible.',
        jobs['workaround']['question'],
        'The real tool trace shows file discovery, documentation retrieval, source reads and a search for earlier cases.\n\nThe key observation: the stored email is lowercase, but the lookup passes the input through unchanged.\n\nNo source file was modified.',
        'ACCOUNTS = {\n  "alex@example.test": "Alex"\n}\n\ndef find_account(email):\n    return ACCOUNTS.get(email)',
        'Live source read · no execution',
        'Next, mixed-case email input fails to find an existing account. The bot reads the fixture. The stored key is lowercase, but the lookup performs an exact dictionary match without normalizing input. It does not modify or execute the code.',
        body_label='OBSERVED INVESTIGATION',side_label='ACTUAL FIXTURE SOURCE',mono=True)

    add('3. Explain the cause and offer a workaround',
        'Understanding the implementation does not prove that the product behavior is intended.',
        excerpt(section(answer('workaround'),'Assessment'),300),
        excerpt(section(answer('workaround'),'Workaround'),820),
        'Evidence-based suggestion\n\nUse the stored lowercase form.\n\nThe answer labels the workaround\nas suggested and untested.\n\nProduct intent remains unconfirmed.\nDeployed version may differ.',
        'No automatic bug fix',
        'The answer separates a code-supported explanation from product intent. It suggests using the stored lowercase address as a workaround and labels the suggestion untested. Whether case-insensitive lookup should be supported remains a product decision.',
        question_label='ACTUAL ASSESSMENT · EXCERPT',side_label='WHY THIS MATTERS')

    add('4. Ask for evidence instead of inventing a cause',
        'The unknown case is a required outcome, not an invitation to guess.',
        jobs['insufficient_evidence']['question'],
        excerpt(section(answer('insufficient_evidence'),'Missing information'),850),
        'Outcome\nInsufficient evidence\n\nNo supported workaround found.\nNo reproduction claimed.\n\nRequest the app version, context,\nreproduction details and logs.',
        'Uncertainty is explicit',
        'For an unexplained purple screen, there is not enough information to identify a cause. The bot says so and requests details such as the app version, platform, screenshots, reproduction steps, and logs. It does not invent a diagnosis.',
        body_label='ACTUAL REQUEST FOR INFORMATION · EXCERPT',side_label='ASSESSMENT')

    add('5. Turn a reviewed answer into reusable knowledge',
        'The demonstration applies a scripted reviewer confirmation to the actual answer.',
        'A configured reviewer adds a white-check-mark reaction to the workaround reply.',
        'The service records the original problem, the confirmed resolution, its reviewer and thread provenance.\n\nOnly confirmed cases enter the reusable case corpus.\n\nThe case index uses the existing local embedding and SQLite retrieval pipeline.\n\nRemoving the reaction revokes the confirmation.',
        'Verified in the capture\n\nHuman confirmation saved: PASS\n\nCase retrieval: PASS\n\nConfirmation revocation: PASS',
        'Human-controlled support memory',
        'A reviewer confirmation makes the resolution eligible for future retrieval. In this demonstration, that reviewer action is scripted. The case index stores the problem, answer, and provenance. Removing the reaction revokes the confirmation.',
        question_label='REVIEWER ACTION',body_label='OBSERVED CASE WORKFLOW',side_label='RECORDED CHECKS')

    add('6. Reuse a similar case—and check the code again',
        'A previous resolution guides the investigation; it is not replayed blindly.',
        jobs['similar_confirmed_case']['question'],
        excerpt(section(answer('similar_confirmed_case'),'Assessment'),850),
        tools('similar_confirmed_case'), 'Confirmed case + current source',
        'A similar question now uses an uppercase email address. The bot retrieves the confirmed case and rereads the current source. Its answer says the workaround still applies, while keeping product intent unconfirmed. This is retrieval with revalidation, not a blind cache hit.')

    add('7. Continue only when the bot is mentioned',
        'Ordinary discussion stays quiet; a mention requests another analysis in the same thread.',
        jobs['mentioned_followup']['question'],
        excerpt(answer('mentioned_followup'),850),
        'Recorded checks\n\nOrdinary reply ignored: PASS\n\nMentioned follow-up in the\nsame thread: PASS\n\nSource unchanged: PASS',
        'Thread context is preserved',
        'Normal conversation does not trigger another answer. An explicit mention asks whether the workaround needs a code change. The bot uses the existing thread and answers there. The recorded checks verify both behaviors.')

    add('The complete support workflow is verified',
        'A small end-to-end demonstration—not a broad model-quality or real-server compatibility benchmark.',
        'Automatic triage → evidence-based analysis → practical workaround or targeted questions → human-confirmed reuse.',
        'All eight checks passed.\n\nTests: Python, Go and Go race tests passed.\nBuild: rebuilt support-bot and mattermost-mcp.\n\nSource capture:\nresults/task2-support-demo-verified.json\n\nInteractive demo:\nhttp://127.0.0.1:8765',
        'PASS  Human confirmation\nPASS  Confirmed-case retrieval\nPASS  Current-code recheck\nPASS  Ordinary reply ignored\nPASS  Mentioned follow-up\nPASS  Confirmation revoked\nPASS  Source unchanged\nPASS  No write tool executed',
        'Ready to demonstrate to a tutor',
        'All eight workflow checks passed, along with the automated tests and rebuild. The full questions, answers, evidence, and tool calls remain in the JSON capture. The browser simulator is available for an interactive demonstration. Thank you for watching.',
        question_label='RESULT',body_label='REPRODUCIBLE ARTIFACTS',side_label='ACCEPTANCE CHECKS')

    work = ROOT / 'data/support-video'
    work.mkdir(parents=True, exist_ok=True)
    scenes_file = work / 'scenes.json'
    scenes_file.write_text(json.dumps(scenes, indent=2))
    subprocess.run(['swift','-module-cache-path',str(work / 'swift-cache'),
                    str(ROOT / 'scripts/support_video.swift'),str(scenes_file),str(work)],check=True)
    segments=[]
    for n, scene in enumerate(scenes):
        duration = [10, 14, 18, 18, 24, 24, 18, 22, 18, 12][n]
        segment = work / f'scene-{n:02d}.mov'
        subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-y','-loop','1','-framerate','12',
                        '-i',str(work / f'scene-{n:02d}.png'),
                        '-t',str(duration),'-vf','format=yuv420p','-c:v','libx264','-preset','fast',
                        '-crf','20','-an','-movflags','+faststart',str(segment)],check=True)
        segments.append(segment)
        print(f'Rendered scene {n+1}/{len(scenes)}',flush=True)
    concat = work / 'concat.txt'
    concat.write_text(''.join("file '" + str(p).replace("'", "'\\''") + "'\n" for p in segments))
    subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-n','-f','concat','-safe','0','-i',str(concat),
                    '-c','copy','-movflags','+faststart',str(output)],check=True)
    subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-n','-i',str(output),
                    '-c','copy','-movflags','+faststart',str(mp4)],check=True)
    print(f'Created {output}\nCreated {mp4}')


if __name__ == '__main__':
    main()
