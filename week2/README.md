# advent-agent — Week 2 Task 0

`advent-agent` is a persistent, provider-neutral chat harness with explicit,
profile-scoped memory. The shipped provider adapter is DeepSeek. The CLI talks
to `advent-agentd` over a private Unix socket; the daemon owns requests,
sessions, compression, branches, accounting, and persistence.

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
```

Memory placement is always explicit; nothing is promoted automatically.
Mutations are rejected while an answer or compression operation is active.
`/clear` removes only the current session's dialog and summary. Metrics,
working memory, and long-term memory remain.

Every answer reloads memory, so CRUD changes affect the next prompt. Prompt
context order is snapshotted instructions, long-term INI, working INI, rolling
summary, then current dialog. Memory blocks are labeled contextual data, not
behavioral instructions. Factual conflict precedence is current dialog >
working memory > long-term memory.

The supported context strategies are `full`, `summary`, `sliding`, and
`branching`. Week 1 compression, token/cost accounting, leases, daemon
recovery, and branch checkpoints remain. Branches snapshot dialog state while
sharing their profile's project working memory and long-term memory. The old
automatic `sticky-facts` strategy was removed because memory placement is now
explicit.

## Reproducible demonstration

After `make test && make build`, use the rebuilt `week2/advent-agent`:

1. Run `./advent-agent --profile coding`, then store a long-term preference
   and project constraint with `/remember preference ...` and `/remember
   working ...`; ask a question that makes both visible.
2. Start another coding session in the same directory and use `/memory` to
   show both persist.
3. Run the coding profile from another Git repository or directory; `/memory`
   shows long-term memory but an empty working section.
4. Run `./advent-agent --profile writer resume`; its session list, profile
   config (including temperature/model), snapshotted `/instructions`, and
   memory are independent.
5. Store writer tone and genre entries, ask for prose, then use `/memory edit`,
   `/memory move`, and `/forget` and ask again to show immediate changes.
6. Run `/clear`, followed by `/memory`, to show only dialog and rolling summary
   were removed.

See `config.example.toml` for all harness settings.
