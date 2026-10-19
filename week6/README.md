# Week6 — Developer's assistant

## Task4 — Synthetic data through the real server library

`data-mcp` is a standalone Kotlin/JVM stdio MCP server using the latest published
`com.healbe:server-api-jvm:1.6.0` (verified October 19, 2026). Its 25 tools expose
DataService downloads/uploads and weight read/add/batch/update operations.
DeepSeek in `advent-agent` generates the scenario and values; the MCP server
authenticates internally with `InMemoryAccessProvider` and calls the library.
The macOS app bundle includes Java. No RAG, registration, firmware, or weight-goal
tools are included. The read-only support bot cannot use the data tools.

See [build, connection and video instructions](data-mcp/README.md). From Week6,
run `make build-data`, `make test`, and `make build`. Private dependency access
uses existing global Gradle properties. The exact MCP artifact is
`/Users/keldzh/Develop/AiAdventChallenge/week6/data-mcp/build/package/healbe-data-mcp.app/Contents/MacOS/healbe-data-mcp`;
the harness artifacts are `week6/advent-agent` and `week6/advent-agentd`.

The plain CLI automatically detects the bundled data MCP when present.
After rebuilding, stop the old daemon with `./advent-agentd stop`; the next
request starts the new one. Inspect tools directly with `./advent-agent mcp list`
(no model or account credentials required), or ask in chat to inspect the actual
operation's connections. Set `data_mcp = ""` in a custom configuration to disable
automatic discovery. Normal CLI use loads privately configured credentials from
`data/state/healbe-credentials.json` when present; it never reads the SDK test
sources itself. This local file is configured on this workstation, Git-ignored,
mode 0600, and excluded from agent file access.

After successful tests/builds, launch the video-ready interactive session:

```sh
python3 scripts/demo_data.py --chat
```

The helper privately loads the account from the SDK tests, starts an isolated
daemon, and prints a ready-to-paste prompt. Passwords need not appear in chat or
the recording. Explicit credential arguments remain supported, but credentials
typed in chat reach the model and remain in user-message history.

The [live Task4 capture](results/task4-data-demo.json) passed: DeepSeek generated
30 daily weight/body-composition records, the server accepted all 30, and an
independent MCP download matched every timestamp, weight and muscle-mass value.
It used 5 tools, 7 provider calls, and 148,535 provider-reported tokens. This was
a real HEALBE test-account upload using the bundled runtime, not a mock capture.
The mobile-app display and video recording remain manual checks. The generated
records remain in the account.

For weight generation, the harness supplies exact UTC dates/timestamps with
`plan_daily_timestamps`. `render_weight_chart` reads selected record IDs from
MCP download evidence and appends an exact ASCII chart; the model does not draw
it. Tool failures become evidence, and Markdown quotes with only line-wrapping
differences are restored to their exact source span before strict validation.

Offline validation includes JVM input/schema tests, real JVM/SDK/stdio calls
against a local HTTP fixture, Go MCP integration, and the full Go race suite.
Uploads are real mutations: matching DataService identities may be overwritten;
`resync` is fixed false. Weight batches report partial outcomes and do not retry.
The library can accept a weighing while body-composition enrichment fails, so
the demo verifies downloaded fields rather than relying on upload receipts alone.

## Task2 — Read-only support investigation bot

`support-bot` runs locally and polls configured Mattermost channels every 15
seconds. New human root messages receive threaded analyses; follow-ups require
`@bot-username`. First launch starts with new messages; restarts catch up from
saved cursors. Each channel maps to named codebase roots. The read-only
`mattermost-mcp` stdio adapter supplies thread context. Only the service posts.

DeepSeek reads/searches code, uses Task0 Markdown RAG and Git context, and searches
human-confirmed cases. The support runner excludes write tools, rejects forged
mutation calls, and has no shell. Existing developer CLI editing is unchanged.
Responses distinguish documented expectations, code-supported explanations or
suspected bugs, and insufficient evidence. Code alone cannot establish intended
behavior. Workarounds need supporting evidence and are never claimed as tested.
Missing versions, logs, and reproduction details are requested explicitly.
Answers include source paths, line references and exact quotes. Citation
validation establishes provenance, not semantic correctness.

A configured reviewer's ✅ (`white_check_mark`) reaction confirms a specific
human or bot answer. Removing it, deleting the answer, or editing the answer or
problem invalidates confirmation; edited content needs a new reaction. Confirmed
cases use a separate SQLite embedding index scoped to channel/project mappings.
They guide a fresh investigation, not verbatim cache replay. Confirmations are
rechecked before retrieval. Missing RAG or embeddings permit live-file fallback.

Use [support.example.toml](support.example.toml) for a real server, with a genuine
bot account token in `MATTERMOST_TOKEN`, channel membership, and reviewer IDs.
DeepSeek credentials/model and local Ollama embeddings use the existing setup.
Selected thread content and code are sent to DeepSeek. Attachments are not read.
Read-only enforcement is an application capability boundary, not an OS sandbox.

After `make test` and `make build`, use the exact rebuilt artifact
`/Users/keldzh/Develop/AiAdventChallenge/week6/support-bot`:

```sh
./support-bot --config /absolute/path/support.toml
```

### Demo without installing Mattermost

The Python standard-library simulator replaces only Mattermost's REST transport.
The Go service, real stdio MCP connection, DeepSeek, source boundaries, and RAG
are unchanged. Its browser UI is labeled **simulated**; this demonstrates the
workflow, not compatibility with a particular Mattermost deployment.

After tests/build, start Ollama if needed and run:

```sh
python3 scripts/mattermost_simulator.py
```

Open <http://127.0.0.1:8765>. In another terminal, capture documentation lookup,
a code-backed workaround, insufficient evidence, confirmation and similar-case
reuse, ordinary reply suppression, and confirmation revocation:

```sh
python3 scripts/demo_support.py --simulated --output results/support-demo.json
```

For manual questions after the capture, keep the simulator running and start:

```sh
MATTERMOST_TOKEN=simulated-bot-token ./support-bot \
  --config "$PWD/data/support-simulator/support.toml"
```

The dummy token is only for the simulator. Its channel maps to Week6 and the
labeled account-directory fixture. Captures refuse overwrite and record artifact
hashes, operations, evidence, tool calls, and checks. Refresh documentation indexes
using `advent-agent --project PATH rag index`; confirmed-case indexes rebuild
automatically during retrieval. Do not run the continuous bot and scripted
capture simultaneously: a state-directory lock permits one service. Ctrl-C
stops foreground processes.

Queue/publication records survive restarts. Uncertain posts are reconciled using
bot-authored markers before retrying; this is not an exactly-once guarantee under
all network/server failures. Failed investigations retry three times, then post
an operational-failure notice rather than a diagnosis. State has no retention
policy yet; this version targets a small support channel, not production scale.

Verified on October 19, 2026: `make test` (Python, Go, and race tests) and
`make build` passed. The [verified live capture](results/task2-support-demo-verified.json)
passed five scenarios and all eight checks, using 37 provider calls and 170,443
provider-reported tokens. It exercised a real DeepSeek/Ollama/MCP investigation
against simulated Mattermost transport. Source files stayed unchanged; confirmed
case reuse and revocation, mention-only follow-ups, and ordinary reply suppression
passed. This is a small workflow demonstration, not a quality benchmark or a
real-Mattermost compatibility test.

The [first capture](results/task2-support-demo.json) passed workflow checks but
manual review found overconfident “not a bug” wording. Support-specific final
synthesis now explicitly distinguishes implementation from product intent.
The [intermediate capture](results/task2-support-demo-final.json) records a safely
withheld answer with unused quotes. Validation remains strict; repair feedback
now identifies each unused quote, and support retries can finalize saved evidence
without repeating investigation. The verified capture uses the rebuilt fixes.

## Task3 — Assistant that works with project files

Task3 is implemented before Task2. Give a goal in chat or with `-p`: DeepSeek
chooses file discovery, literal search, reading, writing, optional Markdown RAG,
and live Git branch tools. Requested edits save directly; the CLI shows recorded
unified diffs and a source-grounded answer. File work needs no Ollama or index.

The project is the CLI's working directory, or the exact directory selected with
`--project`. It is saved with the session and enforced by the daemon on every
file operation. It is **not** expanded to the Git repository root. Resuming keeps
the saved boundary. Ordinary directories without Git also work.

Reads and writes stay inside that directory. Internal symlinks are supported;
escaping links, `.git` metadata (including aliases), and daemon state are blocked.
The active indexer and Git helper cannot be overwritten by file tools. Writes
require a current file hash or a nonexistent destination, use atomic replacement,
and do not modify external hard-linked files. New files require an existing parent
directory. There is no shell, command execution, deletion, or automatic rollback.

Tools handle UTF-8 text up to 256 KiB per file. Discovery visits at most 20,000
entries; list/search pages contain at most 100 rows and approximately 32 KiB of
text. Reads return at most 500 lines and 32 KiB. An operation has at most 30 agent
turns, 60 tool calls, 240,000 bytes of conversation context, and the configured
timeout. Large or incomplete results are reported explicitly.

After `make test` and `make build`, use the rebuilt
`/Users/keldzh/Develop/AiAdventChallenge/week6/advent-agent`:

```sh
./advent-agent -p 'Find every use of the ProjectRoot API and explain its callers.'
./advent-agent --project /absolute/path/to/project -p 'Update the README to match the current implementation.'
```

The second command saves real edits. Invalid final answers get up to two correction
attempts against the same evidence, with no tool calls. If reporting still fails,
`/retry` retries only the final answer from saved evidence. Failures during tool
work instead reexamine current files and prior write outcomes. `/discard` abandons
the operation without undoing
saved edits. Multi-file changes are not a transaction. Diffs remain available
in the session even if the final provider call fails. Stop an older daemon before
using the new protocol. Task0 sessions remain readable but cannot gain editing
permissions; create a new scoped session.

Two scenarios use a three-file catalog fixture: find API definitions/callers and
update its stale README from source. Offline tests script provider responses;
the live capture uses DeepSeek to choose the tools itself. After testing/building:

```sh
python3 scripts/demo_files.py --output results/another-files-demo.json
```

The script uses the exact rebuilt binaries above and their siblings, an isolated
daemon, and disposable fixture copies. It records prompts, tool calls, usage,
before/after files, and diffs; checks source and boundary sentinels; and refuses
to overwrite an existing report. No Ollama service is needed for these scenarios.

Verified on October 18, 2026: `make test` (including Go race tests) and
`make build` passed. The [live Task3 capture](results/task3-demo-final.json)
passed both scenarios and all seven acceptance checks, with 9 provider calls and
16,942 provider-reported tokens. DeepSeek read all three files, saved the corrected
README, and preserved source files plus the Git/outside-project sentinels.
The [initial attempt](results/task3-demo.json) records a sandboxed daemon-startup
failure before any model call; the successful capture used an isolated daemon
with local-socket and network access. This is a small behavioral demonstration,
not a broad model-quality evaluation.

## Task1 — Human PR review inside the agent harness

The shared [review-bitbucket-pr skill](skills/review-bitbucket-pr/SKILL.md)
lets me list Bitbucket Cloud PRs, read diffs in revdiff inside agterm, annotate
them, and publish comments or approve/decline a PR without leaving the harness.
It also supports an explicitly authorized merge and local destination update.
I still perform the review and make the decisions myself while review
documentation is being established; automated PR review is a later task.

The [setup guide](skills/review-bitbucket-pr/SETUP.md) covers dependencies,
installation, and credentials. This public copy uses generic workspace and
repository fixtures. Credentials are supplied through the environment; private
review artifacts and the original recording are not included.

The skill is shared under `skills/`, rather than installed into this project's
active skills. Copy `skills/review-bitbucket-pr` into the target clone's
`.agents/skills/` directory to install it there. Comment publication, approval,
decline, and merge each retain the skill's explicit human confirmation steps.

Run its 19 offline tests from `week6/`:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s skills/review-bitbucket-pr/scripts -p 'test_bitbucket_pr.py'
```

## Task0 — Repository documentation assistant

Task0 introduced repository documentation retrieval. Task3 retains it as an
optional tool in the combined assistant, alongside live file tools and Git
context fetched through MCP. Answers show source paths,
line references, and exact supporting quotes, or explain missing evidence.

The CLI and daemon remain; generation uses DeepSeek only. Local Ollama is used
only for embeddings. Earlier weeks are preserved. Task lifecycle, explicit memory
commands, conversation branching, scheduling/pipelines, HomeBox,
local/VPS generation, and benchmark commands are absent from this version.
The daemon provides a base for a later scheduled-review task.

## Build and test

Requires Go 1.27+, Python 3.9+, and Git on macOS or Linux. Optional RAG also
requires Ollama with `embeddinggemma:300m`.
The Python indexer needs no third-party packages. From `week6/`:

```sh
make test
make build
```

Tests use temporary repositories, a real stdio MCP connection, fake embeddings,
and mock generation. They do not need DeepSeek credentials or a running Ollama.
Build produces `week6/advent-agent`, `week6/advent-agentd`, and `week6/git-mcp`;
the indexer is `week6/scripts/repo_index.py`. Binaries and state are ignored by Git.

## Setup and usage

For optional RAG, start your installed Ollama service and install the embedding model:

```sh
ollama serve
# In another terminal, if the model is not already installed:
ollama pull embeddinggemma:300m
```

Set `DEEPSEEK_API_KEY` in the environment before daemon startup, or reuse the
existing macOS Keychain entry (`advent-agent.deepseek`, account `api`). Generation
sends questions, selected file contents, tool results, and conversation context to DeepSeek.
Embedding generation stays on the configured loopback Ollama endpoint.

After tests and build pass, these commands use the freshly rebuilt
`/Users/keldzh/Develop/AiAdventChallenge/week6/advent-agent`, from `week6/`:

```sh
./advent-agent rag index
./advent-agent rag status
./advent-agent
```

Inside chat:

```text
/help
/help How are weeks and tasks organized in this repository?
Find every use of ProjectRoot in this project.
/help What format should a commit message use?
Which branch am I on?
What is this project's undocumented production SLA?
/stats
/exit
```

Bare `/help` explains commands. Ordinary text and `/help QUESTION` are equivalent.
Use `./advent-agent -p 'QUESTION'` for one question. Resume with
`./advent-agent resume SESSION_ID`. `/retry` refreshes evidence for a failed or
interrupted operation; `/discard` abandons it. `/summary` shows conversation
context. Ctrl-C cancels the current request and exits the CLI. Stop this daemon
with `./advent-agentd stop`.

Global flags precede subcommands. To explicitly include the whole challenge
repository, use `./advent-agent --project ..` and
`./advent-agent --project .. rag index`. Resuming uses the session's saved directory.

## Configuration and refresh

Defaults resolve beside the executables and use `week6/data/state` for the socket,
logs, sessions, and per-project indexes. Earlier daemons, profiles, and indexes
are not touched. Optional `--config /absolute/path/config.toml` accepts the keys
in [config.example.toml](config.example.toml). Filesystem overrides must be
absolute. Stop the daemon before changing its configuration or credentials;
use separate `state` directories for independent configurations.

Within the selected directory, tracked and non-ignored Markdown includes READMEs,
AGENTS.md, docs, reports, and results. Runtime directories and escaping symlinks
are excluded. Re-run `rag index` after Markdown changes; the assistant detects
stale indexes. Indexing may take several minutes on the first run and reuses
cached embeddings later. Git branch context is fetched when the model calls its tool and does not require
reindexing. Missing/stale indexes produce a RAG tool error; the assistant can
continue using live files. Markdown edits do not automatically rebuild indexes.
For directories without Git, discovery includes non-runtime Markdown files.

The [commit convention](docs/commit-conventions.md) is derived from all 30 existing
commit messages. [Architecture and data](docs/architecture.md) describes the API,
session schema, retrieval pipeline, and failure behavior. These documents are
part of the index themselves.

## Task0 historical verification

Validated on October 18, 2026. Python tests, Go tests, Go race tests, and
`make build` passed. The live corpus contained 29 documents and 727 chunks
before this verification section was expanded. Ollama 0.40.3 served the existing
`embeddinggemma:300m` model locally.

[The final live capture](results/task0-demo-final.json) passed all six questions:
repository structure, Week3 MCP registration/inspection, commit conventions,
current branch, an unsupported production-SLA question, and current branch again
after daemon shutdown and automatic restart. Five answers passed quote/citation
validation; the unsupported control returned unknown. The saved session also
retained history and generated a conversation summary. The capture recorded
13 provider calls and 67,716 provider-reported tokens, including scoring and
summary calls. Artifact hashes, commands, answers, and operation evidence are
retained in the JSON report.

Manual review found the five factual answers supported by their displayed
quotes. This small demonstration is not a broad answer-quality benchmark.
[The initial capture](results/task0-demo.json) preserves a rejected answer whose
citation numbers did not match its quoted sources. Evidence now carries explicit
citation numbers; validation was not relaxed. Its earlier-week question asked
for Task0 implementation history absent from the retrieved passages and correctly
returned unknown. The final demonstration instead asks a documented MCP workflow.

To capture another run after testing, rebuilding, and refreshing the index:

```sh
python3 scripts/demo.py --output results/another-demo.json
```

The script refuses to overwrite captures, stops only the existing Week6 daemon
before starting the rebuilt artifact, and checks another stop/restart during the
run. It leaves the restarted daemon available for chat. It does not start Ollama.
The final documentation refresh is indexed separately after recording results.
Citation validation establishes source provenance, not semantic correctness.
This capture predates Task3 file editing. No generated shell commands are executed.
Video recording and upload remain with the user.
