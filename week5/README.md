# Week5 Task0 — Local LLM launch

Run a local text-generation model with Ollama and demonstrate three requests of
increasing difficulty. `local-llm` is a standalone Go CLI using only the standard
library; it calls the local HTTP API and saves the prompts, streamed answers,
completion status, and timing metrics.

## Model and hardware

- Target: MacBook Air M1 with 8 GB unified memory.
- Model: **Spark-X2.5-4B**, a text-only general-purpose model with coding capabilities.
- Model release: **September 1, 2026**, within three months of this task.
- Quantization: **Q4_K_M**, approximately **2.6 GB** of weights.
- Exact identifier: `hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M`.
- Context: 4,096 tokens; output limit: 2,048 tokens; one request at a time.
- Thinking is disabled for this short demonstration. Returned thinking text is
  still retained in the report, separately from the answer.

Ollama is inference software; Spark is the model it runs. The default
`SparkLLM/Spark-X2.5-4B` download contains 8.2 GB BF16 weights: use the explicit
quantized identifier above on an 8 GB machine. Weight size does not include runtime
buffers, KV cache, or macOS memory usage.

Sources: [official release history](https://github.com/XHToken/Spark-X2.5),
[model and author-reported coding benchmarks](https://huggingface.co/XHToken/Spark-X2.5-4B),
[official GGUF variants and runtime requirements](https://huggingface.co/XHToken/Spark-X2.5-4B-GGUF),
[BF16 Ollama package](https://ollama.com/SparkLLM/Spark-X2.5-4B).

## Prerequisites and setup

Go 1.27 or later and Ollama 0.34.1 or later are required. This Mac already has
Ollama installed. Reuse its service if it is running; otherwise start it with
`brew services start ollama`, or run `ollama serve` in another terminal.

Check the running server, download the model, and inspect its quantization:

```sh
curl -fsS http://127.0.0.1:11434/api/version
ollama pull hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M
ollama show hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M
```

Downloads need internet access. Once the model is installed, generation uses
`http://127.0.0.1:11434`, with no cloud fallback or additional dependencies.

## Build and test

From `week5/`:

```sh
make test
make build
```

Tests use mock HTTP servers and need neither internet access nor an installed
model. They cover streaming, thinking text, metrics, HTTP and generation errors,
malformed or interrupted responses, deadlines, missing models, empty answers,
token-limit truncation, and preservation of partial results.

The build produces `/Users/keldzh/Develop/AiAdventChallenge/week5/local-llm`
alongside the Task1 agent executables and checks their help commands. The binary and model weights are excluded from Git.

## Run the demonstration

After tests and the build succeed, run this freshly rebuilt artifact from `week5/`:

```sh
./local-llm demo
ollama ps
```

The three independent English prompts are:

1. **Easy:** explain stacks and queues, with practical examples.
2. **Medium:** implement a balanced-bracket checker in Go, covering edge cases
   and explaining complexity.
3. **Hard:** diagnose and correct an LRU cache that does not refresh recency on
   reads and corrupts bookkeeping when updating existing keys. The complete
   defective implementation is part of the fixed prompt.

The CLI prints prompts, streams answer text, then prints elapsed time, generated
token count, and completion reason. It uses temperature 1.0, top-p 0.95, and
top-k -1. Requests have a ten-minute timeout and keep the model loaded for ten
minutes after use, allowing inspection with `ollama ps`.

Flags: `--model MODEL`, `--output PATH`, and `--timeout DURATION`. The default
report is `results/task0.json`; each run replaces it. Save a recording run to
another path if you want to retain the submitted capture.

Reports include server version, model metadata, all prompts and answers, any
thinking text, timing metrics in Ollama's nanoseconds, elapsed seconds, token
counts, and `/api/ps` data identifying the loaded model, memory placement, and
context size. See [chat API](https://docs.ollama.com/api/chat) and
[running models API](https://docs.ollama.com/api/ps).

An unavailable server, missing model, timeout, invalid stream, or generation error
produces a clear error and unsuccessful exit. Completed and partial responses
are saved after every request. A token-limit completion is marked `truncated`
and treated as unsuccessful. Ctrl-C cancels generation and preserves partial
results. Generated code is captured as text and is never executed by the CLI.

## Video recording sequence

1. Show the selected model and Q4_K_M quantization with `ollama show`.
2. Show `make test` and `make build` succeeding.
3. Run `./local-llm demo --output results/video.json` and show all three answers.
4. Show `ollama ps`, demonstrating the model is loaded locally.
5. Open the saved JSON report and discuss answer quality and observed timings.

Record the terminal using the macOS screen recorder. The video itself is recorded
and uploaded by the user; this repository supplies the code and reproducible
recording sequence.

## Observed results

Validated on October 10, 2026 (Moscow time), on a **MacBook Air M1, 8 GB RAM**.
The Ollama CLI was 0.40.1 and the running server was 0.35.1. Model inspection
confirmed `spark2_5`, 4.11B parameters, and Q4_K_M quantization.

`make test` and `make build` both passed before the captured demonstration.
The final run exited successfully and all three requests finished with `stop`,
without truncation. Actual responses and metadata are in
[results/task0.json](results/task0.json).

| Difficulty | Elapsed time | Generated tokens | Generation speed |
| --- | ---: | ---: | ---: |
| Easy | 10.66 s | 174 | 17.2 tokens/s |
| Medium | 23.51 s | 368 | 16.2 tokens/s |
| Hard | 45.15 s | 640 | 14.8 tokens/s |

Generation speed is `eval_count / (eval_duration / 1e9)`; elapsed time also
includes prompt evaluation and any model loading. These are single-run
observations, not a benchmark. The final run reused the already loaded model.

`ollama ps` reported **2.7 GB, 100% GPU, context 4096**. The HTTP snapshot
reported both `size` and `size_vram` as **2,743,074,814 bytes**. Its quantization
field was `unknown`; `/api/show` metadata in the same report explicitly confirms
`Q4_K_M`.

Manual answer review (generated code was not compiled or executed):

- **Easy:** correctly explains LIFO and FIFO with practical examples.
- **Medium:** the bracket checker appears correct, but the example table claims
  empty input returns `false`, contradicting both the code and the correct
  answer (`true`). One example is malformed, and the table does not demonstrate
  actual nested brackets.
- **Hard:** the implementation appears to fix both bugs by moving read entries
  to the front and updating existing entries in place. The explanation of
  duplicate entries is imprecise, and its operation sequences do not actually
  demonstrate an existing-key update.

The first run is retained in
[results/task0-initial.json](results/task0-initial.json). Its hard answer reached
the 2,048-token limit after 147.66 seconds, contained invalid Go
(`e.Value.entry.value = value`), and gave inconsistent recency examples. The CLI
flagged truncation and exited unsuccessfully. The prompts were then shortened
to request concise explanations and operation sequences; tests and rebuild
passed again before the final three-request run.

**Result:** the local model launches, is reachable through the local HTTP API,
and answers three requests of different difficulty. Its answers still require
review, especially code explanations and examples. The remaining submission
step is recording the video using the sequence above.

## Task1: local advent-agent

Week 5 carries the Week 4 Go harness forward and adds an Ollama provider.
`advent-agent` sends chat messages and conversation summaries to the installed
local model, displays completed answers, and persists sessions and explicit
project memory. Generation needs no API key, cloud LLM, or internet connection.
The inherited default provider remains DeepSeek; select `--profile local` for
local generation. There is no automatic provider fallback.

Task0's CLI and tests now live under `cmd/local-llm`; its executable, commands
and recorded results are preserved. `make test` runs all carried Go tests plus
race checks. `make build` builds eight Go executables, including `local-llm`,
`advent-agent` and `advent-agentd`, and checks their help commands. Python indexing
and the old virtual environment are not needed for Task1. The other carried
executables retain their inherited cloud/RAG behavior and are not the local app.
See [HARNESS.md](HARNESS.md) for inherited features and
[config.local.example.toml](config.local.example.toml) for local settings.

### Install and run

After `make test` and `make build` succeed, install the isolated profile:

```sh
python3 scripts/install-local-profile.py
```

The installer resolves `XDG_CONFIG_HOME` and `XDG_STATE_HOME` (using the usual
home-directory defaults), creates `advent-agent/profiles/local/config.toml`, and
refuses to overwrite different existing configuration. The profile overrides
provider, history and timeout settings inherited from shared configuration.
Its socket, sessions and logs live in a separate `advent-agent/local` state
directory. It does not change the default profile or installed Week 4 service.

These commands use the newly rebuilt
`/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent` from `week5/`:

```sh
./advent-agent --profile local -p 'Explain a stack in two sentences.' --stats
./advent-agent --profile local --chat
./advent-agent --profile local resume SESSION_ID
```

The CLI automatically starts its sibling `advent-agentd` at the local profile's
socket and verifies local-provider capability. Use `/exit` to leave a chat,
`/summary` to inspect its summary, `/settings` to inspect provider/model settings,
`/model` to display the current model, `/model MODEL` to change it,
and `/remember working language Go` to save explicit project memory. Invalid model
changes keep the previous setting. Use the
same project directory when resuming to keep project-scoped context consistent.
Stop only the isolated local daemon with:

```sh
./advent-agentd --profile local stop
```

Generation uses the native Ollama `/api/chat` endpoint with thinking disabled,
a 4096-token context, at most 1024 answer tokens, at most 256 summary tokens,
a ten-minute deadline and ten-minute keep-alive. The default model and sampling
settings match Task0. Requests are serialized within the local daemon. Answers
appear after completion; this agent does not stream tokens. Stats include actual
provider token counts, request timing and zero generation API charges, excluding
hardware costs. Ollama is reused rather than installed or started by the agent.

### History, failures and Task1 boundaries

Local summaries target at most 100 words and replace complete older exchanges
while retaining four recent messages. All summary calls use the same local
provider and session model. Failed or token-limited summaries preserve the prior
summary and original messages. A completed answer that hits its output limit is
retained with a truncation warning; retry/discard handle failed operations.

The input guard conservatively estimates UTF-8 bytes plus 64 units per message
(including role bytes) and 128 framing units, reserving the requested output
budget. This is not the model's tokenizer and can reject prompts that would fit.
It summarizes eligible older exchanges before an oversized answer request;
it never silently trims instructions, explicit memory or the current message.
Large project `AGENTS.md` files, long memory, or large individual exchanges can
therefore require shortening input or increasing configured context and available
memory. Starting a new session helps only when older history caused the excess.
Summary requests have their own budget guard.

Only loopback HTTP Ollama endpoints are accepted. The adapter refuses HTTP
redirects, `:cloud` models and models whose `/api/show` metadata identifies a
remote model or host. Model inspection happens before each generation request.
The local daemon never looks up cloud credentials, registers the cloud balance
tool, or starts automatic schedules/pipelines. Plain local chat does not use
model-directed tools. Each local request explicitly tells the model that files,
browsing and tools are unavailable, and that current releases cannot be verified
offline. Unsupported tool syntax remains rejected; `/retry` adds a corrective
text-only instruction, including for failed sessions created before this fix.
Rejected tool text is retained on the current operation for inspection and is
not added to accepted conversation history. Local RAG requests and task-lifecycle sessions are rejected
before generation; use `--chat` for interactive local conversations. Fully local
retrieval/scoring and autonomous tool calling remain future work.

Mock tests cover provider options, usage, errors, deadlines, model locality,
context guards, summary recovery, restart/resume, explicit memory, daemon
capabilities and provider mismatches. They do not assess the live model's answer
quality. The adapter shares the harness's persistent session and memory formats;
legacy DeepSeek sessions keep their existing provider interpretation.

### Task1 observed result

Validated on **October 10, 2026 (Moscow)** using the rebuilt Week 5 artifacts on
the same MacBook Air M1 with 8 GB RAM. Full Go tests, race checks and `make build`
passed. The isolated `local` profile is installed on this machine; its daemon
can run alongside the existing Week 4 service without replacing it.

The reproducible capture script uses the real CLI, strips `DEEPSEEK_API_KEY` from
child environments, saves session snapshots and checks completion. It uses a
temporary project directory so its explicit memory does not affect other projects.
After tests/build and profile installation, run:

```sh
python3 scripts/demo-local-agent.py --output results/task1/new-demo.json
```

It refuses to overwrite an existing report. The captured result is
[results/task1/demo.json](results/task1/demo.json), with a separate
[review](results/task1/review.md). Four answer calls and two local summary calls
completed with `stop`, including the final answer after daemon shutdown and
CLI auto-start/resume. Saved explicit working memory retains `language = Go`.
The model kept Cedar's name, Go language and offline-notes purpose after restart.

| Call | Elapsed generation request |
| --- | ---: |
| Initial project question | 4.84 s |
| Standard-library package question | 1.81 s |
| File-format question | 2.48 s |
| First local summary | 2.18 s |
| Project recall after daemon restart | 2.63 s |
| Second local summary | 3.15 s |

These are single-run HTTP request durations, excluding model inspection and CLI
polling. Total recorded provider usage is 846 input and 138 output tokens;
generation API charges are zero. `ollama ps` reported 2.7 GB, 100% GPU and a
4096-token context; the server reported version 0.35.1. The local MCP catalog
was empty and daemon capabilities correctly reported local RAG unavailable.

Answer review found a factual error: the model presented SQLite as a Go
standard-library package. `database/sql` is standard, but SQLite needs an
additional driver. The later summary repeated that generated mistake.
The integration and persistence checks passed; local summaries do not verify
facts, and generated technical advice still needs review. No generated code was
executed. Video recording/upload remain with the user.

### Interactive regression checks

Two follow-up fixes were validated after the original capture:

- `/model` without an argument now displays the selected model, and invalid
  changes leave it intact. [The live check](results/task1/model-command-check.json)
  shows `/model`, a successful local answer, and another `/model` inspection.
- A plain local chat inherited repository instructions to read files and emitted
  unsupported tool syntax. Local requests now declare that file access, browsing
  and tools are unavailable, and retries add a text-only correction. After full
  tests, race checks, rebuild and isolated daemon restart,
  [the exact-question check](results/task1/offline-question-check.json) asked
  “What is the last released Kotlin version?” from `week5/`, with repository
  instructions loaded. It completed with a text answer explaining that current
  releases cannot be verified offline. The original failed session is preserved
  and can be resumed and retried.

The [pre-fix direct reproduction](results/task1/tool-call-reproduction.json)
returned a different, longer text response, showing the model's behavior varies
between runs. That response includes unverified release claims and is retained
only as diagnostic evidence. Capability instructions reduce unsupported tool
attempts; they do not make a small model's claims reliable or give it live data.
The existing guard continues rejecting unsupported tool syntax.
