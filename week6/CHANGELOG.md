# Changelog

All notable changes to the Week6 developer's assistant since **Week6 Task0**
(repository documentation assistant).

Task0 is the baseline: it is not listed as a change below. Entries are grouped by
task and describe the resulting behavior, not individual commits. The repository
records tasks as commits rather than folders or branches, so this file is a
narrative summary of the work done after Task0.

## Task3 — Assistant that works with project files

Implemented before Task2.

### Added

- Goal-driven file tools: `list_files`, `search_files`, `read_file`, and
  `write_file`, selected by DeepSeek in a native function-calling loop.
- Live Git branch context through a short-lived stdio MCP server (`git-mcp`)
  exposing only `git_branch`, with the repository fixed by `--project` at launch.
- Project-scoped sessions: the CLI's working directory or the exact directory
  given with `--project` is saved with the session and enforced by the daemon on
  every file operation. It is not expanded to the Git repository root, and
  resuming keeps the saved boundary.
- Support for ordinary directories without Git; Git calls return an error there.
- A shared Go filesystem layer that opens each path component with directory
  descriptors and `O_NOFOLLOW`, resolves internal symlinks explicitly, and
  rejects escaping or cyclic links.
- Hash-checked, atomic writes: writes require a current file hash or a
  nonexistent destination, use a new inode plus atomic publication, preserve
  existing permission bits, and do not modify external hard-linked files.
- Recorded unified diffs and write receipts in the CLI and session, including
  intent/saved/failed status per change.
- `/retry` to reexamine current files and prior write outcomes after failure, and
  `/discard` to abandon an operation without undoing saved edits.
- `scripts/demo_files.py` and a three-file catalog fixture for two live
  scenarios: finding API definitions/callers and updating a stale README from
  source.

### Changed

- Task0 documentation retrieval is retained as an optional `rag_search` tool in
  the combined assistant, alongside live file tools and Git context.
- File work no longer requires Ollama or a RAG index; missing or stale indexes
  are tool errors that permit live-file fallback.
- Search and list evidence stores plain text, with pagination and other tool
  metadata kept outside that text so JSON escaping does not alter the verbatim
  quotes checked by the validator.
- The daemon API is now `week6-v2` and adds `POST /retry` and `POST /discard`.
- Session schema records tool calls/results and file changes with before/after
  hashes, unified diffs, and change status.
- Version-1 (Task0) sessions remain readable but are refused for new operations;
  a new explicitly scoped session is required.

### Limits

- No shell, command execution, deletion, link creation, or permission
  management; no automatic rollback, and multi-file changes are not a
  transaction.
- Reads accept only regular UTF-8 text files up to 256 KiB; reads return at most
  500 lines and 32 KiB, and list/search pages contain at most 100 rows and
  approximately 32 KiB of text.
- Discovery visits at most 20,000 entries; an operation has at most 30 agent
  turns, 60 tool calls, 240,000 bytes of conversation context, and the
  configured timeout.
- New files require an existing parent directory.
- Not a filesystem-wide transaction or an OS sandbox against another process
  moving already-open directories or modifying files at the exact replacement
  instant.

### Validation

- Verified on October 18, 2026: `make test` (including Go race tests) and
  `make build` passed.
- The live capture `results/task3-demo-final.json` passed both scenarios and all
  seven acceptance checks, with 9 provider calls and 16,942 provider-reported
  tokens. DeepSeek read all three files, saved the corrected README, and
  preserved source files plus the Git/outside-project sentinels.
- The initial attempt `results/task3-demo.json` records a sandboxed
  daemon-startup failure before any model call; the successful capture used an
  isolated daemon with local-socket and network access.
- This is a small behavioral demonstration, not a broad model-quality
  evaluation.

## Task1 — Human PR review inside the agent harness

### Added

- A shared `review-bitbucket-pr` skill under `skills/` for reviewing Bitbucket
  Cloud pull requests in revdiff inside agterm: list PRs, read diffs, annotate
  them, and publish inline or global comments, approve, decline, or merge a PR
  without leaving the harness.
- An explicitly authorized merge path that also updates the local destination
  branch.
- `scripts/bitbucket_pr.py` helper for Bitbucket Cloud API operations, deriving
  the workspace and repository slug from the selected Git remote, with
  `--remote` when `origin` is not the correct repository.
- `SETUP.md` covering dependencies, installation, credentials, and
  troubleshooting, plus `agents/openai.yaml` for the skill.
- 19 offline tests for the helper.

### Notes

- The skill is shared under `skills/` rather than installed into this project's
  active skills; copy `skills/review-bitbucket-pr` into the target clone's
  `.agents/skills/` directory to install it there.
- The review and its decisions remain human; automated PR review is a later
  task.
- Credentials are supplied through the environment (`BITBUCKET_API_TOKEN`) and
  are never printed, persisted, passed on the command line, or placed in an
  agterm session context.
- Comment publication, approval, decline, and merge each retain explicit human
  confirmation steps.
- This public copy uses generic workspace and repository fixtures; private
  review artifacts and the original recording are not included.
- The workflow targets `bitbucket.org` only, not Bitbucket Data Center.

## Documentation

### Added

- `docs/architecture.md` describing the runtime, filesystem and source
  boundaries, session schema and recovery, and the daemon API.
- `docs/commit-conventions.md` recording the observed commit message practice
  derived from all 30 commits reachable through local Git refs before Week6
  Task0.
- `config.example.toml` documenting the accepted configuration keys.
- `results/task0-demo-final.json` and `results/task3-demo-final.json` live
  captures, alongside the earlier `results/task0-demo.json` and
  `results/task3-demo.json` attempts.

### Changed

- The README now documents Task3, Task1, and Task0 in that order, plus build,
  setup, configuration, and refresh instructions.
- The Task0 historical verification section records the final live capture: all
  six questions passed, five answers passed quote/citation validation, and the
  unsupported control returned unknown, with 13 provider calls and 67,716
  provider-reported tokens.
- The initial Task0 capture is preserved as a rejected answer whose citation
  numbers did not match its quoted sources; evidence now carries explicit
  citation numbers and validation was not relaxed.

## Build and test

### Changed

- `make test` now runs the Python indexer tests, the skill's offline tests, Go
  tests, and Go race tests.
- `make build` builds `advent-agent`, `advent-agentd`, and `git-mcp`, then
  smoke-checks each binary's `--help` and the indexer's `--help`.
- Binaries and runtime state (`/advent-agent`, `/advent-agentd`, `/git-mcp`,
  `/data/`, `/.venv/`, `__pycache__/`, `*.pyc`) are ignored by Git.

## Not yet implemented

- Task2 work is planned but not implemented.
- Task lifecycle, explicit memory commands, conversation branching,
  scheduling/pipelines, HomeBox, local/VPS generation, and benchmark commands
  are absent from this version. The daemon provides a base for a later
  scheduled-review task.
- Week6 does not register a login service.
