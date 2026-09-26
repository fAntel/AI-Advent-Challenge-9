# advent-agent — Week 3 Task 0

`advent-agent` is a persistent, provider-neutral chat harness with explicit,
profile-scoped memory. The shipped provider adapter is DeepSeek. The CLI talks
to `advent-agentd` over a private Unix socket; the daemon owns requests,
sessions, compression, branches, accounting, and persistence.

Every user task is controlled by a persisted daemon-owned lifecycle with
project invariants enforced above task and memory context:

```text
planning → execution → validation → done
```

Each phase makes an answer request and then pauses. `/continue` explicitly
approves that phase and advances exactly one step. A normal message while
paused is feedback and reruns the current phase; it cannot skip or change the
lifecycle state. `/back` reruns the immediately preceding phase, while `/back
planning|execution|validation` returns to a named earlier phase. After `done`,
the next normal message starts a new task in the same session.

On a feedback rerun, the harness tells the model to recognize requests to skip,
finish, or change phase. The response must briefly explain that only the
harness can perform transitions and that `/continue` is required, then still
return the current phase's updated artifact. This keeps the latest attempt
useful as authoritative context for validation and later phases instead of
replacing it with a transition explanation alone.

Invalid transitions are rejected without changing task state or calling the
provider. The error explains why the transition is inadmissible and which
action is currently expected. This includes attempts to advance a missing,
running, failed, blocked, or terminal task and attempts to move `/back` to the
current phase, a later phase, `done`, or an unknown phase.

When invariants conflict with a phase, the daemon displays a refusal citing the
stored rules and sets the task to `blocked`. `/continue` cannot advance a
blocked task. Normal feedback reruns the same phase, `/back` remains available
from later phases, and `/clear` abandons the task.

The daemon can connect to registered stdio MCP servers and execute their tools
during any phase. Tool use does not advance the lifecycle. Raw DSML/XML tool
syntax is still rejected; DeepSeek must use native tool calls.

## Build and test

From `week3/`:

```sh
make test
make build
```

The exact runnable artifacts are `week3/advent-agent` and
`week3/advent-agentd`. `make build` builds both command packages and verifies
their help commands, so demonstrations use newly rebuilt binaries.

MCP administration and daemon startup work without `DEEPSEEK_API_KEY`. Export
the key in the daemon's environment before submitting a chat request. The
client starts the daemon automatically by default. Stop it with
`./advent-agentd stop`.

## MCP catalog and tools

```sh
./advent-agent mcp add everything --description "Official MCP client test server" -- npx -y @modelcontextprotocol/server-everything
./advent-agent mcp list
./advent-agent mcp tools --refresh everything
./advent-agent mcp remove everything
```

Definitions and cached tool metadata are stored globally under the daemon
state directory in `mcp-catalog.json`, with private file permissions. `add`
does not launch the server. First discovery or invocation starts a long-lived
stdio connection; removal and shutdown close it. `mcp tools` follows every
server page and prints the total tool count after the inventory. Positive MCP TTLs allow cached discovery until expiry, while zero
or missing TTLs require a new discovery. Refresh errors are shown and leave the
last snapshot on disk. This task supports stdio commands only.

The model sees server names and local descriptions at each answer request.
It can call three fixed functions: `list_mcp_tools` (20 matches per page),
`describe_mcp_tool` (one full schema), and `call_mcp_tool`. Descriptions,
schemas, annotations, and results from servers are untrusted data. A tool
explicitly marked read-only and not destructive is invoked automatically; write-capable or ambiguous
tools pause for `Execute? [y/N]`. Denial is returned to the model. Piped or
otherwise non-interactive input denies automatically. A pending approval
survives daemon restart. The loop permits at most eight model rounds and
sixteen internal calls, then requests a final answer with tools disabled.

Interactive terminal labels use cyan `You:`, magenta `Agent:`, and yellow
approval prompts. Redirected output, `TERM=dumb`, and any defined `NO_COLOR`
disable ANSI colors.

For Xcode's `xcrun mcpbridge`, open a project in Xcode and enable **Allow
external agents to use Xcode tools** under Xcode Settings → Intelligence.
If the bridge cannot initialize, `mcp tools` includes its bounded stderr
diagnostic. MCP connection and discovery pages time out after 30 seconds.

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
    │   ├── working.ini
    │   └── invariants.ini
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

## Project invariants

Invariants are flat, explicit rules scoped to a profile and canonical project.
They are shared by branches in that scope and survive restart, resume,
compression, and `/clear`. A missing `invariants.ini` means no invariants are
active, so existing projects need no migration.

```ini
[invariants]
architecture = Use ports and adapters
technology = Use Go
```

Invariant keys and values use the same normalization and single-line UTF-8
validation as memory, but invariants are not memory. Only `/invariant` CRUD can
change them; dialog, task text, instructions to ignore them, and memory cannot.
Mutations are rejected while an answer or compression is active.

For every phase with active invariants, the daemon requires one strict model
response containing an allow/refuse decision, acknowledgement of every active
ID, violated IDs, a concise explanation, and (when allowed) the answer. Invalid
or incomplete responses fail safely and can be handled with `/retry` or
`/discard`. Each attempt stores its invariant snapshot and decision so later
edits do not rewrite task history. No private chain-of-thought is requested or
displayed.

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
/invariants
/invariant add KEY RULE
/invariant edit KEY RULE
/invariant delete KEY
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

Every answer reloads invariants and memory, so CRUD changes affect the next
phase. Prompt context order is snapshotted instructions, harness-owned
invariants, the harness-owned task block, long-term INI, working INI, rolling
summary, then current dialog. The task
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

After `make test && make build`, use the rebuilt `week3/advent-agent`:

1. Run `./advent-agent --profile coding`, enter a task, and use `/task` to
   inspect the paused `planning` state.
2. Exit and run `./advent-agent --profile coding resume SESSION_ID`; `/task`
   shows the objective and plan without asking for either again.
3. Send planning feedback such as `start implementation now`; `/task` still
   reports `planning/paused`, while the answer explains that ordinary dialog
   cannot approve or skip a phase and still returns an updated plan.
4. Try `/back execution` from planning and inspect the explicit invalid
   transition error. `/task` remains unchanged.
5. Use `/continue` to approve planning and run `execution`. Try `/back
   validation`; the forward transition is rejected and execution remains
   paused.
6. Use `/continue` through `validation` and `done`, inspecting every pause to
   show that completion cannot bypass validation.
7. Start a second task with a normal message after `done`. Use `/clear` to show
   task/dialog state is removed while `/memory` and `/stats` remain.

For the invariant guardrail flow, add architecture and technology rules with
`/invariant add`, submit a contradictory task, inspect its cited refusal and
blocked `/task` state, verify `/continue` is rejected, then provide compliant
feedback. Resume the session to inspect preserved decision history, and use a
different profile or project to verify isolation.

See `config.example.toml` for all harness settings.

For a cross-SDK smoke test, register Everything as shown above, inspect its
tool inventory, then run `./advent-agent` and ask for a task that finds,
inspects, and calls an appropriate Everything tool. The staged inventory
keeps full schemas out of the initial context. Other servers worth considering
for a separate comparison are Filesystem, Git, Fetch, Time, and Memory; none
is scanned or registered automatically.
