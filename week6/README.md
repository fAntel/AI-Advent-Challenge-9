# Week6 — Developer's assistant

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

Task0 turns `advent-agent` into a documentation assistant for the entire AI Advent
Challenge repository. Plain questions and `/help QUESTION` use repository RAG
plus the current Git branch fetched through MCP. Answers show source paths,
line references, and exact supporting quotes, or explain missing evidence.

The CLI and daemon remain; generation uses DeepSeek only. Local Ollama is used
only for embeddings. Earlier weeks are preserved. Task lifecycle, explicit memory
commands, conversation branching, model tools, scheduling/pipelines, HomeBox,
local/VPS generation, and benchmark commands are absent from this version.
The daemon provides a base for a later scheduled-review task.

## Build and test

Requires Go 1.27+, Python 3.9+, Git, and Ollama with `embeddinggemma:300m`.
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

Start your installed Ollama service and install the embedding model if needed:

```sh
ollama serve
# In another terminal, if the model is not already installed:
ollama pull embeddinggemma:300m
```

Set `DEEPSEEK_API_KEY` in the environment before daemon startup, or reuse the
existing macOS Keychain entry (`advent-agent.deepseek`, account `api`). Generation
sends questions, selected documentation, and conversation context to DeepSeek.
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
/help How can I register and inspect MCP servers in Week3?
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

The repository root is resolved from the working directory, so every week's
files belong to the same project. Override it with `--project PATH`. Global flags
precede subcommands: `./advent-agent --project .. rag index`. Resuming uses the
session's saved project root.

## Configuration and refresh

Defaults resolve beside the executables and use `week6/data/state` for the socket,
logs, sessions, and per-project indexes. Earlier daemons, profiles, and indexes
are not touched. Optional `--config /absolute/path/config.toml` accepts the keys
in [config.example.toml](config.example.toml). Filesystem overrides must be
absolute. Stop the daemon before changing its configuration or credentials;
use separate `state` directories for independent configurations.

All tracked and non-ignored Markdown is included across all weeks: READMEs,
AGENTS.md, docs, reports, and results. Runtime directories and escaping symlinks
are excluded. Re-run `rag index` after Markdown changes; the assistant detects
stale indexes. Indexing may take several minutes on the first run and reuses
cached embeddings later. Git branch context is fetched anew on every question
and does not require reindexing. A missing/stale index is an error, even for a
branch question; an empty document match still permits a live branch answer.

The [commit convention](docs/commit-conventions.md) is derived from all 30 existing
commit messages. [Architecture and data](docs/architecture.md) describes the API,
session schema, retrieval pipeline, and failure behavior. These documents are
part of the index themselves.

## Verification

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
No generated commands are executed and no repository changes can be made through
the assistant. Video recording and upload remain with the user.
