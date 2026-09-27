# advent-agent — Week 3

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

For direct conversation without a task lifecycle, use `-p`/`--prompt` or start
an interactive chat with `--chat`:

```sh
./advent-agent -p "Find the Desk lamp in HomeBox and tell me its entity ID"
./advent-agent --chat
```

Chat sessions keep their mode when resumed. They use the same MCP discovery and
approval flow, with no planning, execution, or validation phase. Plain
`./advent-agent` still starts task-oriented interactive sessions.

## Build and test

From `week3/`:

```sh
make test
make build
```

The exact runnable artifacts are `week3/advent-agent`, `week3/advent-agentd`,
`week3/homebox-mcp`, `week3/budget-mcp`, `week3/report-mcp`, and `week3/supplier-mcp`. `make build` builds all six command packages and
verifies their help commands, so demonstrations use newly rebuilt binaries.

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
resolves path-like server executables against the CLI's current directory, so
the daemon can launch them even when it runs from another directory. `add`
does not launch the server. First discovery or invocation starts a long-lived
stdio connection; removal and shutdown close it. `mcp tools` follows every
server page and prints the total tool count after the inventory. Positive MCP TTLs allow cached discovery until expiry, while zero
or missing TTLs require a new discovery. Refresh errors are shown and leave the
last snapshot on disk. This task supports stdio commands only.

Run a registered tool directly, without a model or a pipeline, to inspect its
result in the terminal:

```sh
./advent-agent mcp call homebox list_tags
./advent-agent mcp call homebox search_entities --args '{"tags":["TAG_ID"],"pageSize":100}'
```

The explicit `mcp call` command authorizes that call, including write-capable
tools. The CLI validates arguments against the tool schema and prints text
results as readable JSON when possible.

## HomeBox MCP server (HomeBox 0.26.2)

`homebox-mcp` connects to HomeBox's authenticated HTTPS API. Create a private
JSON config outside the repository, for example:

```sh
mkdir -p ~/.config/advent-agent
cat > ~/.config/advent-agent/homebox-mcp.json <<'JSON'
{
  "url": "https://homebox.example.com",
  "apiKey": "YOUR_HOMEBOX_PERSONAL_API_KEY"
}
JSON
chmod 600 ~/.config/advent-agent/homebox-mcp.json
```

The URL is the HomeBox base URL, without `/api`. `caFile` may be added for a
private certificate authority. The config path, rather than the key, is stored
in the MCP catalog. The server rejects a config readable by other users.

After `make test && make build`, register and inspect the rebuilt artifacts:

```sh
./advent-agent mcp add homebox --description "HomeBox inventory" -- ./homebox-mcp --config ~/.config/advent-agent/homebox-mcp.json
./advent-agent mcp tools --refresh homebox
./advent-agent -p "Find the Desk lamp in HomeBox and tell me its entity ID"
```

In interactive chat (`./advent-agent --chat`), ask: `Find the Desk lamp in HomeBox and tell me its entity ID.` The
model can discover `search_entities`, `get_entity`, `list_entity_types`,
`list_tags`, `create_entity`, and `update_entity`. Search supports `query`,
`page`, `pageSize`, `tags`, and `parentIds`. Create accepts a name and optional
description, type ID, parent ID, quantity, and tag IDs. Update patches the type,
parent, quantity, or tag IDs; supply an empty `tagIds` array to clear tags.
Read tools run automatically. Create and update trigger the harness's approval
prompt before an API request is sent.

For a reproducible HTTPS mock, start this in another terminal from `week3/`:

```sh
go run ./testdata/homebox-mock --config /private/tmp/homebox-demo.json
```

Then use the same `mcp add`, `mcp tools`, and chat commands above with
`--config /private/tmp/homebox-demo.json`. The mock returns a Desk lamp
(`item-1`, quantity 1), LED bulb (`item-2`, quantity 0), and Floor lamp
(`item-3`, quantity 4). Stop the mock with Ctrl-C. The end-to-end Go test also verifies
the harness discovers the tool, calls the stdio server, and uses `Desk lamp`
from the API result in its answer.

The model sees server names and local descriptions at each answer request.
It can call three fixed functions: `list_mcp_tools` (20 matches per page),
`describe_mcp_tool` (one full schema), and `call_mcp_tool`. Descriptions,
schemas, annotations, and results from servers are untrusted data. A tool
explicitly marked read-only and not destructive is invoked automatically; write-capable or ambiguous
tools pause for `Execute? [y/N]`. Denial is returned to the model. Piped or
otherwise non-interactive input denies automatically. A pending approval
survives daemon restart. The loop permits at most twelve model rounds and
thirty-two internal calls, then requests a final answer with tools disabled.

Interactive sessions print numbered MCP events in tool order. They show the
chosen server, tool, and status, without tool arguments or results. The current
operation keeps these events after completion; a new request starts a new
sequence.

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

## Three-server orchestration demonstration

`supplier-mcp` is a read-only local offer catalog. Its `search_offers` tool
returns matching offers with SKU, unit price, currency, available quantity,
shipping cost, and delivery days. It does not rank or select an offer; the
user's request supplies the selection rule. The fixture includes an unavailable
low-priced Desk lamp offer so the agent must check stock as well as price.

After `make test && make build`, start the HomeBox mock shown above, then
register all three rebuilt MCP binaries from `week3/`:

```sh
./advent-agent mcp add homebox --description "HomeBox inventory and item quantities" -- "$PWD/homebox-mcp" --config /private/tmp/homebox-demo.json
./advent-agent mcp add supplier --description "Read-only supplier offers with price, stock, shipping and delivery facts" -- "$PWD/supplier-mcp"
./advent-agent mcp add report --description "Summarize inventory and save Markdown reports" -- "$PWD/report-mcp" --output-dir /private/tmp/advent-reports
./advent-agent mcp tools --refresh homebox
./advent-agent mcp tools --refresh supplier
./advent-agent mcp tools --refresh report
./advent-agent --chat
```

In chat, ask: `Inspect all HomeBox inventory. For each item with quantity below
3, search supplier offers and recommend the cheapest available total cost for
the quantity needed to reach 3 (unit price times quantity plus shipping).
Summarize the inventory and save a replenishment report named
replenishment.md. Do not change inventory or place an order.` Approve the
`report.save_report` call when prompted. The terminal's numbered MCP events
show discovery and calls across HomeBox, supplier, and report in order. Check
`/private/tmp/advent-reports/replenishment.md` for the saved recommendation.

The scripted end-to-end test uses the same three server types and checks tool
routing, call order, report content, and approval handling. A live run shows
the model's own tool choices; the scripted test verifies the harness mechanics.
If `supplier` was registered before the binary was built, run `make build` and
restart `advent-agentd` before retrying. Existing `./supplier-mcp` catalog
entries can resolve the binary next to the rebuilt daemon. `mcp list` shows
the stored command, and `mcp tools --refresh supplier` checks discovery before
starting a chat.

## Configured MCP pipelines

A named pipeline runs registered MCP tools in order without a model. Each step
has literal JSON arguments and optional bindings from an earlier step's
structured result. A binding uses `ARG=STEP:/json/pointer`; use
`ARG=STEP:$text` to pass the first text result. Bindings replace top-level
arguments and cannot conflict with literal arguments. A missing field, invalid
resolved argument, or tool error stops the pipeline before later steps run.
Definitions are stored in private `pipelines.toml` under the configuration
directory. Pipeline runs and schedules explicitly authorize all configured
steps, including write-capable tools.

After `make test && make build`, register the rebuilt HomeBox and report MCP
binaries. For the local HomeBox mock shown above, use its private config file:

```sh
./advent-agent mcp add homebox --description "HomeBox inventory" -- ./homebox-mcp --config /private/tmp/homebox-demo.json
./advent-agent mcp add report --description "Inventory reports" -- ./report-mcp --output-dir /private/tmp/advent-reports
./advent-agent pipeline add lamps
./advent-agent pipeline step add --server homebox --tool search_entities --args '{"query":"lamp"}' lamps search
./advent-agent pipeline step add --server report --tool summarize_inventory --bind 'items=search:/items' lamps summary
./advent-agent pipeline step add --server report --tool save_report --args '{"filename":"lamps.md"}' --bind 'content=summary:/markdown' lamps save
./advent-agent pipeline show lamps
./advent-agent pipeline run lamps
./advent-agent schedule add --pipeline lamps --every 1h lamp-report
./advent-agent schedule run lamp-report
```

`pipeline list`, `pipeline step remove PIPELINE STEP`, and `pipeline remove NAME`
manage definitions. A pipeline used by a schedule cannot be removed until the
schedule is removed. Scheduled runs use the current pipeline definition. The
report MCP server writes atomically inside its configured output directory and
returns the saved file path. The mock pipeline writes
`/private/tmp/advent-reports/lamps.md` containing `Desk lamp` and `item-1`.

## Scheduled MCP calls and budget reports

`advent-agentd` runs delayed and recurring calls to registered MCP tools. Jobs
and their last results are saved in private `schedules.toml` under the user
configuration directory. The first daemon start with the rebuilt `budget-mcp`
next to `advent-agentd` registers that server and creates an hourly `budget`
job. Set `[scheduler].default_interval` in `config.toml` to change the default
interval; jobs using `default` adopt the new interval after their next run.
Removing the budget job persists across daemon restarts.

```sh
./advent-agent schedule list
./advent-agent schedule reload
./advent-agent schedule run budget
./advent-agent schedule add --server budget --tool get_report --every 6h --notify budget-six-hours
./advent-agent schedule add --server budget --tool get_report --at 2026-10-01T09:00:00+03:00 budget-once
./advent-agent schedule remove budget-six-hours
```

`--args '{"name":"value"}'` supplies tool arguments. A scheduled job is an
explicit authorization for its future tool calls, including write-capable
tools; the daemon validates the tool and arguments when the job is added.
After editing `schedules.toml` by hand, run `./advent-agent schedule reload`.
Changing `every` starts a new interval from reload time and writes the updated
`next_run` back to TOML. Restarting the daemon also detects an interval change.
After sleep or restart, an overdue recurring job runs once before resuming its
interval. `schedule run` starts a job immediately. Results and errors are
recorded in macOS unified logging under subsystem
`dev.aiadvent.advent-agent`, category `scheduler`:

```sh
log show --last 1d --predicate 'subsystem == "dev.aiadvent.advent-agent" && category == "scheduler"'
```

The budget MCP stores balance samples in private
`~/.local/state/advent-agent/budget/snapshots.json`. Its `capture_balance`
tool queries DeepSeek and stores a sample; `get_report` reads the latest
sample. Reports show balance in DeepSeek's currency, observed balance change
between samples, and separately the daemon's estimated USD cost and tokens
for the last hour and current local day. A balance increase is labeled an
adjustment. The daemon keeps a separate private `usage.jsonl` ledger so
deleting sessions does not erase these usage totals. Pre-existing calls are
not backfilled into that ledger.

Connected interactive chats receive new scheduled reports through a Unix-socket
event stream. A reconnecting chat sees the latest report. Notices are gray on
color terminals and stay out of conversation history and model context.

On macOS, a per-user LaunchAgent can keep the daemon running after login and
restart it if it exits. Store the DeepSeek API key in Keychain before installing
the service; `service key set` reads it without echoing it or placing it in a
command argument. `DEEPSEEK_API_KEY` remains supported for manually started
daemon chat calls, while `budget-mcp` reads Keychain directly.

```sh
./advent-agent service key set
./advent-agent service install
./advent-agent service status
./advent-agent service stop
./advent-agent service start
./advent-agent service uninstall
```

The installed plist is `~/Library/LaunchAgents/dev.aiadvent.advent-agent.plist`.
`service stop` unloads the LaunchAgent so `KeepAlive` does not immediately
restart it. Jobs remain in TOML across logout and reboot, and resume after the
next login. The service uses the absolute path of the rebuilt `advent-agentd`
at installation time; reinstall after moving the binaries.

For a cross-SDK smoke test, register Everything as shown above, inspect its
tool inventory, then run `./advent-agent` and ask for a task that finds,
inspects, and calls an appropriate Everything tool. The staged inventory
keeps full schemas out of the initial context. Other servers worth considering
for a separate comparison are Filesystem, Git, Fetch, Time, and Memory; none
is scanned or registered automatically.
