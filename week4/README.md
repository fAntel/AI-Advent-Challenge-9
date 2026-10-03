# Kotlin documentation index — Week 4

A local document index for helping a coding agent find current Kotlin guidance,
examples and version history. This week's task builds and compares indexes;
it does not generate code or modify a consumer project's Kotlin version.

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

`make build` installs a fresh wheel into the virtual environment and checks CLI
help. All demonstrations use the rebuilt artifact
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

## Later integration

The consuming agent should read `AGENTS.md` and resolve real project constraints,
for example `kotlin` from `gradle/libs.versions.toml`, before retrieval. Instructions
such as “do not change Kotlin unless asked” belong to the consumer, not the
corpus. The diagnostic JSON search output is available for the next task; this
implementation adds no MCP server or week 3 harness changes.

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
