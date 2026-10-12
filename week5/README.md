# Week5 — Local LLM, agent and RAG

## Task0 — Local LLM launch

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
and the old virtual environment are not needed for Task1. Task2 below adds fully local grounded RAG to `advent-agent`. The other carried
executables, including `mini-chat`, retain their inherited cloud behavior.
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
not added to accepted conversation history. Task-lifecycle sessions are rejected before generation; use `--chat` for
interactive local conversations. Task2 below enables local retrieval/scoring and
grounded answers. Autonomous tool calling remains unsupported.

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


## Task2 — Fully local grounded RAG

`advent-agent --profile local-rag --rag` reuses Week4's full pipeline: local
Ollama embeddings, SQLite cosine retrieval, model relevance scoring, filtering,
and grounded answer generation. Both scoring and answering use the installed
Spark model. Requests run sequentially; scoring uses batches of two candidates
and a 256-token output limit. Scoring requests use short IDs mapped back to
immutable chunk IDs, avoiding long-hash copying errors; raw mappings are saved.
Local scoring and grounded answers use Ollama JSON schemas and retain strict
application validation. Sessions requiring the project-invariant outer envelope
use JSON mode and the existing envelope validator. No cloud credentials or fallback are used.

Retrieval keeps Week4's semantic chunks, 20 candidates, minimum relevance score
2/3, five final chunks and 6,000-character whole-chunk budget. Rewriting and
reranking remain opt-in and use the same local model. Empty filtered evidence
returns an explicit unknown response without answer generation. Generated
answers must pass the existing citation and exact-quote validator. This proves
provenance, not semantic correctness. Malformed scores, invalid answers and
truncation are retained as failures, never replaced with unfiltered retrieval.

### Setup and usage

The Week4 Python environment, its built index, `embeddinggemma:300m`, and Task0's
Spark model must already be installed. The installer copies and verifies the
index into ignored `week5/data/`, so query-cache writes do not alter Week4 data.
A Week5 wrapper invokes Week4's Python indexer; keep the two week directories
together. It rejects HTTP redirects and remote embedding models. Ollama 0.40 can
add `runner`/`format` fields and alternate-runner manifest metadata: the wrapper
accepts this only when removing that packaging metadata reproduces the index's
exact old manifest hash. All inference layers and configuration stay checked.
Changed weights or parameters still fail compatibility checks.
The isolated profile does not change Task1's `local` profile or Week4's service.

From `week5/`, test and rebuild before running these commands:

```sh
make test
make build
export XDG_CONFIG_HOME="$PWD/data/task2-config"
export XDG_STATE_HOME="$PWD/data/task2-state"
/opt/homebrew/bin/python3.14 scripts/install-local-rag.py --context 16384
./advent-agent --profile local-rag --rag -p 'When did the common Kotlin UUID API become stable?' --stats
./advent-agent --profile local-rag --rag --chat
./advent-agent --profile local-rag resume SESSION_ID
./advent-agentd --profile local-rag stop
```

These commands use the freshly built
`/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent` and its sibling
`advent-agentd`. The installer respects XDG paths and refuses to replace different
existing configuration. Without the exports, it installs in the usual user
configuration directory. The default context is 16,384, selected after the pilot showed the inherited
evidence budget can exceed the conservative 8,192-token guard. `--context 8192`
remains available for smaller inputs. Existing profiles require a deliberate
edit and daemon restart.
Evidence and grounding instructions count toward the conservative input guard.
Older eligible exchanges can be summarized locally; oversized current input
fails explicitly instead of silently dropping sources or instructions.

A rejected generated answer is a failed operation, not an unknown response.
Resume the printed session and use `/retry` to regenerate with saved evidence,
or `/discard` to abandon it. The measured comparison deliberately does not retry
failed trials.

### Repeated comparison

After a successful test/build, run the frozen ten-question Week4 suite:

```sh
./advent-agent --profile local-rag rag compare --matrix repeated --runs 3 --suite ../week4/rag-grounding-suite.json --output results/task2/answers.json
/opt/homebrew/bin/python3.14 scripts/render-local-rag.py
```

Each pass independently repeats retrieval, scoring and answering. The evaluator
saves every completed trial, continues after errors, and exits unsuccessfully
if any trial failed. Rerunning resumes unfinished work without retrying terminal
failures. Use one evaluator per output file. Changed suite, index, model, context or settings require a new output
path. A process interruption can repeat an in-flight request whose response was
not checkpointed. Expected facts and review criteria never enter model prompts.

For a macOS capture with external network connections denied, use
`scripts/capture-local-rag.py` instead of the direct comparison command. It uses
the workspace XDG paths above, verifies external-network denial, permits loopback,
and saves model, binary, memory and swap metadata beside the answers. It refuses
to overwrite an environment capture; resume directly with the CLI if needed.

Quality reviews are authored separately against saved evidence and bound to
complete trial hashes. Missing/stale reviews withhold semantic totals. Speed is
split into retrieval, scoring and answer generation. Stability records failures
and variation over three passes. The historical DeepSeek baseline is
`../week4/reports/grounding/`; no new cloud calls are made. Different relevance
scores can select different evidence, so the comparison measures pipelines,
not generation models in isolation. Cloud repeated-run stability is unavailable.


### Development checks and recording

Development pilots are retained separately as `results/task2/pilot-8192.json`
and `results/task2/pilot-16384.json`. They exposed long source-ID copying errors,
missing response fields, and Ollama's manifest metadata migration. They are not
part of the final three-pass measurements. The local provider now constrains
answer source IDs to actual evidence through its JSON schema; exact quote and
citation checks still reject unsupported output.

To capture the CLI workflow after tests/build and workspace profile setup:

```sh
/opt/homebrew/bin/python3.14 scripts/demo-local-rag.py
```

The script uses a temporary project, saves CLI output and session snapshots,
asks an answerable and an unsupported question, stops only the `local-rag`
daemon, then resumes the saved session after automatic daemon restart. It
refuses to overwrite a capture. For a video, show tests/build, the model settings,
one grounded answer with sources/quotes, one unknown response, session resume,
and the comparison report. Video recording/upload remain with the user.


For the stronger offline check, `scripts/offline-local-rag.py` starts an isolated
Ollama server on loopback port 11435 and runs the CLI restart/resume demonstration
with both server and clients denied external network access. It uses its own
workspace profile/state directories and stops its server afterward. Existing
models must already be installed. Avoid running it concurrently with the full
benchmark on an 8 GB machine; a second Ollama server can duplicate loaded weights.
The retrieval wrapper honors `OLLAMA_HOST` for this alternate loopback port.
The offline harness pins the original embedding manifest in an isolated model
store (sharing installed blobs with pruning disabled), because Ollama 0.40.3
otherwise selects different weights under the same tag. Initial compatibility
failures are retained in the offline diagnostic captures.


### Task2 observed comparison

The three-pass run completed on **October 12, 2026**, on the **MacBook Air M1,
8 GB RAM**, using Spark Q4_K_M and a 16,384-token context. All 30 trials and their
raw evidence/requests are in [answers.json](results/task2/answers.json); the
hash-bound manual review and full comparison are in
[comparison.md](results/task2/comparison.md). No new DeepSeek calls were made.

| Measurement | Local pipeline | Historical DeepSeek pipeline |
| --- | ---: | ---: |
| Valid saved responses | 18/30 first attempts | 10/10 after a recorded TLS retry |
| Factual answers fully supported by displayed quotes | 6/12 accepted factual answers | 4/7 |
| Correct unknown controls | 6/9 | 3/3 |
| Median retrieval | 0.48 s | 0.21 s |
| Median scoring | 106.30 s | 5.70 s |
| Median answer generation (excluding skips) | 63.85 s | 2.45 s |
| Median local trial / cloud recorded-stage sum | 148.70 s | 7.85 s |

Cloud stage sums exclude unrecorded orchestration time and are not wall-clock
trial measurements. The runs happened on different dates; the cloud report has
one pass, so equivalent repeated-run cloud stability is unavailable. Local
passes returned 6/10, 7/10, and 5/10 valid responses. All 300 scoring calls
completed; there were no scoring-format, retrieval, or timeout failures in the
final run. Answer failures comprised quotation/citation errors, three invalid
unknown responses, and one output-limit truncation. They were never silently
retried or published as valid answers.

The separate source review found 23 fully covered, six partially covered, and
seven omitted/incorrect expected facts across the accepted factual answers.
Rejected answers receive no delivered-answer credit. In particular, the model
repeatedly confused importing JavaScript APIs with exporting Kotlin interfaces,
even with a relevant external-declarations passage in the packed context.
Quote provenance alone did not prevent that semantic error. The payroll and
PostgreSQL controls consistently filtered all evidence and skipped generation;
the Kotlin 3.0 control consistently returned an invalid unknown envelope.

The run recorded **324 local calls and 362,966 provider tokens**. Generation API
charges were zero, excluding hardware/electricity. Runtime metadata is retained
in [answers.environment.json](results/task2/answers.environment.json). Ollama's
API reported version 0.40.1 and a loaded Spark allocation of 2,805,999,860 bytes
at the finish. System swap was already 7,244.88 MB at the start and rose to
16,128.94 MB. These system-wide snapshots do not attribute all memory use to RAG,
but they show substantial memory pressure; this was not an idle-machine memory
benchmark. The models used by the task were unloaded afterward.

The full pipeline runs locally on this machine, but its latency, grounding
failures, and observed memory pressure are practical limitations. Two suite
questions were used during protocol-development pilots, so this is an
illustrative evaluation, not a held-out benchmark.

### Final offline verification

`make test` (including Go race checks) and `make build` passed. The rebuilt
artifact used was `/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent`.
[The isolated-server capture](results/task2/offline-demo-pinned.json) and
[environment record](results/task2/offline-demo-pinned.environment.json) show
Ollama 0.40.3 and the client running with external networking denied and no
cloud key. A grounded UUID answer passed exact-quote validation; the unsupported
payroll question returned unknown. The saved session retained local RAG settings
after daemon stop and automatic restart, but its next answer was rejected for
a missing citation/quote. The demo therefore exited 1; this is a recorded model
validation failure, not a fully passing three-turn demo. The owned daemon and
isolated server were stopped afterward. Initial embedding compatibility failures
are preserved in the other offline captures; no index or validator was weakened.

## Task3 — Local relevance scorer optimization

Task3 keeps the installed **Spark Q4_K_M** weights fixed and tunes only the
local relevance scorer. Quantization is deliberately excluded: the existing
quantized model is already required by this machine's memory budget. Answer
settings, embeddings, twenty retrieved candidates, the 2/3 relevance threshold,
five-chunk limit, and 6,000-character evidence budget stay fixed.

Optional `[rag.scorer]` settings configure `temperature`,
`context_window_tokens`, `output_tokens`, `batch_size`, and `prompt_version`.
Omitted fields retain Task2 behavior: temperature 1, the agent's context window,
256 output tokens, batches of two, and `legacy-v2`. Explicit temperature zero
is supported. `compact-v1` uses a shorter evidence-classification prompt and
removes retrieval bookkeeping from candidate inputs while retaining complete
text, titles, sections, platforms, release versions, and availability. It also
replaces the general chat capability preamble with scorer-specific instructions.
`compact-v2` additionally returns an ordered score array, with exact array length
constrained by the schema. The application maps positions back to immutable IDs
and rejects missing, extra, null, or out-of-range scores. These settings do not
affect answers, summaries, or query rewriting.

Set `rerank = true` inside `[rag]` to enable score sorting by default for a
profile. Filtering and sorting reuse the same scores; sorting itself makes no
additional model calls. Existing profiles continue using their old defaults.
Scoring keeps schema-constrained JSON, short-ID mapping, exact candidate
coverage, and strict score validation. Oversized inputs and truncated outputs
fail explicitly. Changed scorer settings invalidate saved preparations rather
than mixing scores from different experiments.

### Bounded evaluation

The original three-pass, ten-question comparison was stopped to avoid hours of
runtime. The default `all` workflow now ends with one bounded scoring pass;
expensive repeated `final` and `answers` stages require explicit selection.
The selected candidate uses temperature 0, `compact-v2`, batch size 4, an
8,192-token scoring context, and 64 output tokens. Screening measured about
2× lower median scoring time on four questions. The bounded validation measured
**84.5 → 34.8 seconds (2.43× faster)**, with all four deadlines met, filtering
recall 90.9% → 100%, and packed-evidence recall unchanged at 81.8%. Filtering
precision fell from 52.6% to 45.8%, and nDCG@5 from 90.7% to 85.2%. This is a
development result, not evidence of equivalent answer quality across the full
suite. Sampled RSS did not improve; see the report for resource measurements.

The bounded pass reuses the controlled screening baseline for q01, q13, q19,
and q23. Each measured question gets **60% of its own recorded baseline time**.
Warmup has a separate 60-second cap. A timeout cancels the request, saves a
terminal failure, and is never retried automatically. Failed trials receive no
delivered relevance credit. The target remains **2× median scoring speedup**,
with no scoring failures and at most a five-percentage-point loss in filtering
or packed-evidence recall; the 0.6 cutoff is a looser rejection threshold.
Saved baseline hashes and budgets prevent incompatible resumes.

From `week5/`, after rebuilding and testing:

```sh
make test
make build
python3 scripts/run-score-bench.py bounded
python3 scripts/render-score-bench.py
```

The runner invokes `/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent`,
creates isolated profiles in `data/task3-config/`, and records measurements in
`results/task3/`. It removes the cloud key from child environments and never
downloads another model. Both runs unload the model before their recorded
warmup. The cancelled repeated run and uncontrolled-cache pilot are excluded.
Existing captures cannot be resumed with a different binary or command.

Equivalent standalone replay against the saved baseline:

```sh
XDG_CONFIG_HOME="$PWD/data/task3-config" \
XDG_STATE_HOME="$PWD/data/task3-state" \
./advent-agent --profile task3-finalist rag score-bench \
  --input results/task3/candidates.json \
  --questions q01,q13,q19,q23 --runs 1 \
  --baseline results/task3/screen-baseline-1.json \
  --timeout-factor 0.6 --warmup-timeout 60s \
  --output results/task3/manual-bounded.json
```

For a new experiment, `prepare-score-bench.py --output-dir PATH` freezes Task2's
first-pass candidates and separate relevance labels. Pass that directory to
`run-score-bench.py screen`, then `extra`, then `bounded`, and to the renderer.
Screening itself runs several variants and takes longer; reuse the recorded
baseline and finalist when only validating the runtime cutoff. Input contains
full candidate passages but no expected answers or labels. The implementing
assistant labelled the examples before tuning; they are not an independent or
held-out test set.

See [the measured comparison](results/task3/comparison.md) for the outcome and
resource samples. Model allocation, process RSS, and system swap are distinct;
five-second samples can miss peaks and swap cannot be attributed solely to the
model. Answer generation settings stay fixed, but selected evidence can change.
Two optional answer spot checks (`run-score-bench.py spot`) cover q01 and q23,
with a 90-second deadline per complete pipeline trial and no retries. They only
run after the bounded scoring gate passes. Their Task2 first-pass answers provide
a historical comparison; this is not a new controlled full-suite experiment.
In the recorded run, q01 finished scoring but answer generation hit the
90-second deadline; q23 correctly returned unknown in 62.3 seconds. The scorer
speed target passed, but end-to-end answer quality and speed remain unproven.
The baseline remains the default unless both speed and reviewed answer-quality
gates pass; `install-fast-rag.py` refuses an unqualified result.
