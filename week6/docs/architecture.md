# Week6 architecture and data

## Repository context

AI Advent Challenge #9 stores each week's learning work in a `weekN/` directory.
Tasks are represented by commits, not task folders or branches. Week0 starts
with a DeepSeek CLI; Week1 adds a persistent daemon and context management;
Week2 adds memory and task state; Week3 adds MCP and automation; Week4 adds RAG;
Week5 explores local models and a private VPS model. Week6 reduces that history
to a documentation assistant for this entire repository.

Earlier weeks remain separate historical implementations. A feature described
in a Week3 or Week5 report is not automatically part of Week6.

## Runtime

`advent-agent` is an interactive or one-shot CLI. It automatically starts its
sibling `advent-agentd` and communicates over a private Unix socket. The daemon
persists sessions and serializes their operations. `git-mcp` is a short-lived
stdio MCP server, started for each question and stopped after its response.
It exposes only `git_branch`, without arguments; its repository is fixed by
`--project` at launch. The server runs read-only Git commands, not arbitrary shell
strings. API keys are removed from MCP and indexer child environments.

Every question fetches fresh Git context through MCP, searches the Markdown
index, asks DeepSeek to score candidates, filters and reranks them, and generates
a grounded answer. The model is never offered tools. Documentation has up to
20 candidates, five final chunks, minimum relevance 2/3, and a 6,000-character
budget. Git evidence is added separately and is not discarded by document scoring.

The grounding validator checks JSON shape, valid citation numbers, and exact
quotes from evidence. It checks provenance, not whether every claim logically
follows from a quote; answer correctness still needs review. An unsupported
question should produce an explicit unknown response. Missing/stale indexes
fail before generation; unavailable MCP produces a visible warning while still
allowing documentation answers. No old branch snapshot substitutes for live data.

## Index and source boundaries

The Python standard-library indexer adapts Week4's embedding prompts, normalized
vectors, cached embeddings, SQLite storage, and exact cosine search. It indexes
tracked and non-ignored `.md` files throughout the repository, including results
and reports. `.git`, `.venv`, `node_modules`, `__pycache__`, and runtime `data`
directories are excluded. Symlinks outside the root are excluded. Documents retain
original text, line ranges, headings, week names, and content hashes.

`AGENTS.md` is searchable reference material. It is not loaded as an executable
system instruction. Neither retrieved documents nor conversation history can
override the assistant's capability and grounding instructions.

Embedding model identity is recorded and checked on search. Changed weights or
Markdown content require explicit reindexing. Builds atomically replace the index
and reuse cached vectors for unchanged embedding inputs. No Git history, source
code, non-Markdown schema files, or remote pages are indexed automatically.
Schema and API descriptions written in Markdown are included.

## Session schema and recovery

Each private `sessions/<id>.json` file contains `id`, canonical repository `root`,
`model`, full accepted `messages`, `summary`, `summarized` message offset, and
`operations`. Each operation records the question, timestamp, selected evidence,
Git snapshot or warning, raw answer, validated answer, provider calls/usage,
error, and discard state. A saved operation without an answer represents a
failed or interrupted request. Its question can be retried with fresh evidence.
It is not added to accepted conversation history. Retrying does not erase old
attempts; discarding marks an unsuccessful attempt explicitly.

After more than eight unsummarized messages, the daemon summarizes older messages
while retaining four recent messages. Full accepted messages remain on disk.
Failed summaries retain the prior summary and raw messages. Summaries are
conversation context, never cited evidence. Excessive remaining context produces
an explicit error instead of silently truncating it.

Writes use private temporary files and atomic replacement. A process lock prevents
two daemons owning the same state directory. Week6 does not migrate earlier
sessions or register a login service. Scheduling will be added in a later task.

## Daemon API

The private Unix socket serves `GET /health` (`week6-v1`) and `POST /stop`.
JSON endpoints are `POST /create` with `project`, `POST /get` with `id`,
`POST /ask` with `id` and `question`, and `POST /retry` or `/discard` with `id`.
Session endpoints return `{ "session": {...}, "error": "..." }`, omitting absent
fields. An error can accompany a session containing a saved failed operation.
Disconnecting a requesting client cancels its work. Requests have a configurable
five-minute timeout. This API is local; there is no network listener.
