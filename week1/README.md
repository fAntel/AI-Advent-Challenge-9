# deepseek-agent

`deepseek-agent` is a persistent chat agent for DeepSeek. Unlike Week 0's
single-process CLI, the user interface does not call DeepSeek itself:

```text
deepseek-agent CLI -> HTTP over private Unix socket -> Agent -> DeepSeek API
```

The separate `deepseek-agentd` process owns prompt construction, API requests,
response validation, conversation history, background work, and persistence.
Closing the CLI does not cancel a running request.

## Build and test

From `week1/`:

```sh
make test
make build
```

Do not use `go build -o deepseek-agentd .`: the module root is the reusable
`agent` library, so that command creates a Go archive rather than an executable.
The `make build` target names both command packages explicitly and executes
their help commands to verify the resulting artifacts.

The resulting demonstration artifacts are `week1/deepseek-agent` and
`week1/deepseek-agentd`.

## Configuration and API key

Configuration is loaded from built-in defaults, then
`/etc/deepseek-agent/config.toml`, then
`~/.config/deepseek-agent/config.toml`. Later values override earlier ones.
Unknown keys and invalid values are rejected. See
[`config.example.toml`](config.example.toml) for every setting.

The API key is deliberately not accepted in TOML. Export it in the shell that
starts the client or daemon:

```sh
export DEEPSEEK_API_KEY='your-api-key'
./deepseek-agent
```

An exported value in `~/.zprofile` is inherited when the client or daemon is
launched from a login shell that sourced that file. Neither program reads shell
profiles itself. By default, the client automatically starts `deepseek-agentd`
as a detached process when its socket is unavailable. The daemon inherits the
client's environment, including the exported key. Set `client.autostart = false`
in TOML to require manual startup with `./deepseek-agentd`.

Stop either an automatically or manually started daemon gracefully with:

```sh
./deepseek-agentd stop
```

`./deepseek-agentd --stop` is an equivalent spelling. The stop command does not
need `DEEPSEEK_API_KEY`; access is controlled by the private per-user socket.
It is safe to run when the daemon is already stopped; in that case it prints
`deepseek-agentd is not running` and exits successfully.

The daemon runs as the user who started it and uses that user's
private socket and session directory, so another OS user cannot list or resume
those sessions.

## Chat and sessions

Start a new interactive session:

```sh
./deepseek-agent
```

The CLI prints the persistent session ID. Type one message per line. `/exit`
disconnects; a request already accepted by the daemon continues in the
background. Resume by ID, or run `resume` without an ID for an interactive
session picker:

```sh
./deepseek-agent resume SESSION_ID
./deepseek-agent resume
./deepseek-agent delete SESSION_ID
```

One CLI may attach to a session at a time. The CLI renews a lease while it is
connected. When no CLI is connected and no request is running, the daemon saves
and removes the session from RAM; resuming loads it from disk.

One-shot and redirected input are also supported:

```sh
./deepseek-agent -p 'Explain goroutines briefly'
printf 'Explain goroutines briefly' | ./deepseek-agent
```

## Session prompt and answer controls

Set the immutable session system prompt at creation, or configure a default in
the TOML `[agent]` section:

```sh
./deepseek-agent --system-prompt 'You are a concise Go tutor.'
./deepseek-agent --system-prompt-file system-prompt.txt
```

The Week 0 controls remain available as long flags (with the original short
forms where applicable): `--format`, `--length`, `--stop`, `--approach`,
`--roles`, `--temperature`, `--model`, `--reasoning`, `--stats`, and `--debug`.
In the REPL, `/help` lists their command equivalents and `/settings` displays
the current values.

## Token accounting and context growth

After every successful API call, the agent stores DeepSeek's returned usage
alongside the session. These are the authoritative server-side counts rather
than a local tokenizer estimate. The statistics distinguish:

- **latest API-call input context** (`prompt_tokens`): the system prompt plus
  the complete dialog history sent to the model, including the newest message;
- **latest model answer** (`completion_tokens`);
- **whole dialog**: cumulative input, output, total tokens, API calls, and
  estimated cost across all calls in the session.

Type `/stats` at any time to print the current summary. `/stats on` prints it
after every answer and `/stats off` disables that automatic per-answer output.
`/exit` always prints one final session summary, regardless of that setting:

```text
Session token stats:
  latest API call: input context=200 (cache hit=30, miss=170), model answer=25, total=225
  context window: 200 / 1000000 (0.02%)
  whole dialog: calls=2, cumulative input=220, model answers=30, total=250
  estimated dialog cost: $0.00003508 USD
  input-context growth: 20 -> 200 (+180 tokens)
  latest finish reason: stop
```

Because each request resends the conversation, the latest input context and
the cumulative billed input normally grow during a long dialog. Per-call
records and totals are persisted in the session JSON, so they survive daemon
restart and RAM eviction. Sessions created by an older build retain their
messages, but their cumulative totals cover only calls made after this token
tracking feature was added; the CLI labels that case explicitly.

The default `agent.context_window_tokens` is 1,000,000 for the current DeepSeek
models and is used for the utilization display and its warning at 90%. It can
be changed in TOML if the selected model's limit changes. If DeepSeek returns
`finish_reason = "length"`, the CLI keeps and prints the partial answer but
marks the operation `truncated` and displays a warning. If the API instead
rejects an oversized input context, the operation fails with an explicit
`model context window exceeded` message; no usage is invented for a rejected
call.

For a practical comparison, start a fresh session, ask one short question,
run `/stats`, continue with several questions that depend on earlier answers,
and run `/stats` again. The `input-context growth` line compares the first and
latest calls. The automated tests also exercise a simulated token-limit answer
and context-window rejection without sending a costly million-token request:

```sh
go test ./...
```

For an optional real context stress test, generate compact synthetic input and
load it from a file so macOS's command-line argument limit is not involved:

```sh
awk 'BEGIN { for (i = 0; i < 900000; i++) printf "x " }' > /tmp/context-900k.txt
./deepseek-agent --system-prompt-file /tmp/context-900k.txt --stats \
  -p 'Reply with only OK.'
```

Increase `900000` gradually and use the returned `prompt_tokens`, not the word
count, to find the actual boundary. The daemon accepts request bodies up to
8 MiB so a million-token text payload can pass through to DeepSeek. Requests
larger than that are rejected locally to prevent accidental unbounded memory
use. A near-limit request can incur API charges.

Cost is an estimate calculated from the cache-hit input, cache-miss input, and
output rates in DeepSeek's [current pricing table](https://api-docs.deepseek.com/quick_start/pricing)
(checked 2026-09-14). DeepSeek documents the response fields and `length`
finish reason in its [Chat Completion API reference](https://api-docs.deepseek.com/api/create-chat-completion/)
and notes that local tokenizers only estimate API usage in its
[token usage guide](https://api-docs.deepseek.com/quick_start/token_usage/).

`/stop TEXT` enables the clarification workflow for subsequent requests and
keeps that sequence until it is replaced. `/unset stop` returns to ordinary
chat. The agent uses the whole session history during clarification:

```text
/stop THAT'S IT
You: Plan a weekend trip
Agent: Which city are you leaving from?
You: Moscow. THAT'S IT
Agent: ...final answer...
```

Failed or daemon-interrupted work is never retried automatically. Use `/retry`
to repeat it or `/discard` to remove that unfinished workflow.

## Debug logging

No log file is created while daemon debug mode is disabled. Enable it in
`~/.config/deepseek-agent/config.toml` and restart an already-running daemon:

```sh
[daemon]
debug = true
```

```sh
./deepseek-agentd stop
./deepseek-agent
tail -n +1 -f "$HOME/Library/Logs/deepseek-agent/agentd.log"
```

The client autostarts the daemon using the updated configuration. Alternatively,
start it manually with `./deepseek-agentd --debug`. The `tail` command displays
the complete existing log and continues printing new events; `Ctrl+C` stops
watching without stopping the daemon.

The private, mode-`0600` log records session loading, attachment, API request
and response, token usage and estimated cost, answer receipt, saving, lease
expiry, and RAM eviction. HTTP authorization is always written as
`Bearer [REDACTED]`, and literal occurrences of the API key are redacted from
headers, bodies, URLs, and errors. Logs rotate at 10 MiB by default and retain
three backups.

Debug logs still contain complete prompts, system prompts, conversation data,
and model answers. Treat them as sensitive. The client `/debug` setting is
separate: it returns masked HTTP diagnostics for that operation, while daemon
debug mode controls the persistent lifecycle log.
