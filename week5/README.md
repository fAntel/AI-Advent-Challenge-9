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

The build produces `/Users/keldzh/Develop/AiAdventChallenge/week5/local-llm` and
checks its help command. The binary and model weights are excluded from Git.

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
