# advent-agent — Week 2 Task 2

`advent-agent` is a persistent, provider-neutral chat harness with explicit,
profile-scoped memory. The shipped provider adapter is DeepSeek. The CLI talks
to `advent-agentd` over a private Unix socket; the daemon owns requests,
sessions, compression, branches, accounting, and persistence.

Every user task is controlled by a persisted daemon-owned lifecycle:

```text
planning → execution → validation → done
```

Each phase makes an answer request and then pauses. `/continue` advances one
phase. A normal message while paused is feedback and reruns the current phase;
it cannot skip or change the lifecycle state. `/back` reruns the immediately
preceding phase, while `/back planning|execution|validation` returns to a named
earlier phase. After `done`, the next normal message starts a new task in the
same session.

The provider has no shell, filesystem, network, or other tool executor.
Execution therefore returns the best code, prose, patch, or instructions it can
produce in text. If a provider emits raw tool-call syntax such as DSML anyway,
the daemon rejects that phase as failed instead of treating it as completed;
use `/retry` for a text-only result or `/discard` to restore the prior phase.

## Build and test

From `week2/`:

```sh
make test
make build
```

The exact runnable artifacts are `week2/advent-agent` and
`week2/advent-agentd`. `make build` builds both command packages and verifies
their help commands, so demonstrations use newly rebuilt binaries.

Export `DEEPSEEK_API_KEY` before starting a client or daemon. The client starts
the daemon automatically by default. Stop it with `./advent-agentd stop`.

## Profiles, configuration, and instructions

A profile is a behavioral hat, for example `coding` or `writer`. Sessions and
all three memory layers are isolated by profile. Configuration is merged in
this order: built-in defaults, user `config.toml`, profile `config.toml`, then
CLI/session flags.

```text
$XDG_CONFIG_HOME/advent-agent/
├── config.toml
├── AGENTS.md
└── profiles/
    ├── coding/
    │   ├── config.toml
    │   └── AGENTS.md
    └── writer/
        ├── config.toml
        └── AGENTS.md
```

TOML configures the harness: provider, model, temperature, reasoning, paths,
timeouts, and context strategy. `AGENTS.md` configures behavior. At session
creation, instructions are resolved user-wide → profile → repository →
`--system-prompt`/`--system-prompt-file`, then their content and source paths
are stored in the session. Editing a source file does not alter old sessions.

```sh
./advent-agent profiles
./advent-agent --profile coding
./advent-agent --profile writer
./advent-agent --profile writer resume
```

Session listing, resume, deletion, and branches are restricted to the selected
profile. A session cannot change profile.

The resume picker reports task lifecycle state rather than the last request
state: for example, `planning/paused`, `execution/failed`, or `done/terminal`.
`empty` means the session was created but no task was submitted.
`legacy/completed` identifies a conversation saved before task lifecycle state
was introduced; its final provider request completed, but it has no task phase.

## Three memory scopes

| Layer | Scope | Persistence |
| --- | --- | --- |
| Short-term | profile + session | rolling summary and ordered dialog in session JSON |
| Working | profile + canonical project | `working.ini` |
| Long-term | profile | `long-term.ini` |

The project ID is a SHA-256-derived ID of the canonical Git root, or canonical
current directory outside Git. XDG defaults are:

```text
~/.config/advent-agent/                 configuration
~/.local/state/advent-agent/
└── profiles/<profile>/
    ├── long-term.ini
    ├── projects/<project-id>/
    │   ├── project.json
    │   └── working.ini
    └── sessions/<session-id>/session.json
$XDG_RUNTIME_DIR/advent-agent/agent.sock
```

If `XDG_RUNTIME_DIR` is absent, the socket falls back to the private state
directory. Directories and files are private to the OS user.

Working and long-term memory are deterministic, sorted INI:

```ini
[working]
database = SQLite with WAL mode
language = Go
```

```ini
[preferences]
answer-style = concise

[solutions]
sqlite-test-isolation = create a temporary database per test

[knowledge]
project-purpose = AI Advent Challenge homework
```

Keys are lowercased CRUD handles and may contain letters, digits, `.`, `_`,
and `-`. Values are non-empty, single-line UTF-8. Creates reject duplicates;
malformed files produce an error and are never overwritten. Writes use private
temporary files and atomic replacement.

## Commands

```text
/memory
/memory short
/memory working
/memory long
/remember working KEY VALUE
/remember preference KEY VALUE
/remember solution KEY VALUE
/remember knowledge KEY VALUE
/memory edit SCOPE KEY NEW_VALUE
/memory move SOURCE KEY DESTINATION
/forget SCOPE KEY
/clear
/instructions
/task
/continue
/back [planning|execution|validation]
```

Memory placement is always explicit; nothing is promoted automatically.
Mutations are rejected while an answer or compression operation is active.
`/clear` removes the current task together with the session's dialog and
summary. Metrics, working memory, and long-term memory remain.

Every answer reloads memory, so CRUD changes affect the next prompt. Prompt
context order is snapshotted instructions, the harness-owned task block,
long-term INI, working INI, rolling summary, then current dialog. The task
block contains the original objective, current phase, fixed phase instruction,
and relevant prior attempts. Memory blocks are labeled contextual data, not
behavioral instructions. Factual conflict precedence is current dialog >
working memory > long-term memory.

`/task` displays the objective, phase, status, deterministic expected action,
and phase-attempt history. Failed or daemon-interrupted phase calls remain in
that phase: `/retry` reruns them and `/discard` restores the exact pre-operation
task snapshot. Checkpoints include task state, so branches evolve independently
from the selected snapshot while profile and project memories remain shared.

The supported context strategies are `full`, `summary`, `sliding`, and
`branching`. Week 1 compression, token/cost accounting, leases, daemon
recovery, and branch checkpoints remain. Branches snapshot dialog state while
sharing their profile's project working memory and long-term memory. The old
automatic `sticky-facts` strategy was removed because memory placement is now
explicit.

## Reproducible demonstration

After `make test && make build`, use the rebuilt `week2/advent-agent`:

1. Run `./advent-agent --profile coding`, enter a task, and use `/task` to
   inspect the paused `planning` state.
2. Exit and run `./advent-agent --profile coding resume SESSION_ID`; `/task`
   shows the objective and plan without asking for either again.
3. Send planning feedback and inspect the superseded attempt with `/task`, then
   use `/continue` to run `execution`.
4. Use `/back planning` to revise from an earlier phase, then `/continue`
   through `execution`, `validation`, and `done`, inspecting every pause.
5. Start a second task with a normal message after `done`. Use `/clear` to show
   task/dialog state is removed while `/memory` and `/stats` remain.

See `config.example.toml` for all harness settings.
