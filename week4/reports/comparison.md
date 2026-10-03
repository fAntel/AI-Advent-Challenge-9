# Chunking comparison

Corpus: **31 documents**, **83,701 unique prose words**, **167.4 equivalent pages** (500 words/page).

Model: `embeddinggemma:300m`; digest: `85462619ee721b466c5927d109d4cb765861907d5417b9109caebc4e614679f1`.

| Metric | Fixed | Semantic |
| --- | ---: | ---: |
| chunk_count | 481 | 1342 |
| characters_min | 218 | 5 |
| characters_median | 1800.00 | 538.00 |
| characters_p95 | 1800.00 | 1195.60 |
| characters_max | 1800 | 2346 |
| chunks_with_split_code | 183 | 0 |
| vector_bytes | 1477632 | 4122624 |
| seconds | 61.62 | 166.45 |
| embedding_calls | 481 | 6940 |
| hit_at_5 | 0.800 | 0.900 |
| recall_at_5 | 0.800 | 0.900 |
| reciprocal_rank | 0.646 | 0.718 |
| coverage_at_6000_chars | 0.608 | 0.663 |

## Interpretation

The table reports measured results, without assuming either chunker wins. Relevance means overlap with manually selected source sections; it does not establish correctness of generated code. Both strategies use identical query vectors and exact cosine ranking. Coverage uses the first 6,000 retrieved text characters, truncating the final chunk and deduplicating overlapping source spans. This is a character budget, not a model-token budget.

Indexing times include boundary detection, cached embedding lookups and database insertion. Semantic runs after fixed and can reuse identical cached inputs; embedding-call counts describe actual calls in this run, not a controlled cold-cache speed benchmark. The full SQLite file contains both strategies and document snapshots; vector bytes provide the directly comparable per-strategy storage.

## Per-question evidence

| Question | Fixed hit@5 | Semantic hit@5 |
| --- | ---: | ---: |
| How can a function require an implicit service and how is that service resolved? | 1 | 1 |
| What is the Elvis operator and how can I return early when a nullable value is absent? | 1 | 1 |
| Which operations on Kotlin sequences are lazy, and which ones trigger processing? | 1 | 1 |
| Which scope function should I use to configure an object and return that same object? | 0 | 1 |
| How can I perform a side effect in a call chain while retaining the original object? | 0 | 0 |
| How do I combine two collections into pairs? | 1 | 1 |
| How can an inline generic function access its type argument at runtime? | 1 | 1 |
| How do in and out make a generic Kotlin interface safely consume or produce values? | 0 | 1 |
| How should Kotlin handle Java references whose nullability is not known? | 1 | 1 |
| How can Java callers use a Kotlin function with default parameters without passing every argument? | 1 | 1 |
| Where should expect and actual declarations live and which parts must match? | 1 | 1 |
| Can I satisfy a common expected class using an existing platform type instead of wrapping it? | 1 | 1 |
| How are Kotlin exceptions exposed to Swift and Objective-C callers? | 1 | 1 |
| How does Swift export expose Kotlin suspend functions? | 1 | 1 |
| How do I make a Kotlin declaration accessible to JavaScript callers? | 1 | 1 |
| How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation? | 0 | 0 |
| How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side? | 1 | 1 |
| When did the common Kotlin UUID API become stable? | 1 | 1 |
| What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript? | 1 | 1 |
| Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types? | 1 | 1 |
| What is the internal production API endpoint for my company payroll service? | unanswerable | unanswerable |
| What breaking changes will Kotlin 3.0 ship with? | unanswerable | unanswerable |
| How do I configure a PostgreSQL replication slot for Debezium? | unanswerable | unanswerable |

## Representative results

### How can I perform a side effect in a call chain while retaining the original object?

- **fixed** first result: [java-to-kotlin-interop](https://kotlinlang.org/docs/java-to-kotlin-interop.html), lines 547–559; score 0.416. Calling Kotlin from Java > Inline value classes > Inherited functions
- **semantic** first result: [null-safety](https://kotlinlang.org/docs/null-safety.html), lines 179–189; score 0.519. Null safety > Safe call operator

### How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

- **fixed** first result: [wasm-js-interop](https://kotlinlang.org/docs/wasm-js-interop.html), lines 121–168; score 0.653. Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External type hierarchy / Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke / Interoperability with JavaScript > Use JavaScript code in Kotlin > Kotlin functions with JavaScript code
- **semantic** first result: [wasm-js-interop](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; score 0.694. Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations

### Which scope function should I use to configure an object and return that same object?

- **fixed** first result: [scope-functions](https://kotlinlang.org/docs/scope-functions.html), lines 42–80; score 0.587. Scope functions / Scope functions > Function selection / Scope functions > Distinctions
- **semantic** first result: [scope-functions](https://kotlinlang.org/docs/scope-functions.html), lines 170–178; score 0.608. Scope functions > Distinctions > Return value

### How can a function require an implicit service and how is that service resolved?

- **fixed** first result: [context-parameters](https://kotlinlang.org/docs/context-parameters.html), lines 38–90; score 0.409. Context parameters / Context parameters > Context parameters resolution
- **semantic** first result: [context-parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; score 0.430. Context parameters

### What is the internal production API endpoint for my company payroll service?

- **fixed** first result: [inline-functions](https://kotlinlang.org/docs/inline-functions.html), lines 165–175; score 0.259. Inline functions > Inline properties / Inline functions > Restrictions for public API inline functions
- **semantic** first result: [inline-functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; score 0.275. Inline functions > Restrictions for public API inline functions

### What breaking changes will Kotlin 3.0 ship with?

- **fixed** first result: [whatsnew23](https://kotlinlang.org/docs/whatsnew23.html), lines 757–797; score 0.625. What's new in Kotlin 2.3.0 > Breaking changes and deprecations / What's new in Kotlin 2.3.0 > Documentation updates / What's new in Kotlin 2.3.0 > How to update to Kotlin 2.3.0
- **semantic** first result: [whatsnew23](https://kotlinlang.org/docs/whatsnew23.html), lines 1–19; score 0.636. What's new in Kotlin 2.3.0

### How do I configure a PostgreSQL replication slot for Debezium?

- **fixed** first result: [compatibility-guide-23](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 293–325; score 0.203. Compatibility guide for Kotlin 2.3.x > Tools > Unsupported KGP version warning when using kotlin-dsl and kotlin("jvm") plugins / Compatibility guide for Kotlin 2.3.x > Tools > Deprecate kotlin-android plugin for AGP versions 9.0.0 and later / Compatibility guide for Kotlin 2.3.x > Tools > Deprecate testApi configuration
- **semantic** first result: [whatsnew21](https://kotlinlang.org/docs/whatsnew21.html), lines 824–840; score 0.237. What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings

Out-of-corpus questions still produce nearest neighbors. No answerability threshold or refusal capability is claimed. Inspect the JSON report for all top-five results and scores.

## Findings for this run

Semantic found the designated section in 18/20 questions versus fixed's 16/20,
with greater mean source-span coverage (66.3% versus 60.8%) and no split code
blocks. It required roughly 2.7 times the indexing time and 2.8 times as many
stored chunk vectors. It remains an imperfect splitter: the shortest chunk is
only a five-character heading. These results support using semantic as the
initial search default for this corpus, not a general claim that it always wins.

The labels designate specific sections rather than exhaustively labeling every
possible answer. For example, the JavaScript external-declaration question
retrieves related Wasm documentation but misses the designated Kotlin/JS
section. Both strategies also miss the designated `also` section for the
side-effect question. Review these cases before interpreting every metric miss
as irrelevant retrieval. The Kotlin 3.0 question returns older release notes
with substantial similarity, demonstrating why similarity alone cannot establish
that an answer is supported.
