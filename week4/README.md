# RAG agent and Kotlin documentation index — Week 4

A local document index for helping a coding agent find current Kotlin guidance,
examples and version history. Task0 builds and compares indexes. Task1 carries the full week 3 Go harness
forward and adds optional retrieval and a source-reviewed answer comparison.

The CLI uses Python, SQLite, exact cosine retrieval and local Ollama embeddings.
The corpus covers common Kotlin, JVM/Android interoperability, Native/Swift,
JavaScript and Wasm. Release and compatibility notes start at Kotlin 2.1.0;
the corpus baseline is Kotlin 2.4.20. Current guide pages are dated snapshots,
not frozen documentation for every historical release.

## Setup

Python 3.11+ is required. The development machine uses Python 3.14 installed as
an Ollama dependency. No change to macOS system Python is needed.

```sh
brew install ollama
/opt/homebrew/bin/python3.14 -m venv .venv
.venv/bin/python -m pip install setuptools==80.9.0 wheel==0.45.1
.venv/bin/python -m pip install .
```

Start Ollama as a background service on macOS and download the local model:

```sh
brew services start ollama
ollama pull embeddinggemma:300m
```

This model is approximately 622 MB. The CLI defaults to
`http://127.0.0.1:11434`; it uses no hosted embedding or generation API.
Downloads require internet access; building saved snapshots and evaluating
queries only require the local model. No login or API key is required.
The Homebrew service restarts after login. Alternatively, run `ollama serve`
in a separate terminal and keep it open while indexing or searching. Stop the
background service with `brew services stop ollama`. A connection-refused error
means the configured Ollama server is not listening; check `brew services list`
and start it again.

## Build and test

From `week4/`:

```sh
make test
make build
```

`make build` installs a fresh wheel and builds all seven Go executables, then checks CLI
help. `make test` runs Python tests plus normal and race-enabled Go tests. All demonstrations use the rebuilt artifact
`week4/.venv/bin/kotlin-index`. Tests use a fake embedding service and require
neither internet access nor Ollama.

## Reproduce the result

After tests and build pass, run these commands from `week4/`:

```sh
.venv/bin/kotlin-index fetch
.venv/bin/kotlin-index build --strategy both
.venv/bin/kotlin-index inspect
.venv/bin/kotlin-index compare
.venv/bin/kotlin-index search 'How do I expose a Kotlin function to JavaScript?' --platform js --limit 3
```

The explicit fetch downloads only URLs in `corpus.json`. Build never refreshes
sources implicitly. Outputs are:

- `data/sources/`: original HTML, normalized text and structured snapshots.
- `data/index.sqlite`: completed index containing both chunking strategies,
  document snapshots, metadata and normalized float32 embeddings.
- `data/embeddings.sqlite`: reusable embedding cache.
- `corpus-checksums.json`: source URLs, retrieval timestamps and content hashes.
- `reports/comparison.md` and `reports/comparison.json`: measured comparison
  and the full per-question results.

Downloads, databases and the virtual environment are ignored by Git. The
manifest, checksums, evaluation questions and comparison report are retained.
A later fetch can differ from the recorded corpus because official pages change;
retain `data/sources/` to reproduce an exact snapshot. These source materials
belong to their respective authors; all documents carry their official URLs.
Upstream source repositories:

- [Kotlin website](https://github.com/JetBrains/kotlin-web-site): language guides
  in `docs/topics`, release notes in `docs/topics/whatsnew`, and shared version
  variables in `docs/v.list`.
- [Kotlin Multiplatform documentation](https://github.com/JetBrains/kotlin-multiplatform-dev-docs):
  the expected/actual guide and other multiplatform documentation.

This task uses rendered HTML deliberately: JetBrains' build has already resolved
Markdown extensions, tabs, includes and version variables. The extractor keeps
article content and handles WebHelp `div.code-block` examples, including those
inside lists. Saved HTML plus checksums provides provenance without reproducing
the website build. Consult each upstream repository for its source license.

Global options precede the subcommand:

```sh
.venv/bin/kotlin-index --db data/index.sqlite search 'Use context parameters' --strategy fixed --json
```

`--data` changes the snapshot/cache directory; `--db` independently chooses the
index file. Search returns evidence, source lines and scores, not an answer.
`--platform` is a coarse document filter that also includes common-language
documents; broad release notes contain multiple platforms and still require
reading the returned section. Version compatibility is not inferred from a
release-note title. No older-version safety guarantee is provided.

## Chunking and metadata

Fixed chunks contain 1,800 Unicode characters with 200-character overlap.
Semantic chunks use section boundaries and adjacent-block embedding similarity,
with a 0.65 cosine threshold, a 600-character minimum before topic splits and
a 2,400-character maximum. Headings and maximum size may create smaller chunks.
Code stays with preceding explanations when it fits; oversized code blocks split
at line boundaries, falling back to character boundaries for very long lines.
Chunks intersecting partial code blocks carry `split_code=true`.

Sizes are **characters, not tokens**. Ollama truncation is disabled: if inputs
exceed the model context, reduce chunk sizes and rebuild. Model digest,
dimensions, input formatting and settings are recorded. The embedding cache is
keyed by exact formatted text and model identity. Queries reject model mismatch.
Changing the model or chunk settings requires rebuilding the index.

SQLite stores document provenance separately from chunks. Join `chunks.document_id`
to `documents.id` for source/title/platform/version metadata. Chunk JSON contains
its ID, strategy, text, section paths, anchor, character offsets, line range and
code-split flag. Line numbers refer to normalized `.txt` snapshots, not HTML.
Release version and feature availability are distinct; unsupported availability
inferences remain `unknown`. Feature status and restrictions remain in the
retrieved text.

The index is replaced only after all requested strategies finish successfully.
Interrupted builds preserve the previous completed index and the embedding
cache; rerunning rebuilds deterministically while reusing cached vectors.
Run only one build per database at a time.

## Evaluation

`evaluation.json` contains 20 answerable questions with manually inspected
source-section labels and three out-of-corpus questions. Both strategies use
the same query embeddings and exact cosine ranking. Metrics include section
hit/recall at five, reciprocal rank and deduplicated evidence coverage under a
6,000-character retrieval budget. Section overlap is a retrieval proxy; a
partial overlap can count as a hit even if it does not answer the whole question.

Corpus size counts unique non-code blocks using a word-pattern tokenizer;
500 words equal one estimated page. Build requires 15,000 words, equivalent to
30 pages. This is an explicit homework convention, not PDF pagination.
The report includes code volume, chunk lengths, split code, time, calls and storage.

Semantic indexing includes additional block embeddings. It runs after fixed
indexing and may reuse exact cached inputs; timings are actual pipeline timings,
not controlled cold-cache benchmarks. Unknown questions still return neighbors:
there is no calibrated answerability threshold. Evaluation does not establish
code-generation correctness; compiler and test validation belongs in the later
harness task.

## Project constraints

The harness retains `AGENTS.md` loading and project invariants. Automatic discovery
of consumer project Kotlin versions remains outside this task. Instructions
such as “do not change Kotlin unless asked” belong to the consumer, not the
corpus. The Go harness consumes the diagnostic JSON search through a direct subprocess;
no retrieval MCP server is needed.

## Recorded result

The first completed local run indexed 31 documents: 83,701 unique prose words
(167.4 equivalent pages) and 158,940 code characters, using 768-dimensional
EmbeddingGemma vectors. The SQLite index is approximately 13 MiB.

| Measure | Fixed | Semantic |
| --- | ---: | ---: |
| Chunks | 481 | 1,342 |
| Designated source section found in top five | 16/20 | 18/20 |
| Mean evidence coverage within 6,000 characters | 60.8% | 66.3% |
| Chunks containing split code blocks | 183 | 0 |
| Indexing seconds | 61.6 | 166.5 |

Semantic chunking is the default diagnostic search strategy for this corpus.
Its tradeoffs are more stored vectors and many additional boundary embeddings.
Some semantic chunks contain only short headings (the minimum observed size is
five characters), so section coherence is not guaranteed by semantic splitting.
The twenty questions are a small, manually labeled benchmark, not a broad
quality claim. Labels identify designated sections and are not an exhaustive
catalog of equivalent answers elsewhere in the corpus. In particular, shared
JavaScript/Wasm concepts can retrieve useful evidence from a different page.
See [the comparison](reports/comparison.md) and its JSON counterpart for all
results, including failures and out-of-corpus nearest neighbors.


## Task1: optional retrieval

See [HARNESS.md](HARNESS.md) for all inherited chat, task lifecycle, memory,
invariant, MCP, pipeline, scheduling and service features. Configuration,
credentials, sessions, memory, catalog and socket retain the week 3 locations.
Old sessions remain plain. Ordinary provider defaults remain unchanged.

After `make test` and `make build`, these commands use the newly built
`week4/advent-agent` and `week4/.venv/bin/kotlin-index`:

```sh
./advent-agent -p 'What is the Elvis operator?'
./advent-agent --rag -p 'What is the Elvis operator?'
./advent-agent --rag --rag-strategy fixed --rag-platform js -p 'How do I export a Kotlin declaration?'
./advent-agent resume SESSION_ID --rag
```

Plain mode is the default and never invokes the indexer or Ollama. `--rag=false`
disables retrieval on resume. `--rag-strategy` defaults to `semantic`,
`--rag-limit` to 5, and `--rag-budget` to 6000 Unicode characters, including
reference headers. `--rag-platform` defaults to no filter. These settings
persist through resume and branches. `--strategy` still controls conversation
history. The original task plus latest feedback supplies the task-mode query;
chat uses the current question. Compression never retrieves.

Whole chunks are packed in relevance order, skipping chunks that do not fit.
Evidence includes source titles, sections, URLs, normalized line ranges and
version caveats. It is untrusted reference data subordinate to instructions and
invariants. RAG answers now require validated source references and verbatim quotes. Weak
context produces a harness-owned unknown response (see the grounding section below).
Retrieval is not a guarantee of correctness or version compatibility.
A compact retrieval summary and references appear after answers.

Evidence is saved with each operation before provider calls and reused by
retries and MCP continuations. Retrieval failures stop the operation with a
diagnostic; fix the index/model/service and retry. A successfully saved evidence
snapshot survives later index changes. New operations retrieve afresh.

Default paths resolve beside the daemon regardless of its working directory:
`.venv/bin/kotlin-index` and `data/index.sqlite`. Optional `[rag]` configuration
keys `executable` and `database` must be absolute. RAG clients detect old daemons
and request a rebuild/restart instead of silently sending plain requests.

To upgrade the installed macOS service after tests and build:

```sh
./advent-agent service install
./advent-agent service status
```

Installation points the existing LaunchAgent at `week4/advent-agentd` and restarts
it without removing state. Existing explicitly configured MCP executable paths
remain valid; sibling fallback paths now resolve beside week 4.

## Task1: paired answer comparison

```sh
./advent-agent rag compare --suite rag-suite.json --output reports/task1/answers.json --model deepseek-v4-pro --reasoning low --rag-strategy semantic
.venv/bin/python scripts/render_rag_review.py
```

The comparison runs directly through the same provider adapter, retrieval and
evidence prompt assembly as the daemon. It deliberately excludes sessions,
history, memory, AGENTS.md, lifecycle and MCP tools. Both arms receive the same
base instructions and explicit model settings, with no temperature override.
Mode order alternates by question. Expected facts in the suite never enter
provider messages. Completed answers are atomically saved and skipped on resume;
changed suite, settings or index fingerprints require a new output path.

`reports/task1/answers.json` saves exact provider messages, evidence, settings,
index fingerprint, answer, usage, latency and estimated generation cost. Embedding
latency/cost is not included. The ten hardest answerable questions were selected by reviewer judgment for
version sensitivity, interop edge cases and Task0 retrieval difficulty; this is
not a measured difficulty ranking. `rag-suite.json` records pre-generation expected
facts and source sections. `reports/task1/sources/` retains review excerpts with
original normalized line numbers; Task0 source snapshots and checksums remain
unchanged. `reports/task1/review.json` contains the assistant's separate factual
and citation assessments, bound to hashes of each complete answer record. The
renderer never generates judgments: missing or stale reviews show UNREVIEWED.
Use a new output path for regeneration, then review new answers explicitly.

This single twenty-answer comparison is illustrative, not a broad reliability
claim. The reference standard is the saved corpus, including its snapshot and
version limitations. See [the reviewed comparison](reports/task1/comparison.md).

### Pro versus Flash

The same ten questions were also run with `deepseek-v4-flash`, low reasoning,
using exactly the same provider messages and retrieved evidence as Pro:

```sh
./advent-agent rag compare --suite rag-suite.json --output reports/task1/flash/answers.json --model deepseek-v4-flash --reasoning low --rag-strategy semantic
.venv/bin/python scripts/render_rag_review.py --answers reports/task1/flash/answers.json --review reports/task1/flash/review.json --output reports/task1/flash/comparison.md
```

See [the Pro/Flash comparison](reports/task1/model-comparison.md) for the
four-condition results and my question-by-question assessment, and
[the Flash report](reports/task1/flash/comparison.md) for all twenty new answers
and their independent factual/citation reviews. The original Pro artifacts are
preserved. The report compares this single run, not general model reliability.


## Earlier experiment: rewriting, reranking and relevance filtering

This section records the earlier opt-in experiment. Current RAG always scores and
filters evidence, and requires quotes; the grounding section below supersedes the
old filtering defaults. Saved reports retain their original eight-mode behavior.

The agent supports three independent opt-in enhancements. `--rag-rewrite` asks
Flash to rewrite only the search query. `--rag-rerank` sorts candidates using
Flash relevance scores. `--rag-filter` removes candidates scoring below
`--rag-threshold` (default 2). Auxiliary calls always use `deepseek-v4-flash`,
reasoning `none`, no tools and no temperature override, even if the answering
model differs. All switches default off and persist through resume and branches.
`--rag=false` skips retrieval and all auxiliary calls.

The scoring rubric is 0 = unrelated/incompatible, 1 = topic only without answer
support, 2 = useful partial support, 3 = direct support. These are ordinal model
judgments, not probabilities. The filter accepts scores greater than or equal
to the threshold (integer 0–3). Filtering alone preserves cosine order;
reranking preserves cosine order for tied relevance scores. Both together sort
and discard low scores. Scoring always targets the original question, including
when search uses a rewrite. The answer always receives the original question.

`--rag-candidates` retrieves 20 candidates by default; `--rag-limit` selects at
most 5 after processing; the existing 6000-character whole-chunk budget is then
applied without backfilling beyond the final limit. Candidates must be at least
the final limit, and both must be in 1–100. Legacy settings with no candidate
count normalize to at least 20 or their existing final limit. Zero surviving
chunks are valid and instruct the answer to acknowledge insufficient corpus
evidence. Plain diagnostic `kotlin-index search` continues to report cosine
neighbors without hosted inference.

After `make test` and `make build`, these examples use `week4/advent-agent` and
`week4/.venv/bin/kotlin-index`:

```sh
./advent-agent --rag --rag-rewrite --rag-rerank --rag-filter --rag-candidates 20 --rag-limit 5 --rag-threshold 2 -p 'How can browser code call my Kotlin function?'
./advent-agent rag compare --matrix enhanced --suite rag-task3-suite.json --output reports/task3/answers.json --model deepseek-v4-flash --reasoning none --rag-candidates 20 --rag-limit 5 --rag-threshold 2
.venv/bin/python scripts/render_rag_matrix.py
```

The comparison defaults to Flash with reasoning disabled. `--matrix paired`
retains the original plain-versus-RAG experiment; `--matrix enhanced` runs the
four combinations of reranking/filtering both with and without rewriting,
regardless of individual enhancement flags. The Task3 suite has ten unchanged
Task1 questions and three out-of-corpus controls: 104 answers. Expected facts and
review criteria never enter provider prompts. The rubric, cutoff and K values
are frozen before this run; the report includes regressions rather than tuning
settings against the answers.

Preparation saves rewrites, all candidates, cosine/relevance scores, raw
auxiliary requests/responses, decisions and metrics. Scoring batches contain at
most five chunks and require a valid score for every ID; invalid responses stop
the operation explicitly. Completed preparation stages survive interruption;
saved final evidence is reused for answer retries and MCP continuations.
Auxiliary calls appear in session/operation usage. Old daemons must be rebuilt
and restarted before using the new client with RAG.

Within the matrix, one rewrite and one candidate pool per query condition are
shared across modes, as are applicable scores. Preparation and answers are
checkpointed atomically. Resume skips completed work and rejects changed suite,
index, settings or preparation version. Mode order rotates. Reports distinguish
actual shared-call costs from each mode's full independent preparation cost;
these independent costs must not be summed as experiment expenditure. Retrieval
latency includes local subprocess and embedding work. Answer reviews are authored
separately against saved sources and bound to complete answer hashes. Missing or
stale reviews withhold quality summaries. This is one illustrative experiment,
not a statistically established improvement or code-correctness guarantee.

See [the Task3 comparison](reports/task3/comparison.md) for measured results,
individual answers, source references and reviews. Prior Task1 artifacts remain
unchanged. Use a new output path to regenerate rather than overwrite that run.


### Task3 recorded result

The completed Flash/no-reasoning run contains 104 independently reviewed answers.
Expected-fact coverage was 27/30 for baseline RAG, 27/30 with reranking, 25/30
with filtering, and 23/30 with both. With rewriting enabled, the corresponding
figures were 23/30, 26/30, 28/30 and 28/30. These counts exclude partial facts;
some omitted facts are useful caveats beyond the literal short question.

Filtering removed all evidence for all three out-of-corpus controls. All eight
modes already acknowledged corpus insufficiency on these controls, and several
still volunteered erroneous general-knowledge advice. Rewriting improved packed
designated-section hits from 9/10 to 10/10, but section presence did not guarantee
complete answers. The report records citation failures, omissions and a clearly
separated post-generation external check of PostgreSQL claims.

The actual experiment used 221 hosted calls and 355,481 tokens, with estimated
cost $0.050258 using the repository adapter's pricing. Scoring added approximately
5.6–5.8 seconds per question; rewriting added 0.7 seconds. Enhancements remain
opt-in: rewriting plus filtering was promising for this run, not an established
universal improvement. Raw requests, responses, evidence and scores are retained
in `reports/task3/answers.json`; hash-bound reviews are in `review.json`.


## Task3: quotes, sources and unknown responses

`--rag` now enables the full grounding contract without additional flags. Every
factual answer includes an Answer, Sources and Quotes section. Sources come from
saved chunk metadata: URL, title, section and chunk ID. Quotes are model-selected,
verbatim substrings of packed chunks. The harness rejects malformed output,
unknown citations, missing quotes and altered fragments before displaying an answer.
Exact matching proves provenance; it does not prove that every claim follows from
its quote. The separate ten-question review checks meaning against the quotes.

All RAG retrievals use Flash relevance scoring (0–3) and discard scores below
`--rag-threshold` (default 2). The existing 0–3 threshold range is retained; setting
0 deliberately accepts every scored candidate. Scoring adds hosted calls in batches
of up to five candidates, even without reranking. Rewriting and reranking remain
opt-in. `--rag-filter` is deprecated: either value leaves filtering enabled.
`--rag=false` still skips all retrieval and auxiliary calls.

If no chunks survive filtering, selection and the character budget, the harness
returns “I don't know based on the available sources” and asks for the Kotlin
feature, platform, version or documentation. It does not call the answering model
or execute MCP tools. Sources and quotes are explicitly unavailable. With some
support, the model must answer only supported parts and clarify missing coverage;
it may also return unknown. No background-knowledge supplementation is allowed.
Unknown task responses block the current phase until feedback supplies context;
`/continue` cannot advance them. Existing project-invariant refusals take precedence
when an answer is generated. Invalid generated output is a retryable error, not an
unknown response.

Operations retain the raw response and validated structured result, alongside
saved evidence and scoring calls. Answer retries reuse that evidence. Old completed
sessions remain readable; incompatible pending evidence must be discarded and
resubmitted. New clients require the daemon's `rag_grounded` capability: rebuild
and restart the service to upgrade it. Old comparison checkpoints require a new
output path. Historical Task1 and earlier eight-mode reports are preserved.
New enhanced comparisons have four distinct rewrite/rerank combinations, all filtered.

After `make test` and `make build`, from `week4/`, use the rebuilt
`week4/advent-agent` and `week4/.venv/bin/kotlin-index`:

```sh
./advent-agent --rag -p 'When did the common Kotlin UUID API become stable?'
./advent-agent --rag -p 'What breaking changes will Kotlin 3.0 ship with?'
./advent-agent rag compare --matrix grounded --suite rag-grounding-suite.json --output reports/grounding/answers.json
.venv/bin/python scripts/render_rag_grounding.py
```

The grounding suite contains seven existing answerable questions and all three
out-of-corpus controls. The single-condition comparison defaults to Flash with
reasoning disabled, no rewriting or reranking, cutoff 2, 20 candidates, five final
chunks and a 6000-character budget. Expected facts never enter provider messages.
Raw answers, quotes, metadata, scores and costs are retained. Failed attempts are
saved separately; empty-context responses do not count as generation calls.
Semantic reviews are separately authored in `reports/grounding/review.json` and
bound to complete answer hashes. Missing/stale reviews withhold quality totals.
The renderer checks quote/source provenance without manufacturing semantic judgments.


### Grounding recorded result

The [ten-question review](reports/grounding/comparison.md) produced seven factual
answers, all with source metadata and exact verbatim quotes, and three correct
unknown responses that skipped answer generation. No answerable question was refused.
Four factual answers were fully supported by their displayed quotes. Three received
partial semantic-support assessments: Swift async and JS lambda answers omitted
some supporting details from the displayed quotes despite having them in the full
chunks; the JS interop answer did not clearly identify its Wasm source scope.
These limitations are retained in the report rather than counted as full passes.

The run recorded 48 API attempts (47 completed calls plus one TLS-handshake failure
retried from a checkpoint), 100,031 tokens and estimated generation/scoring cost
$0.006089 using the repository adapter's rates. Three answer calls were skipped.
See `reports/grounding/answers.json` for exact inputs, raw responses and evidence,
and `reports/grounding/review.json` for the separately authored hash-bound judgments.
The corpus is unchanged; these checks assess support against saved documentation,
not an independent verification of current Kotlin behavior.


## Task4: standalone mini-chat

`mini-chat` is a separate Go CLI. It reuses the Kotlin SQLite index, local Ollama
embeddings, DeepSeek provider and grounded RAG contract. It does not start the
agent daemon or load its sessions, task lifecycle, project instructions, MCP tools
or scheduling. It reads the existing default-profile provider endpoint and RAG
path configuration; credentials use `DEEPSEEK_API_KEY` or the existing macOS
Keychain entry. Generation and memory extraction default to Flash with reasoning
disabled. Relevance scoring uses Flash, as in the earlier RAG implementation.

After `make test` and `make build` succeed, these commands use the rebuilt
`week4/mini-chat` (run from `week4/`):

```sh
./mini-chat
./mini-chat sessions
./mini-chat resume SESSION_ID
```

The default conversation database is `data/chat.sqlite` beside the executable,
separate from `data/index.sqlite`. `--db FILE` overrides conversation storage;
`--index FILE` and `--indexer FILE` override retrieval paths. Relative overrides
resolve from the current directory. New chats also accept `--model` and
`--reasoning`; resume uses saved session settings and accepts only `--db`.
The local model and built index from Setup are required for questions.

Start with a natural statement, for example:

> Our goal is to design a Kotlin library for browser JavaScript. Keep Kotlin/JS,
> not Wasm. By client I mean the browser JavaScript consumer. How do I export a function?

Each question runs structured task-memory extraction, resolves its search question,
retrieves and filters fresh evidence for the resolved platform, and validates the generated answer's citations
and verbatim quotes before display. The default retrieval settings are semantic
chunks, 20 candidates, relevance cutoff 2, five selected chunks and a 6,000-character
budget. The extractor resolves the saved target to the index platform filter (for example,
Kotlin/JS to `js` and Swift/iOS to `native`); common-language documents remain
eligible. Insufficient evidence produces an explicit unknown response with a Sources
section explaining that supporting sources are unavailable. A memory-conflict
clarification also has a Sources section, identifying that no documentation claims
were made, even though retrieval ran.

Memory contains the user-stated goal, constraints, definitions and the prior-question
log. Automatic fact updates require an exact quote from the current user turn.
The extractor is instructed to reuse keys and distinguish explicit corrections
from ambiguous contradictions. The harness preserves a conflicting saved value
unless the extracted change identifies an explicit replacement; otherwise it asks
for clarification. Provenance validation proves where text came from, not the
correctness of the model's interpretation. Inspect or correct memory at any time:

```text
/memory
/goal Design a Kotlin browser library
/constraint platform=Kotlin/JS
/definition client=browser JavaScript consumer
/forget constraint platform
/history
/help
/exit
```

Local commands do not call the model or retrieve. Memory changes are displayed
when saved. Only the last four completed exchanges are supplied as recent history;
the goal, fixed facts and all non-discarded prior questions are supplied separately.
The complete transcript remains stored. If a request exceeds 120,000 characters,
the turn fails explicitly instead of silently trimming task memory; start a new
session or shorten facts explicitly. Single questions are limited to 16,000
characters and individual memory values to 2,000 characters.

SQLite stores versioned session records, turn checkpoints, memory revisions, exact
provider requests/responses and usage, retrieved evidence and validated results.
Updates are atomic, with a per-session OS lock during processing. Different
sessions can run independently. A crash releases the lock; resume exposes the
pending turn. `/retry` reuses completed extraction/retrieval stages; `/discard`
restores the pre-turn memory and retains the abandoned turn in the audit history.
New questions and memory edits wait until the pending turn is resolved. A remote
call interrupted before its response is saved may be repeated on retry.

This is a local single-user CLI for macOS/Linux. History and evidence are not
rotated automatically; chat storage grows with use. The memory interpreter and
quote/claim relationship still require semantic review.

### Task4 dialog validation

`mini-chat-suite.json` contains two fixed twelve-question dialogs: Kotlin/JS browser
interop and Kotlin/Swift Objective-C export. Each includes early goals and definitions,
follow-ups beyond the recent-history window, an explicit constraint change, an
ambiguous contradiction, confirmation of the saved constraint, and an unsupported
Kotlin 3.0 question. The final question returns to the original goal.

The deterministic tests use SQLite plus fake provider/index services. They test
retention, restarts, isolation, corrections, conflicts, retry/discard, invalid quotes,
context limits and evaluation checkpoint reuse. They do not establish model quality.
After tests and build, run the live suite with the same `week4/mini-chat` artifact:

```sh
./mini-chat eval --suite mini-chat-suite.json --output reports/task4/answers.json --db data/task4-chat.sqlite
```

The evaluator saves each turn and resumes on rerun. Changed suite or model/retrieval
settings require a new output file. Preserve the conversation database to resume.
Raw artifacts include prompts, memory revisions, evidence, sources, quotes and
provider usage. The independently authored review assesses goal/constraint adherence
and semantic support separately from mechanical quote validation.

Render the saved dialogs and separately authored review with:

```sh
.venv/bin/python scripts/render_mini_chat.py
```

The renderer binds each review to a hash of the complete turn, including memory,
provider calls and evidence. Missing or stale reviews withhold semantic quality totals.

### Task4 recorded result

The full Python and Go tests, including Go race checks and both deterministic
12-question dialogs, pass. `make build` succeeds. Both live twelve-question dialogs
completed; the Swift dialog was resumed and finished interactively in agterm's
scratch terminal using the rebuilt `week4/mini-chat`.

The [reviewed live report](reports/task4/dialogs.md) retains all 24 turns. The
original goal and client definition persisted in every memory snapshot. Both
explicit constraint changes were saved, and both ambiguous contradictions asked
for clarification without replacing the fixed value. All 18 factual answers have
validated sources and exact quotations. Two clarifications and four unknown
responses display explicit source-status explanations.

Semantic review found full goal/constraint adherence in 21/24 responses and full
support from displayed quotes in 9/18 factual answers. Remaining issues include
example-language deviations, an unnecessary refusal despite useful evidence,
Objective-C naming being overgeneralized to Swift, and a final import-direction
limitation being applied to an export task. Several partial quote assessments
have additional support in the full retrieved chunks. These limitations are
recorded individually; source validation alone does not establish semantic
correctness or compilable examples.

The final run used 146 API attempts, 367,700 tokens and estimated provider cost
$0.053284. Three TLS failures and four rejected generated answers were retried
from checkpoints; rejected answers were never published. The earlier
[initial attempt](reports/task4/initial-attempt.md) is preserved with its review
and $0.030530 estimated cost. It exposed platform-mismatched retrieval and a missed
conflict, leading to the platform filter and tighter memory extraction contract.
This small suite was used during development, so it is not a held-out benchmark.
