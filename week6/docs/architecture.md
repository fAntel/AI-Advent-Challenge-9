# Week6 architecture and data

## Repository context

AI Advent Challenge #9 stores each week's learning work in a `weekN/` directory.
Tasks are represented by commits, not task folders or branches. Week0 starts
with a DeepSeek CLI; Week1 adds a persistent daemon and context management;
Week2 adds memory and task state; Week3 adds MCP and automation; Week4 adds RAG;
Week5 explores local models and a private VPS model. Week6 starts with a
documentation assistant and adds a project-scoped file assistant in Task3, before the planned Task2 work.

Earlier weeks remain separate historical implementations. A feature described
in a Week3 or Week5 report is not automatically part of Week6.

## Runtime

`advent-agent` is an interactive or one-shot CLI. It automatically starts its
sibling `advent-agentd` and communicates over a private Unix socket. The daemon
persists sessions and serializes their operations. `git-mcp` is a short-lived
stdio MCP server, started when its tool is called and stopped after its response.
It exposes only `git_branch`, without arguments; its repository is fixed by
`--project` at launch. The server runs read-only Git commands, not arbitrary shell
strings. API keys are removed from MCP and indexer child environments.

DeepSeek selects tools in a native function-calling loop. Built-in tools expose
`list_files`, `search_files`, `read_file`, `write_file`, and `rag_search`;
`git_branch` uses the existing MCP server. Calls execute sequentially with strict
argument decoding. File reads and searches produce numbered evidence; successful
writes produce receipts. Search/list evidence stores plain text; pagination and
other tool metadata stay outside that text so JSON escaping does not alter the
verbatim quotes checked by the validator. Final synthesis uses the existing JSON answer shape and
quote/citation validator. This validates provenance, not semantic correctness.

The provider sends assistant tool-call messages and corresponding tool responses
with their IDs. Truncated responses, invalid call IDs/types, and duplicate IDs fail
before executing the batch. Every provider call contributes to usage statistics.
Each operation permits 30 agent turns and 60 tool calls, with a 240,000-byte
conversation budget and the configured request timeout. Relevance scoring and
final synthesis remain separate provider calls. Final synthesis allows two additional
correction calls for invalid JSON, quotes, or citations. Every attempt and its
validation problems are persisted and reported together; quote and citation
requirements remain unchanged. Corrections
use the same evidence and cannot call file tools.

## Filesystem and source boundaries

The client sends an absolute selected directory at session creation. The daemon
canonicalizes it, saves it as `root`, and never widens it to the repository root.
Tool arguments cannot supply a different root. Resume retains the saved boundary.
Directories without Git support file tools and indexing; Git calls return an error.

A shared Go filesystem layer opens each path component using directory descriptors
and `O_NOFOLLOW`. It resolves internal symlinks explicitly, rejects escaping or
cyclic links, and checks resolved components against `.git` and daemon state
restrictions. It also blocks writes to active runtime helpers. Reads accept only
regular UTF-8 text files up to 256 KiB. Directory discovery skips `.git`, `.venv`,
`node_modules`, `__pycache__`, and `data`; explicitly named ordinary files in the
other skipped directories remain accessible. State and Git metadata remain blocked.

Writes require an expected content hash or create-only semantics. A new inode is
written and synced in the resolved parent directory, then published atomically;
create uses no-clobber publication. Existing permission bits are preserved. Existing
files are rechecked immediately before replacement. This avoids overwriting detected
concurrent edits and prevents external hard links from receiving the change. It is
not a filesystem-wide transaction or an OS sandbox against another process moving
already-open directories or modifying files at the exact replacement instant.
The agent has no shell, delete, link-creation, or permission-management tool.
New file parent directories must already exist.

The daemon builds a bounded Markdown corpus through the same filesystem layer and
passes it over stdin to the Python indexer. Git discovery respects tracked and
non-ignored files within the selected directory; ordinary directories use filesystem
discovery. Python runs in isolated mode. The indexer receives a snapshot rather than
opening project files, and the daemon compares the corpus again after it finishes.
Retrieved chunks are checked against current file contents before model scoring.

RAG retains cached embeddings, SQLite storage, and cosine search: 20 candidates,
five selected chunks, minimum relevance 2/3, and a 6,000-character budget. The
corpus format is versioned; old indexes require rebuilding. Missing/stale indexes
are tool errors, permitting live-file fallback. Markdown writes invalidate the
index naturally; rebuilding remains an explicit CLI action.

File contents, `AGENTS.md`, retrieved documents, and history are reference data,
never instructions capable of overriding the user's goal or tool restrictions.

## Session schema and recovery

Each private `sessions/<id>.json` file contains `version`, `id`, canonical project `root`,
`model`, full accepted `messages`, `summary`, `summarized` message offset, and
`operations`. Each operation records the question, timestamp, selected evidence,
Git snapshot or warning, raw answer, validated answer, provider calls/usage,
error, discard state, tool calls/results, and file changes with before/after
hashes, unified diffs, and intent/saved/failed status. A saved operation without an
answer represents a failed or interrupted request. Its question can be retried
with fresh evidence and prior change outcomes if tool work failed. Final-answer
failures instead persist `finalization_pending`, the draft, and `answer_attempts`.
Their retry synthesizes only an answer from saved evidence, explicitly labeled
as an earlier snapshot, without rerunning tools or repeating saved writes. Earlier
quote/citation failures are recognized from their saved raw answer and error.
An intent is persisted before publishing a write; results are checkpointed after
each tool. An interrupted intent is ambiguous, so retry reads current files rather
than replaying writes. Saved edits remain after failure or discard. The CLI displays
recorded diffs even when later generation fails.
Failed operations are not added to accepted history. Retrying does not erase old
attempts; discarding marks an unsuccessful attempt explicitly.

After more than eight unsummarized messages, the daemon summarizes older messages
while retaining four recent messages. Full accepted messages remain on disk.
Failed summaries retain the prior summary and raw messages. Summaries are
conversation context, never cited evidence. Excessive remaining context produces
an explicit error instead of silently truncating it.

Writes use private temporary files and atomic replacement. A process lock prevents
two daemons owning the same state directory. Version-1 sessions remain readable
but are refused for new operations: start a new explicitly scoped session. Week6
does not register a login service. Scheduling will be added in a later task.

## Daemon API

The private Unix socket serves `GET /health` (`week6-v2`) and `POST /stop`.
JSON endpoints are `POST /create` with an absolute `project`, `POST /get` with `id`,
`POST /ask` with `id` and `question`, and `POST /retry` or `/discard` with `id`.
Session endpoints return `{ "session": {...}, "error": "..." }`, omitting absent
fields. An error can accompany a session containing a saved failed operation.
Disconnecting a requesting client cancels its work. Requests have a configurable
five-minute timeout. This API is local; there is no network listener.
