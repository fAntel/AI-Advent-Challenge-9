# Task 3: eight-mode RAG comparison

13 questions × 8 modes. Model `deepseek-v4-flash`, reasoning `none`. Index SHA-256: `8249a981e64ba378fac6690ca46299545fa68f054cfb5ccbe5f311119104040d`.

Candidate K=20; final K=5; minimum relevance=2; evidence budget=6000 characters.

## Findings

In this single run, **rewriting + filtering** and **rewriting + reranking + filtering** covered **28/30 expected facts**, compared with **27/30 for baseline RAG**. Rewriting + filtering had no flagged material errors, but one citation-support problem; adding reranking removed that citation problem but produced an erroneous general-knowledge claim on the PostgreSQL control. This does not establish a universal winning mode.

All four filtering modes rejected all 60 candidates across the three out-of-corpus questions in each rewrite condition, leaving zero evidence. However, **all eight modes already acknowledged insufficient corpus evidence on all three controls**. Filtering therefore improved evidence selection without improving that particular answer-level score. Several modes still volunteered unverified general-knowledge instructions; four q23 answers contained spot-checked errors. Empty retrieval is not a guarantee against hallucination.

Rewriting increased designated-section presence in packed evidence from **9/10 to 10/10**. On q16, rewrite alone reached a nested external-interface subsection; combining it with scoring selected the more directly useful Kotlin/JS external-modifier rules. Section hits are a proxy and do not measure completeness. Filtering also removed two designated-section candidates without rewriting and four with rewriting; these were not necessarily useful to the specific question.

There were regressions: filtering without rewriting scored **25/30**, reranking + filtering without rewriting **23/30**, and rewriting alone **23/30**. Missing signatures, feature-status caveats and historical introduction dates contributed. The very short q20 rewrite-only answer correctly named the requested feature but omitted the broader pattern expected by the suite. Differences between answers sharing identical evidence also show generation variability.

Scoring cost about **5.6–5.8 seconds per question**, in addition to retrieval and answer generation. Rewriting added about **0.7 seconds per question**. Actual shared experiment expenditure was approximately **$0.0503 for 221 calls** using the repository pricing estimates. Keep enhancements opt-in: baseline was already strong, while rewriting + filtering is a promising choice for this corpus that warrants more questions and repeated trials.

## Retrieval

| Mode | Candidate section hits / 10 | Packed section hits / 10 | Empty controls / 3 | Chunks filtered | Designated-section chunks filtered |
| --- | ---: | ---: | ---: | ---: | ---: |
| rag | 10 | 9 | 0 | 0 | 0 |
| rag-rerank | 10 | 9 | 0 | 0 | 0 |
| rag-filter | 10 | 9 | 3 | 210 | 2 |
| rag-rerank-filter | 10 | 9 | 3 | 210 | 2 |
| rag-rewrite | 10 | 10 | 0 | 0 | 0 |
| rag-rerank-rewrite | 10 | 10 | 0 | 0 | 0 |
| rag-filter-rewrite | 10 | 10 | 3 | 210 | 4 |
| rag-rerank-filter-rewrite | 10 | 10 | 3 | 210 | 4 |

## Answer quality

| Mode | Correct facts | Partial facts | Answers with material errors | Supported citation sets | Problematic citation sets | Honest controls / 3 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| rag | 27/30 | 2 | 0 | 10 | 2 | 3 |
| rag-rerank | 27/30 | 3 | 1 | 12 | 0 | 3 |
| rag-filter | 25/30 | 3 | 2 | 10 | 0 | 3 |
| rag-rerank-filter | 23/30 | 6 | 1 | 10 | 0 | 3 |
| rag-rewrite | 23/30 | 5 | 0 | 13 | 0 | 3 |
| rag-rerank-rewrite | 26/30 | 4 | 0 | 12 | 0 | 3 |
| rag-filter-rewrite | 28/30 | 2 | 0 | 9 | 1 | 3 |
| rag-rerank-filter-rewrite | 28/30 | 2 | 1 | 10 | 0 | 3 |

Post-generation external spot-check: Post-generation spot-check of unsolicited q23 configuration claims, separate from the frozen Kotlin corpus and never supplied to generation/scoring. Not a comprehensive PostgreSQL audit.

Official docs list slot.name default debezium, plugin.name default decoderbufs, supported plugins decoderbufs and pgoutput, and explicitly permit manual slot creation. This contradicts the flagged default-name, plugin, and manual-creation claims. [Official Debezium documentation](https://debezium.io/documentation/reference/stable/connectors/postgresql.html) (accessed 2026-10-04).


## Cost and latency

Actual recorded calls (shared preparation counted once): 221; tokens: 355,481; estimated cost: $0.050258.

Below, each mode includes its full preparation cost as if run independently. These rows must not be summed to obtain experiment cost. Local retrieval includes subprocess startup, index checks and embedding work; it has no hosted-token charge. Provider pricing uses the repository adapter estimates.

| Mode | Stage | Seconds | Tokens | Estimated USD |
| --- | --- | ---: | ---: | ---: |
| rag | rag_rewrite | 0.00 | 0 | 0.000000 |
| rag | retrieval | 3.00 | 0 | 0.000000 |
| rag | rag_score | 0.00 | 0 | 0.000000 |
| rag | answer | 22.63 | 18456 | 0.002516 |
| rag-rerank | rag_rewrite | 0.00 | 0 | 0.000000 |
| rag-rerank | retrieval | 3.00 | 0 | 0.000000 |
| rag-rerank | rag_score | 72.98 | 111882 | 0.017024 |
| rag-rerank | answer | 23.10 | 18762 | 0.002340 |
| rag-filter | rag_rewrite | 0.00 | 0 | 0.000000 |
| rag-filter | retrieval | 3.00 | 0 | 0.000000 |
| rag-filter | rag_score | 72.98 | 111882 | 0.017024 |
| rag-filter | answer | 23.83 | 13560 | 0.001887 |
| rag-rerank-filter | rag_rewrite | 0.00 | 0 | 0.000000 |
| rag-rerank-filter | retrieval | 3.00 | 0 | 0.000000 |
| rag-rerank-filter | rag_score | 72.98 | 111882 | 0.017024 |
| rag-rerank-filter | answer | 25.32 | 13596 | 0.001475 |
| rag-rewrite | rag_rewrite | 9.07 | 1225 | 0.000197 |
| rag-rewrite | retrieval | 4.77 | 0 | 0.000000 |
| rag-rewrite | rag_score | 0.00 | 0 | 0.000000 |
| rag-rewrite | answer | 22.04 | 18381 | 0.002471 |
| rag-rerank-rewrite | rag_rewrite | 9.07 | 1225 | 0.000197 |
| rag-rerank-rewrite | retrieval | 4.77 | 0 | 0.000000 |
| rag-rerank-rewrite | rag_score | 74.99 | 112450 | 0.017037 |
| rag-rerank-rewrite | answer | 23.82 | 18486 | 0.001765 |
| rag-filter-rewrite | rag_rewrite | 9.07 | 1225 | 0.000197 |
| rag-filter-rewrite | retrieval | 4.77 | 0 | 0.000000 |
| rag-filter-rewrite | rag_score | 74.99 | 112450 | 0.017037 |
| rag-filter-rewrite | answer | 27.74 | 14492 | 0.002009 |
| rag-rerank-filter-rewrite | rag_rewrite | 9.07 | 1225 | 0.000197 |
| rag-rerank-filter-rewrite | retrieval | 4.77 | 0 | 0.000000 |
| rag-rerank-filter-rewrite | rag_score | 74.99 | 112450 | 0.017037 |
| rag-rerank-filter-rewrite | answer | 24.35 | 14191 | 0.001538 |

## Method and limits

One run per question and mode; no significance or general reliability claim. All modes use the same corpus, base answering instructions and original question. Mode order rotates. Rewrites and candidate pools are shared within each query condition; scoring results are shared across applicable modes. Auxiliary calls always use Flash with reasoning disabled. No temperature override is sent. Expected facts and relevance labels never enter generation or scoring prompts.

The rubric, cutoff and K values were fixed before generation. Flash relevance scores are ordinal judgments, not calibrated probabilities or independent ground truth. Retrieval metrics use exact designated section-path labels, which are not exhaustive and do not guarantee answer support. Filtered designated chunks can be irrelevant to the specific question despite sharing its section. A corpus miss does not imply the model cannot answer from general knowledge.

Reviews were authored separately by the implementing assistant against the saved source excerpts and full answers, not by the Flash scorer. Correct, partial, missing and incorrect facts are distinguished. Citation judgments check the actual packed evidence. Controls pass when corpus insufficiency is honestly acknowledged without invented support; general knowledge may be provided if distinguished from evidence. Missing expected caveats are not automatically material errors. No code examples were compiled. Provider timings vary with caching and service load.

## Answers and independent reviews

### q01 — rag

How can a function require an implicit service and how is that service resolved?

Search query: How can a function require an implicit service and how is that service resolved?

In Kotlin, a function can require an implicitly-available service by declaring a **context parameter**: use the `context` keyword followed by `name: Type` before the function declaration. The body can then use the named parameter directly [5].

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

Resolution happens at the **call site**: Kotlin searches the current scope for a value whose type matches the required context parameter type. Matching is by type only [3]. The matching value must be provided via a surrounding context, e.g. `context(users) { outputMessage("Hi") }` [5].

If two compatible values of the same type are in scope at the same level, the compiler reports an ambiguity error [3]:

```kotlin
context(serviceA, serviceB) {
    outputMessage("This will not compile") // ambiguity: both match UserService
}
```

You can name a context parameter `_` if you don't need to refer to it directly; it still satisfies the requirement for called functions, and you can retrieve its value explicitly with `contextOf<T>()` [1].

Note: context parameters replace the older experimental context receivers feature [5]. The source snapshots don't state a release version or availability status, so I can't confirm which Kotlin versions support this syntax.

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; Context parameters
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 475–509; What's new in Kotlin 2.4.0 > Standard library > New map fallback functions to distinguish null values and missing keys
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [4] [Extensions](https://kotlinlang.org/docs/extensions.html), lines 206–221; Extensions > Extension functions > Anonymous extension functions
- [5] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 1–46; Context parameters

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct. References [3] and [5] support resolution and declaration; [1] supports anonymous parameters.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-rerank

How can a function require an implicit service and how is that service resolved?

Search query: How can a function require an implicit service and how is that service resolved?

A function can require an implicit service by declaring it as a **context parameter** using the `context` keyword followed by `name: Type`:

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

The service is resolved **by type** at the call site: Kotlin searches the current scope for matching context values. It is usually provided via a `context(...) { ... }` block:

```kotlin
val users = object : UserService {
    override fun log(message: String) = println(message)
}

context(users) {
    outputMessage("Hello") // users is resolved implicitly
}
```

If multiple compatible values of the same type are available at the same scope level, the compiler reports an **ambiguity error** [1]. You can name the parameter `_` if you don't need to refer to it directly; it still participates in resolution, and you can access it via `contextOf<UserService>()` [3][4].

Note: context parameters are documented as replacing the older experimental "context receivers" feature [5]. Availability may depend on the Kotlin version and feature status (e.g., they were a preview in Kotlin 2.2.0) [2]; check your specific compiler version.

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [2] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 51–72; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; Context parameters
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 74–81; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters
- [5] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 1–46; Context parameters

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct. Correct historical preview caveat is supported by the retrieved Kotlin 2.2 note.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-filter

How can a function require an implicit service and how is that service resolved?

Search query: How can a function require an implicit service and how is that service resolved?

In Kotlin, a function can require an implicit service by declaring it as a **context parameter** using the `context` keyword, followed by a parameter list of `name: Type` pairs [4]. The parameter acts like an implicit dependency that the caller must supply in the surrounding scope.

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

**Resolution:** Context parameters are resolved at the **call site** by searching the current scope for matching values, matched **by type** [2]. If multiple compatible values exist at the same scope level, the compiler reports an **ambiguity error** [2].

```kotlin
fun main() {
    val serviceA = object : UserService { /* ... */ }
    val serviceB = object : UserService { /* ... */ }

    context(serviceA, serviceB) {
        outputMessage("Hi") // Ambiguity error: both match UserService
    }
}
```

**Anonymous parameters:** You can name a context parameter `_` when you don't need to reference it directly. It still satisfies resolution for called functions but is not accessible by name; use `contextOf<T>()` to retrieve its value explicitly [1][3].

```kotlin
context(_: UserService)
fun logWelcome() {
    outputMessage("Welcome!")          // satisfies UserService context
    contextOf<UserService>().log("Hi!") // explicit access
}
```

**Caveats:** Context parameters are described as a preview feature in Kotlin 2.2.0 [3][4]; availability and exact behavior may vary across versions, and the cited snapshots do not guarantee compatibility with other releases.

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; Context parameters
- [2] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [3] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 74–81; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 51–72; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct; the preview caveat is historical, not a claim that the feature is still preview.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-rerank-filter

How can a function require an implicit service and how is that service resolved?

Search query: How can a function require an implicit service and how is that service resolved?

A function declares an implicit dependency by listing it as a **context parameter** using the `context` keyword before the function declaration. For example:

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

At the call site, Kotlin resolves the required context value by **searching the current scope and matching by type**. If multiple compatible values exist at the same scope level, the compiler reports an ambiguity error [1].

Example of ambiguity:

```kotlin
val serviceA = object : UserService { override fun log(message: String) = println("A: $message") }
val serviceB = object : UserService { override fun log(message: String) = println("B: $message") }

context(serviceA, serviceB) {
    outputMessage("This will not compile") // ambiguity error
}
```

You can also name the context parameter `_` when you don't need to reference it directly; the anonymous parameter still satisfies resolution, and you can retrieve the value explicitly with `contextOf<T>()` [3].

**Caveat:** Context parameters were a preview feature in Kotlin 2.2.0, and availability may vary [2].

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [2] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 51–72; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; Context parameters
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 74–81; What's new in Kotlin 2.2.0 > Language > Preview of context parameters > How to declare context parameters

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct and source-supported, with a historical preview caveat.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-rewrite

How can a function require an implicit service and how is that service resolved?

Search query: require implicit service function resolution

A function can require an implicit service by declaring it as a **context parameter** using the `context(...)` modifier. The service is then resolved at the call site by matching the required type against values available in the surrounding scope [1].

```kotlin
interface UserService {
    fun log(message: String)
}

// The function requires a UserService implicitly
context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

Resolution rules:

- Kotlin searches the current scope at the call site for a value matching the context parameter's **type** [1].
- If multiple compatible values exist at the same scope level, the compiler reports an **ambiguity** error [1].
- You may name the parameter `_` if you don't need to reference it directly; it can still satisfy the context requirement. Use `contextOf<T>()` to access its value explicitly [2].

Example of providing the service at the call site:

```kotlin
val serviceA = object : UserService {
    override fun log(message: String) = println("A: $message")
}

context(serviceA) {
    outputMessage("Hello") // resolves serviceA as the UserService
}
```

The exact availability and syntax of context parameters may vary by Kotlin version; the reference material does not state a specific release for this feature [1][2].

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [2] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 48–59; Context parameters
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 475–509; What's new in Kotlin 2.4.0 > Standard library > New map fallback functions to distinguish null values and missing keys
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 151–166; What's new in Kotlin 2.2.0 > Language > Preview of context-sensitive resolution
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 359–369; What's new in Kotlin 2.4.20 > Kotlin/Wasm > Changes to top-level require() calls in @JsFun declarations

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct; [1] supports resolution and [2] anonymous parameters.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-rerank-rewrite

How can a function require an implicit service and how is that service resolved?

Search query: require implicit service function resolution

A function can declare an implicit dependency with the `context` keyword, listing parameters as `name: Type`. Callers then supply a matching value in the surrounding scope. Kotlin resolves context parameters at the call site by searching for values whose types match; if multiple compatible values exist at the same scope level, it reports an ambiguity. An explicit context argument can resolve such ambiguities. [1][2][3]

Example:

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}

fun main() {
    val service = object : UserService {
        override fun log(message: String) = println(message)
    }

    context(service) {
        outputMessage("Hello")
    }
}
```

Here `outputMessage` requires a `UserService` in context; it is resolved from the `context(service)` block at the call site [1][2].

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 1–46; Context parameters
- [2] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 94–120; Context parameters > Context parameters resolution > Pass context arguments explicitly
- [4] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct. Mentions explicit context arguments without their experimental opt-in caveat; the retrieved selection ends before that caveat. This omission affects the optional extension, not the core example.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-filter-rewrite

How can a function require an implicit service and how is that service resolved?

Search query: require implicit service function resolution

# Requiring and Resolving an Implicit Service via Context Parameters

A function declares an implicit dependency using the `context` keyword followed by a parameter list of the form `name: Type`. The service is resolved at the call site by matching the declared parameter's **type** against values available in the surrounding context scope. [1][4]

## Declaring the requirement

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")   // `users` is available implicitly
}
```

Here `outputMessage` requires a `UserService` in context but does not take it as an ordinary parameter. [4]

## Resolving it at the call site

Provide the value inside a `context(...) { ... }` block. Kotlin searches the enclosing scope for a matching context value, matching by type. [1]

```kotlin
fun main() {
    val service = object : UserService {
        override fun log(message: String) = println(message)
    }

    context(service) {
        outputMessage("Hello")   // `service` is found by type
    }
}
```

## Ambiguity and explicit selection

If multiple compatible values exist at the same scope level, resolution is ambiguous and the compiler reports an error. [1] Overloads that differ only by context parameter can also become ambiguous. [2] In either case you can pass the context argument explicitly by name at the call site: [2]

```kotlin
context(serviceA, serviceB) {
    // outputMessage("...")           // ambiguous: two UserService values
    outputMessage(users = serviceA)  // explicit selection
}
```

## Caveats

- The retrieved docs do not state a Kotlin version or availability status for context parameters; treat feature availability as unverified. [1][2][4] Context parameters replace the older experimental **context receivers** feature. [4]
- Matching is purely by type, not by parameter name or value. [1]

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [2] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 94–120; Context parameters > Context parameters resolution > Pass context arguments explicitly
- [3] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions
- [4] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 1–46; Context parameters

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct. The optional explicit-selection example omits -Xexplicit-context-arguments; retrieved chunks do not include that caveat. Feature-availability caution is general rather than the specific opt-in guidance.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q01 — rag-rerank-filter-rewrite

How can a function require an implicit service and how is that service resolved?

Search query: require implicit service function resolution

A function can declare an implicit dependency with **context parameters**: use the `context` keyword followed by one or more `name: Type` declarations [1]. The function body can then use those parameters without them being passed in the ordinary parameter list.

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

At the call site, Kotlin resolves context parameters by searching the current scope for matching values and matching them **by type** [2]. If multiple compatible values exist at the same scope level, the compiler reports an ambiguity [2]. To resolve ambiguity, you can pass a context argument explicitly, e.g. `sendNotification(emailSender = defaultEmailSender)` [3].

Example:

```kotlin
fun main() {
    val service = object : UserService {
        override fun log(message: String) = println(message)
    }

    context(service) {
        outputMessage("Hello") // service is resolved from the surrounding context
    }
}
```

Note: the retrieved references describe context parameters as replacing the older experimental context receivers feature, but they do not establish version availability [1].

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 1–46; Context parameters
- [2] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 61–92; Context parameters > Context parameters resolution
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), lines 94–120; Context parameters > Context parameters resolution > Pass context arguments explicitly
- [4] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions

- f1: **correct** — Declares context(users: UserService) before the function.
- f2: **correct** — Explains call-site matching by type within the available context.
- f3: **correct** — Explicitly identifies same-scope compatible values as ambiguous.

Material error: False. Citations: supported. All core facts correct. Optional explicit arguments are mentioned without the experimental opt-in caveat, which is outside the packed source span.

Reference excerpt: [reports/task1/sources/context-parameters.txt](../../reports/task1/sources/context-parameters.txt).

### q11 — rag-rerank

Where should expect and actual declarations live and which parts must match?

Search query: Where should expect and actual declarations live and which parts must match?

Expected declarations live in the **common source set** and must be marked with `expect`; they define the API but contain no implementation. Actual declarations live in each **platform-specific source set**, must be marked with `actual`, and provide the implementation. Both must be declared in the **same package** and have **matching signatures** [1][2][5].

Key rules:
- `expect` goes in common code; `actual` goes in platform-specific code [1].
- Every expected declaration must have a matching actual declaration for every platform target [2].
- Expected declarations must not contain any implementation [1][2].
- The package must match (e.g., `org.mygroup.myapp.MyType`) [2].
- Signatures must match; the IDE flags mismatched signatures and package differences [5].

Example:
```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = "JVM"

// nativeMain
actual fun platformName(): String = "Native"
```

Note: when using intermediate source sets (e.g., `iosMain` shared by `iosArm64Main` and `iosSimulatorArm64Main`), the `actual` declaration typically lives in the intermediate source set rather than each platform source set [2].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 269–298; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Application in frameworks
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 29–39; Expected and actual declarations > Rules for expected and actual declarations

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **correct** — Requires matching packages and coverage and explicitly matching signatures.

Material error: False. Citations: supported. Covers all expected facts, including intermediate source sets and matching signatures.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-filter

Where should expect and actual declarations live and which parts must match?

Search query: Where should expect and actual declarations live and which parts must match?

`expect` declarations go in the **common source set** and `actual` declarations go in each **platform-specific source set**; both must be in the **same package**, and every `expect` declaration must have a matching `actual` for every target [1][2].

Key points:

- In common code, declare a standard Kotlin construct (function, property, class, interface, enum, or annotation) and mark it `expect`; it must not contain an implementation [1][2].
- In each platform source set, declare the same construct *in the same package* and mark it `actual`, providing the implementation [1][2].
- The compiler verifies that every `expect` has a matching `actual` in every platform source set, that `expect` has no implementation, and that packages match [2].
- `actual` declarations may live in an intermediate shared source set (e.g. `iosMain`), and the compiler uses them for the associated platforms [2].

Example:

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = "JVM"

// nativeMain
actual fun platformName(): String = "Native"
```

Caveat: The cited guide snapshot does not specify release/version availability, so exact IDE behavior or compiler messaging may vary by version [1][2].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 269–298; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Application in frameworks
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 29–39; Expected and actual declarations > Rules for expected and actual declarations

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Intermediate source sets are explained; signature compatibility is only implicit in matching declarations and the example.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-rerank-filter

Where should expect and actual declarations live and which parts must match?

Search query: Where should expect and actual declarations live and which parts must match?

`expect` declarations live in a **common source set** (e.g. `commonMain`), while their `actual` counterparts live in each **platform-specific source set** [1]. The same construct must be declared in the **same package** on both sides, and the compiler requires a matching `actual` for every `expect` in every targeted platform source set [1][2]. Expected declarations must not contain any implementation [1][2].

Small example:

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = "JVM"

// nativeMain
actual fun platformName(): String = "Native"
```

Version/availability caveat: the cited snapshot has unknown release and availability, so exact behavior may vary by Kotlin version.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 269–298; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Application in frameworks
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 29–39; Expected and actual declarations > Rules for expected and actual declarations

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **partial** — Places actual in platform code; omits shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Omits the shared intermediate-source-set option and explicit signature matching, despite both being present in the supplied evidence.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-rewrite

Where should expect and actual declarations live and which parts must match?

Search query: expect actual declarations location matching requirements interoperability

**Where they live:** the `expect` declaration goes in the **common source set**; the `actual` declarations go in **each platform-specific source set** (or an intermediate shared source set such as `iosMain`) [1][2].

**What must match:** the compiler matches each actual to its expected and requires that:

- Every `expect` declaration in common has a matching `actual` in every platform-specific source set [2].
- Both share the **same package** (e.g. `org.mygroup.myapp.MyType`) [1][2].
- The expected declaration contains **no implementation** [1][2].

The compiler then merges the pair per platform, so common-code uses of the expected declaration resolve to the platform's actual implementation [2].

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = System.getProperty("os.name") ?: "Unknown"
```

Version caveat: expect/actual classes are documented as Beta [5]; the other cited rules carry no release/availability guarantee.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 1–5; Expected and actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 196–218; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Correct placement and package/coverage rules; explicit signature matching omitted.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-rerank-rewrite

Where should expect and actual declarations live and which parts must match?

Search query: expect actual declarations location matching requirements interoperability

**Where they live:**
- The `expect` declaration goes in the **common source set** (e.g. `commonMain`), marked with `expect`, with no implementation [1].
- The `actual` declaration goes in each **platform-specific source set** (e.g. `jvmMain`, `nativeMain`), marked with `actual`, and typically provides the platform-specific implementation [1].
- For intermediate source sets shared across targets (e.g. `iosMain` shared by `iosArm64Main` and `iosSimulatorArm64Main`), the `actual` declarations are typically placed in the intermediate source set rather than in each platform source set [2].

**What must match:**
- Every `expect` declaration must have a matching `actual` declaration in **every** platform-specific source set [2].
- They must be in the **same package** (e.g. `org.mygroup.myapp.MyType`) [2].
- The `expect` declaration must not contain any implementation [2].

**Example:**
```kotlin
// commonMain
expect val identity: Identity

interface Identity {
    val userName: String
    val processID: Long
}
```
```kotlin
// jvmMain
actual val identity: Identity = JVMIdentity()

class JVMIdentity(
    override val userName: String = System.getProperty("user.name") ?: "none",
    override val processID: Long = ProcessHandle.current().pid()
) : Identity
```
[4]

**Note:** An `actual` declaration doesn't have to be written from scratch — it can be satisfied by an existing type (including via a type alias), as long as it meets the requirements of the `expect` declaration [3].

The snapshot doesn't state which Kotlin version introduced these rules, so version-specific behavior isn't guaranteed beyond the cited documentation.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Correct intermediate-source-set guidance and package/coverage rules; signature matching remains implicit.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-filter-rewrite

Where should expect and actual declarations live and which parts must match?

Search query: expect actual declarations location matching requirements interoperability

# Where `expect`/`actual` Declarations Live and What Must Match

## Placement

- **`expect` declarations** go in the **common source set** and are marked with the `expect` keyword. They are standard Kotlin constructs (function, property, class, interface, enum, or annotation) with **no implementation** [1].
- **`actual` declarations** go in each **platform-specific source set**, marked with `actual`, and typically provide a platform-specific implementation [1]. They may also be placed in **intermediate source sets** shared between targets (e.g., `iosMain` shared by `iosArm64Main` and `iosSimulatorArm64Main`) [2].

## What must match

During compilation for a specific target, the compiler matches each `actual` with its corresponding `expect` and enforces [2]:

1. **Every** `expect` in the common source set has a matching `actual` in **every** platform-specific source set.
2. `expect` declarations contain **no implementation**.
3. The `actual` shares the **same package** as the corresponding `expect` (e.g., `org.mygroup.myapp.MyType`).

## Merging

The compiler merges matching `expect`/`actual` pairs, generating one declaration with the `actual` implementation for each platform. Uses of the `expect` declaration in common code resolve to the correct `actual` in the resulting platform code [2].

## Type-alias caveat

An `actual` doesn't have to be written from scratch — it can be satisfied by an existing type (e.g., a third-party class) via a type alias, provided that type meets all requirements of the `expect` declaration [3].

## Example

```kotlin
// commonMain
expect val identity: Identity

interface Identity {
    val userName: String
    val processID: Long
}

// jvmMain
actual val identity: Identity = JVMIdentity()

// nativeMain
actual val identity: Identity = NativeIdentity()
```

Here the `expect val` lives in `commonMain` and each `actual val` provides a platform-specific value [4].

## Version caveat

The snapshot cites no release and marks availability as unknown; **release versions are not feature availability**, so verify these rules against the docs for your specific Kotlin version [1][2].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Correct placement, package and target coverage; does not explicitly identify matching signatures.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag-rerank-filter-rewrite

Where should expect and actual declarations live and which parts must match?

Search query: expect actual declarations location matching requirements interoperability

**Where they live:** The `expect` declaration goes in the **common source set**, and the corresponding `actual` declaration goes in **each platform-specific source set** [1]. Actuals can also be declared in an **intermediate source set** shared between targets (e.g. `iosMain` for `iosArm64Main` and `iosSimulatorArm64Main`); the compiler then uses those actuals for the corresponding platforms [2].

**What must match:**

- **Package** — every actual must share the same package as its expected declaration (e.g. `org.mygroup.myapp.MyType`) [1][2].
- **Construct kind and signature** — the expected construct can be a function, property, class, interface, enum, or annotation, and the actual must match it [1].
- **No implementation in the expected declaration** — it must not contain a body/implementation [1][2].
- **Coverage** — every expected declaration must have a matching actual in *every* platform-specific source set being compiled [2].

The compiler matches each actual to its expected during target compilation; if both rules are satisfied, it merges them into one declaration using the platform implementation, so common-code uses of the expected call the correct actual [2].

Example:

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = "JVM"

// nativeMain
actual fun platformName(): String = "Native"
```

If the implementation already exists as another type, you can satisfy the actual using a **type alias** instead of writing it from scratch, provided the type meets the expected declaration's requirements (e.g. `java.time.Month` for an expected `Month` enum) [3].

One caveat: intermediate source sets are a more advanced pattern; the docs present them as an available option for shared actuals, not as the default structure [2].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 124–152; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions > Expected and actual properties

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **correct** — Requires matching packages and coverage and explicitly matching signatures.

Material error: False. Citations: supported. Explicitly covers signatures, package, coverage and intermediate source sets. The extra characterization of intermediate sets as advanced is editorial, not established by the excerpt.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q11 — rag

Where should expect and actual declarations live and which parts must match?

Search query: Where should expect and actual declarations live and which parts must match?

Expected declarations live in a **common source set** and are marked with `expect`; each actual declaration lives in a **platform-specific source set** (or a shared intermediate source set) and is marked with `actual` [1][2]. During compilation, the compiler matches each `actual` with its corresponding `expect` for a specific target [2].

Required to match:
- Same construct kind (function, property, class, interface, enum, or annotation) [1]
- Same **package**, e.g. `org.mygroup.myapp.MyType` [2]
- Every `expect` must have a matching `actual` in every platform source set, and the `expect` must not contain any implementation [1][2]

Example:

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = System.getProperty("os.name") ?: "Unknown"
```

Version caveat: the reference data does not specify a Kotlin release or feature-availability version, so exact behavior across versions is uncertain. Expected/actual classes are noted as Beta [4].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 1–5; Expected and actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 196–218; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes

- f1: **correct** — Places implementation-free expect declarations in common code.
- f2: **correct** — Places actual in platform code and allows shared intermediate source sets.
- f3: **partial** — Requires matching packages and coverage; signatures are only implicit.

Material error: False. Citations: supported. Correct placement, packages and target coverage, but signature matching is not explicitly explained.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-filter

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes. There are two main approaches:

## 1. Type aliases (most direct)

An `actual typealias` maps the expected declaration directly to an existing platform type:

```kotlin
// commonMain
expect class MyDate

// jvmMain
actual typealias MyDate = java.time.LocalDate
```

The alias must be declared in the same package as the expected declaration, and the referred class lives elsewhere [2]. If the platform type references other types (e.g. `LocalDate` uses `Month`), those must also be declared as `expect`/`actual` — hence the doc's example declares both `Month` and `MyDate` [2].

## 2. Inheriting from the platform class

You can also make an `actual class` extend an existing platform base class, exposing its members as `actual` properties:

```kotlin
// jvmMain
actual class CommonIdentity : Identity() {
    actual val userName = login
    actual val processID = pid
}
```

Here `CommonIdentity` is compatible with your common design while taking advantage of the existing platform type [1].

## Caveats

- `typealias` cannot be used to satisfy an `expect` if the aliased platform type doesn't match the expected shape (e.g. same-enum constants — extra entries are allowed on actualization but the expected constants must all be present) [5].
- Visibility expansion (e.g. an `internal` expect satisfied by a `public` platform type) requires `@Suppress("ACTUAL_WITHOUT_EXPECT")` on the `actual typealias` [3].
- The reference material is version-agnostic; confirm exact rules against your Kotlin release.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 88–120; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 358–373; Expected and actual declarations > Advanced use cases > Additional enumeration entries on actualization

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. Correct core alias guidance. The statement that all referenced platform types must become expect/actual is broader than the source example: this is needed for types exposed in the common contract.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-rerank-filter

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes — use an `actual typealias` when the existing platform type meets all the requirements of the expected declaration.

For example, given these expected declarations [3]:

```kotlin
expect enum class Month { JANUARY, /* ... */ DECEMBER }

expect class MyDate {
    fun getYear(): Int
    fun getMonth(): Month
    fun getDayOfMonth(): Int
}
```

On the JVM you can connect them directly to `java.time` types [1][3]:

```kotlin
actual typealias Month = java.time.Month
actual typealias MyDate = java.time.LocalDate
```

Notes:
- Define the `typealias` in the same package as the expected declaration, and declare the referred class elsewhere [1].
- If the actual type's signature transitively relies on another expected type (e.g. `LocalDate` uses `Month`), both must be declared `expect` in common code [1].
- Type aliases can also satisfy `expect annotation class` declarations (e.g. `actual typealias XmlSerializable = XmlRootElement`) [4].
- If the existing type's visibility is broader than the expected one, you may need `@Suppress("ACTUAL_WITHOUT_EXPECT")` on the actual typealias [2].

A typealias only works when the underlying type satisfies every requirement (constructors, members, supertypes) of the expected declaration; otherwise you must fall back to wrapping or inheriting from the platform class [3][5].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. Correct alias, compatibility and package rules; the dependency wording is limited to expected types in the signature.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-rewrite

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: satisfy common expected class using existing platform type without wrapping

Yes — in Kotlin Multiplatform you can satisfy a common `expect class` with an existing platform type instead of manually wrapping it, using one of these approaches:

**1. Inherit from the platform class in the actual declaration**

If the platform type already exists (e.g. `Identity` on the JVM), your `actual class` can extend it and expose the expected members:

```kotlin
// jvmMain
actual class CommonIdentity : Identity() {
    actual val userName = login
    actual val processID = pid
}
```

The docs describe exactly this pattern for reusing existing platform classes while keeping a common API [1][2].

**2. Use a typealias in the actual declaration**

For expected types whose platform counterpart is a plain class, you can map the `expect` directly to the platform type:

```kotlin
// commonMain
expect class MyDate

// jvmMain (in the same package as the expectation)
actual typealias MyDate = java.time.LocalDate
```

The typealias must be declared in the same package as the expected declaration, and the referred class is defined elsewhere [4]. You can also use typealiases to expose an existing public class as the actual for an internal expected class with `@Suppress("ACTUAL_WITHOUT_EXPECT")` [3].

Caveat: exact keyword combinations (`expect class` + `actual typealias`, inheritance rules, suppression behavior) can differ across compiler versions, and the reference snapshots don't pin a release or availability, so verify against the Kotlin version you're using.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 235–251; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 300–305; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Application in frameworks

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **partial** — Explains direct reuse without wrapping, but omits the complete API-compatibility requirement.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. Explains aliasing without a wrapper and package placement, but does not clearly state that the target must satisfy every expected requirement.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-rerank-rewrite

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: satisfy common expected class using existing platform type without wrapping

Yes. Kotlin Multiplatform lets you satisfy an `expect` declaration with an existing platform type using an `actual typealias`, rather than writing a wrapper class. [1][2]

Example:

```kotlin
// commonMain
expect enum class Month {
    JANUARY, FEBRUARY, /* ... */ DECEMBER
}

expect class MyDate {
    fun getYear(): Int
    fun getMonth(): Month
    fun getDayOfMonth(): Int
}

// jvmMain
actual typealias Month = java.time.Month
actual typealias MyDate = java.time.LocalDate
```

Key requirements:
- Define the `typealias` in the same package as the `expect` declaration, and put the referred class elsewhere. [1]
- The existing platform type must meet all requirements of the expected declaration. [2]
- If the platform type depends on another expected type (like `LocalDate` using `Month`), declare that dependency as `expect` too. [1]

This also works for expected annotation classes, e.g. `actual typealias XmlSerializable = javax.xml.bind.annotation.XmlRootElement`. [3]

Caveat: the alias target must be a real type you can reference; you cannot attach `actual` directly to a third-party or JDK type. [2]

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 88–120; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 235–251; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. All core facts correct. The dependency remark should be read as referring to the common contract, not every implementation dependency.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-filter-rewrite

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: satisfy common expected class using existing platform type without wrapping

Yes. On each platform where that specific `actual` is required, you can satisfy an expected class with an existing platform type by using an `actual typealias`, instead of wrapping the platform type in a new class.

The platform type must meet the requirements of the expected declaration, and the typealias should be in the same package as the expected declaration; the referred class can be defined elsewhere. [1] This is exactly the documented use case for reusing a third-party class as an `actual` declaration, since you cannot add the `actual` keyword directly to an existing type. [3]

Example:

```kotlin
// commonMain
expect class MyDate {
    fun getYear(): Int
    fun getMonth(): Month
    fun getDayOfMonth(): Int
}

expect enum class Month {
    JANUARY, FEBRUARY, MARCH, APRIL, MAY, JUNE, JULY,
    AUGUST, SEPTEMBER, OCTOBER, NOVEMBER, DECEMBER
}
```

```kotlin
// jvmMain
actual typealias Month = java.time.Month
actual typealias MyDate = java.time.LocalDate
```

Note that if the platform type depends on another expected type, that dependency also needs an expected declaration in common code. For example, because `LocalDate` uses `Month`, both need expected classes. [1]

The same pattern can also be used for expected annotation classes, e.g. `actual typealias XmlSerializable = XmlRootElement`. [4]

Caveat: the references do not specify which Kotlin versions support these behaviors; availability may vary.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 88–120; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. All core facts correct; reference [3] supports compatibility and [1] supports alias placement.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-rerank-filter-rewrite

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: satisfy common expected class using existing platform type without wrapping

Yes, in Kotlin Multiplatform you can satisfy an `expect` declaration with an existing platform type by using an `actual typealias` rather than wrapping it in a new class. This works when the existing type already meets all the requirements of the expected declaration [1][2].

**Example** — matching a platform date type:

```kotlin
// commonMain
expect enum class Month {
    JANUARY, FEBRUARY, /* ... */
}
expect class MyDate {
    fun getYear(): Int
    fun getMonth(): Month
    fun getDayOfMonth(): Int
}

// jvmMain
actual typealias Month = java.time.Month
actual typealias MyDate = java.time.LocalDate
```

Constraints to keep in mind:

- The `actual typealias` must be declared in the **same package** as the expected declaration, and the referred class must be defined elsewhere [1].
- The platform type must satisfy the expected declaration's API. For example, because `LocalDate` uses `Month`, you also need an expected declaration for `Month` (as shown above) [1].
- You can't add the `actual` keyword directly to a third-party or JDK type; a typealias is the mechanism for reusing it [2].
- This also works for `expect annotation class`: you can satisfy it with `actual typealias` pointing to an existing annotation type, e.g. `actual typealias XmlSerializable = XmlRootElement` [3].

If the expected declaration is a function or something that must be *created*, an interface-based approach (a common interface with platform-specific implementations) may be more appropriate [4].

**Caveat:** The referenced snapshots don't state a Kotlin release version, so exact behavior and availability of `expect`/`actual typealias` may vary across versions.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 88–120; Expected and actual declarations > Different approaches for using expected and actual declarations > Interfaces with expected and actual functions

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. All core facts correct. Wording that the referred class must be elsewhere is stronger than necessary; the requirement concerns the alias package.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes, in Kotlin Multiplatform you can satisfy a common `expected` class using an existing platform type, but the mechanism depends on the case.

Two documented approaches:

**1. Inherit from the platform class**

If the platform type is a class you can extend, declare an `actual class` that inherits from it and maps its members:

```kotlin
// commonMain
expect class CommonIdentity() {
    val userName: String
    val processID: Long
}

// jvmMain
actual class CommonIdentity : Identity() {
    actual val userName = login
    actual val processID = pid
}
```

The expected `CommonIdentity` is compatible with your design while reusing the existing JVM `Identity` type [1][2].

**2. Use a type alias (no wrapping)**

If you don't need to extend anything and the platform type already has the right shape, a `typealias` can connect the expected declaration directly to the platform type — no wrapper class is generated:

```kotlin
// commonMain
expect class MyDate(...)  // or expect enum class Month

// jvmMain
actual typealias Month = java.time.Month
actual typealias MyDate = java.time.LocalDate
```

The typealias must be declared in the same package as the expected declaration; the referred class can live elsewhere [3]. Type aliases can also expand visibility (e.g., an internal expected class aliased to an existing public type) [4].

Caveats:
- The general guidance is to prefer interfaces or standard constructs over `expect`/`actual` classes when sufficient; expected/actual classes are in Beta and the compiler emits a warning unless you opt in with `-Xexpect-actual-classes` [5].
- A typealias only works when the platform type's API matches the expected declaration's contract; otherwise you still need an actual class (possibly inheriting from the platform type).

The reference snapshot doesn't state version availability, so exact behavior/compiler flags may vary by release.

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 235–251; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 220–233; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. Correct alias mechanism, matching-contract requirement and package placement. The ellipsis constructor example is schematic, not compilable code.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q12 — rag-rerank

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Search query: Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes. In Kotlin Multiplatform, an `actual` declaration can be satisfied with an existing platform type via an `actual typealias`, as long as the aliased type meets all the requirements of the `expect` declaration.

For example, given a common expected class:

```kotlin
expect class MyDate {
    fun getYear(): Int
    fun getMonth(): Month
    fun getDayOfMonth(): Int
}
```

You can satisfy it on the JVM with an existing `java.time.LocalDate` using a type alias:

```kotlin
actual typealias MyDate = java.time.LocalDate
```

The type alias must be declared in the same package as the expected declaration, and the referred class should be created elsewhere [1]. If the expected type depends on other expected types (e.g., `Month`), those also need matching `actual` declarations (which can themselves be type aliases) [1].

You can even expand visibility: an `internal` expected class can be satisfied with an existing `public` type via `actual typealias`, adding `@Suppress("ACTUAL_WITHOUT_EXPECT")` as needed [2].

This approach avoids writing a wrapper class from scratch and lets you reuse existing platform (or third-party) types [3].

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 311–328; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes

- f1: **correct** — Uses actual typealias in JVM/platform code to map the common expected class to LocalDate.
- f2: **correct** — Explains direct reuse without wrapping and the requirement to satisfy the expected API.
- f3: **correct** — Places the alias in the expect package and allows the target type to be defined elsewhere.

Material error: False. Citations: supported. All three expected facts are covered, including reuse without a wrapper and dependent expected types.

Reference excerpt: [reports/task1/sources/multiplatform-expect-actual.txt](../../reports/task1/sources/multiplatform-expect-actual.txt).

### q13 — rag-rerank-filter

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin exceptions are exposed to Swift/Objective-C via the `@Throws` annotation:

- All Kotlin exceptions are unchecked, while Swift only has checked errors handled at compile time. So if Swift/Objective-C code calls a Kotlin method that throws an exception, the Kotlin method should be marked with `@Throws`, listing the "expected" exception classes [1].
- When compiling to a Swift/Objective-C framework, non-`suspend` functions that have or inherit `@Throws` become `NSError*`-producing methods in Objective-C and `throws` methods in Swift. `suspend` functions always have an `NSError*`/`Error` parameter in the completion handler [1].
- If a called Kotlin function throws an exception that is an instance of a class specified in `@Throws` (or a subclass), the exception is propagated as an `NSError`. Other Kotlin exceptions reaching Swift/Objective-C are considered unhandled and cause program termination [1].
- `suspend` functions without `@Throws` propagate only `CancellationException` (as `NSError`). Non-`suspend` functions without `@Throws` don't propagate Kotlin exceptions at all [1].

Note: the reverse direction is not implemented yet — Swift/Objective-C error-throwing methods aren't imported to Kotlin as exception-throwing [1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All expected exception rules are correctly covered by the single retained source chunk.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-rewrite

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: Kotlin exceptions exposed to Swift and Objective-C callers

Kotlin exceptions are unchecked, but Swift only has checked errors. To bridge them, mark Kotlin functions that throw with `@Throws(...)`.

- A non-`suspend` function with/inheriting `@Throws` is exposed to Objective-C as an `NSError*`-producing method and to Swift as a `throws` method [1].
- If a thrown exception is one of the classes listed in `@Throws` or a subclass, it propagates as `NSError` [1].
- Other Kotlin exceptions reaching Swift/Objective-C are unhandled and cause program termination [1].
- `suspend` functions always expose an `NSError*`/`Error` parameter in the completion handler; without `@Throws`, they propagate only `CancellationException` as `NSError` [1].
- Non-`suspend` functions without `@Throws` don’t propagate Kotlin exceptions at all [1].
- Reverse direction isn’t implemented: Swift/Objective-C throwing methods aren’t imported into Kotlin as exception-throwing [1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Calling Java from Kotlin](https://kotlinlang.org/docs/java-interop.html), lines 585–593; Calling Java from Kotlin > Checked exceptions
- [5] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 269–275; Interoperability with Swift/Objective-C > Mappings > Extensions and category members

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. Complete, source-supported exception rules, including cancellation and termination.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-rerank-rewrite

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: Kotlin exceptions exposed to Swift and Objective-C callers

Kotlin exceptions are **unchecked** — normally they are not exposed as declared errors to Swift/Objective-C [1]. To make a Kotlin function callable as a throwing method from Swift/Objective-C, you must mark it with `@Throws`, listing the expected exception classes [1].

Key points:

- Non-`suspend` functions with (or inheriting) `@Throws` become `NSError*`-producing methods in Objective-C and `throws` methods in Swift [1].
- `suspend` functions always have an `NSError*` / `Error` parameter in the completion handler [1].
- If a thrown exception is an instance of a class listed in `@Throws` (or a subclass), it propagates as `NSError`; other Kotlin exceptions reaching Swift/Objective-C are unhandled and cause program termination [1].
- `suspend` functions without `@Throws` propagate only `CancellationException` (as `NSError`); non-`suspend` functions without `@Throws` do not propagate Kotlin exceptions at all [1].
- The reverse direction is not implemented: Swift/Objective-C error-throwing methods are not imported into Kotlin as exception-throwing [1].

Example:

```kotlin
@Throws(IOException::class)
fun writeToFile() {
    throw IOException()
}
```

For Java callers, the same annotation is needed so Java signatures declare the checked exception [3]. Note that this is Kotlin/Native indirect interoperability with Swift through Objective-C [4].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 1–11; Interoperability with Swift/Objective-C
- [5] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 33–39; Interoperability with Swift/Objective-C > Using Kotlin in Swift/Objective-C > Hide Kotlin declarations from Objective-C and Swift

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All expected facts correct and Java is distinguished from the Objective-C bridge. The IOException example needs a Native-appropriate declaration/import.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-filter-rewrite

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: Kotlin exceptions exposed to Swift and Objective-C callers

Kotlin exceptions are exposed to Swift/Objective-C callers only if the Kotlin function is marked with `@Throws` (or inherits it). Such non-suspend functions are represented as `NSError*`-producing methods in Objective-C and `throws` methods in Swift[1].

If a called Kotlin function throws an exception that is an instance of one of the classes specified in `@Throws` or a subclass, it is propagated to Swift/Objective-C as an `NSError`. Any other Kotlin exception reaching Swift/Objective-C is considered unhandled and causes program termination[1].

For `suspend` functions, the completion handler always has an `NSError*`/`Error` parameter. A `suspend` function without `@Throws` propagates only `CancellationException` as `NSError`[1]. Non-suspend functions without `@Throws` do not propagate Kotlin exceptions at all[1].

The reverse translation is not implemented: Swift/Objective-C error-throwing methods are not imported into Kotlin as exception-throwing methods[1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All expected rules correct. The opening only-if statement is overbroad in isolation, but the later suspend/CancellationException exception explicitly corrects that scope.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-rerank-filter-rewrite

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: Kotlin exceptions exposed to Swift and Objective-C callers

Kotlin exceptions are exposed to Swift/Objective-C callers via the `@Throws` annotation, but with important limitations [1]:

- Since all Kotlin exceptions are unchecked while Swift only has checked errors, a Kotlin method that throws and is called from Swift/Objective-C should be marked with `@Throws`, listing the "expected" exception classes [1].
- When compiling to a Swift/Objective-C framework, non-`suspend` functions that have or inherit `@Throws` are represented as `NSError*`-producing methods in Objective-C and as `throws` methods in Swift. `suspend` function representations always have an `NSError*`/`Error` parameter in the completion handler [1].
- If a Kotlin function called from Swift/Objective-C throws an exception that is an instance of one of the `@Throws`-specified classes (or their subclasses), the exception is propagated as an `NSError`. Other Kotlin exceptions reaching Swift/Objective-C are considered unhandled and cause program termination [1].
- `suspend` functions without `@Throws` propagate only `CancellationException` (as `NSError`). Non-`suspend` functions without `@Throws` don't propagate Kotlin exceptions at all [1].

Note: the reverse translation is not implemented — Swift/Objective-C error-throwing methods aren't imported to Kotlin as exception-throwing [1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All expected exception rules correctly covered by the single retained source chunk.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin exceptions are **unchecked**, but Swift/Objective-C only understand checked errors (`NSError`/`throws`). So exposure depends on annotations:

- **Non-suspend functions marked or inheriting `@Throws`** are represented as `NSError*`-producing methods in Objective-C and `throws` methods in Swift [1]. If such a function throws an exception that is an instance of a class listed in `@Throws` (or a subclass), it is propagated as an `NSError` [1].
- **Other Kotlin exceptions reaching Swift/Objective-C are considered unhandled and cause program termination** [1].
- **Non-suspend functions without `@Throws` don’t propagate Kotlin exceptions at all** [1].
- **`suspend` functions** always have an `NSError*`/`Error` parameter in the completion handler; without `@Throws`, they propagate only `CancellationException` (as `NSError`) [1].

Example:

```kotlin
@Throws(IOException::class)
fun readFile() { throw IOException() }
```

When called from Swift/Obj-C, `IOException` can surface as an `NSError`/thrown error. Without `@Throws`, it would not propagate for a non-suspend function.

Note: the reverse direction is not implemented — Swift/Objective-C error-throwing methods are not imported into Kotlin as exception-throwing [1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Calling Java from Kotlin](https://kotlinlang.org/docs/java-interop.html), lines 585–593; Calling Java from Kotlin > Checked exceptions
- [5] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 33–39; Interoperability with Swift/Objective-C > Using Kotlin in Swift/Objective-C > Hide Kotlin declarations from Objective-C and Swift

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: mixed. All core rules correct. The opening overgeneralizes Objective-C as understanding only checked errors; the source says that about Swift. The IOException example needs a Native-appropriate declaration/import; none is supplied.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-rerank

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin's exception model doesn't map directly onto Swift/Objective-C's, so the exposure depends on annotations and function type.

**Key rules**

- All Kotlin exceptions are unchecked (caught at runtime), while Swift has compile-time checked errors [1].
- When Swift/Objective-C calls a Kotlin function that throws, the Kotlin function **should be marked with `@Throws`**, listing the "expected" exception classes [1].
- When compiled to a Swift/Objective-C framework, **non-`suspend` functions that have or inherit `@Throws`** are exposed as `NSError*`-producing methods in Objective-C and `throws` methods in Swift [1].
- **`suspend` functions** always have an `NSError*` / `Error` parameter in the completion handler [1].
- If a Kotlin function called from Swift/Objective-C throws an exception that is an instance of a `@Throws`-listed class (or subclass), it's propagated as an `NSError`. **Any other Kotlin exception reaching Swift/Objective-C is treated as unhandled and causes program termination** [1].
- **`suspend` functions without `@Throws`** propagate only `CancellationException` (as `NSError`). **Non-`suspend` functions without `@Throws` don't propagate Kotlin exceptions at all** [1].
- The reverse direction is **not implemented**: Swift/Objective-C error-throwing methods aren't imported into Kotlin as exception-throwing [1].

**Example**

```kotlin
@Throws(IOException::class)
fun writeToFile() {
    throw IOException()
}
```

With `@Throws`, the method appears in Swift as `throws` and in Objective-C as an `NSError**`-producing method; an `IOException` (or subclass) is converted to `NSError`. Without `@Throws` on a non-`suspend` function, the exception isn't propagated to the caller [1].

Note that the docs also point to a Kotlin-Swift interopedia example for further detail [2].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 563–571; Interoperability with Swift/Objective-C > Unsupported
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All core rules correctly track [1]. The illustrative IOException is not defined for Native; the retrieved Java example is not by itself a Native dependency declaration.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q13 — rag-filter

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Search query: How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin exceptions are exposed to Swift/Objective-C callers through the `@Throws` annotation.

- All Kotlin exceptions are unchecked, whereas Swift has only checked errors handled at compile time. So a Kotlin method that may throw and is called from Swift/Objective-C should be marked with `@Throws`, listing the "expected" exception classes [1].
- When compiling to a Swift/Objective-C framework, non-`suspend` functions with or inheriting `@Throws` are represented as `NSError*`-producing methods in Objective-C and `throws` methods in Swift. `suspend` functions always have an `NSError*`/`Error` parameter in the completion handler [1].
- If a called Kotlin function throws an exception that is an instance of one of the `@Throws`-specified classes (or a subclass), it is propagated as an `NSError`. Other Kotlin exceptions reaching Swift/Objective-C are unhandled and cause program termination [1].
- `suspend` functions without `@Throws` propagate only `CancellationException` as `NSError`. Non-`suspend` functions without `@Throws` don't propagate Kotlin exceptions at all [1].
- The reverse translation is not implemented: Swift/Objective-C error-throwing methods are not imported into Kotlin as exception-throwing [1].

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions

- f1: **correct** — Maps non-suspend @Throws functions to Swift throws and Objective-C NSError methods.
- f2: **correct** — Limits propagation to listed classes/subclasses and states termination for other escaping exceptions.
- f3: **correct** — Explains suspend error parameters, CancellationException-only propagation without @Throws, and non-suspend non-propagation.

Material error: False. Citations: supported. All expected exception rules are correctly covered by the single retained source chunk.

Reference excerpt: [reports/task1/sources/native-objc-interop.txt](../../reports/task1/sources/native-objc-interop.txt).

### q14 — rag-rewrite

How does Swift export expose Kotlin suspend functions?

Search query: Swift export expose Kotlin suspend functions interoperability

Swift export exposes Kotlin `suspend` functions (and suspending functional types) as Swift `async` functions, so you call them with `try await` [1][2][4].

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

```swift
// Swift
let msg = try await hello()
```

Caveats:
- Swift export is currently in Alpha [5].
- This is distinct from the older Objective-C interop path, where suspending functions are exposed via completion handlers, with `async` availability being "highly experimental" [3].
- The snapshot sources don't specify exact version/availability guarantees for Swift export concurrency beyond Kotlin 2.4.0 notes [2]; treat release versions as non-guarantees of feature availability.

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [4] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 1–13; Interoperability with Swift using Swift export

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **correct** — Distinguishes Swift export from Objective-C completion handlers and states Alpha status.

Material error: False. Citations: supported. All facts covered, including Alpha supported by the introductory Swift-export chunk [5].

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-rerank-rewrite

How does Swift export expose Kotlin suspend functions?

Search query: Swift export expose Kotlin suspend functions interoperability

Swift export exposes Kotlin `suspend` functions (and suspend functional types) as Swift `async` counterparts, called with `try await` [1][2].

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

```swift
// Swift
let msg = try await hello()
```

By default, exported suspending code runs on `Dispatchers.Default`; use `withContext()` in Kotlin to change dispatchers [5]. For comparison, the older Objective-C interop presents suspend functions as callback/completion-handler functions, with experimental Swift `async` support [3]. Names and exact behavior depend on the Kotlin version and feature status; Swift export availability is not guaranteed by these snippets.

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [4] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **partial** — Distinguishes Swift export from Objective-C completion handlers, but omits Alpha status.

Material error: False. Citations: supported. Correct mapping, default dispatcher and bridge distinction; no explicit Alpha status.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-filter-rewrite

How does Swift export expose Kotlin suspend functions?

Search query: Swift export expose Kotlin suspend functions interoperability

Swift export exposes Kotlin `suspend` functions as Swift `async` functions, so you can call them with `try await` [1][2]. Suspend functional types are also exported as Swift async counterparts [1][2].

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift!"
}
```

```swift
// Swift
let msg = try await hello()
```

A few caveats:

- This differs from Objective-C interop, where suspend functions appear as callback/completion-handler functions; Swift `async` calling there is described as highly experimental and limited [3].
- Coroutine dispatchers still apply: by default, exported Kotlin suspending code runs on `Dispatchers.Default`, and you can switch contexts with `withContext()` [5].
- The Swift export feature and its concurrency support have version/availability caveats; the Alpha status is noted in Kotlin 2.4.0 release material, but that does not guarantee compatibility with other versions [2].

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [4] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **correct** — Distinguishes Swift export from Objective-C completion handlers and states Alpha status.

Material error: False. Citations: mixed. States Alpha and distinguishes the bridges, but the cited release-note chunk [2] does not contain the Alpha claim.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-rerank-filter-rewrite

How does Swift export expose Kotlin suspend functions?

Search query: Swift export expose Kotlin suspend functions interoperability

Swift export maps Kotlin `suspend` functions (and suspend functional types) to Swift's idiomatic `async` counterparts, so you call them with `await` rather than completion handlers:

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift!"
}
```

```swift
// Swift
let msg = try await hello()
```

[1][2]

A few caveats:

- This behavior belongs to **Swift export**, which is distinct from the older Objective-C interop path. Under Objective-C interop, suspend functions are exposed via completion handlers, with `async` callable from Swift 5.5 onward but described as highly experimental and limited [3].
- By default the exported code runs in a coroutine context using `Dispatchers.Default`; use `withContext()` in Kotlin to change the dispatcher [5].
- Feature availability and stability vary by version and feature status; the snippets do not guarantee compatibility with other releases [1][2][4].

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [4] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **partial** — Distinguishes Swift export from Objective-C completion handlers, but omits Alpha status.

Material error: False. Citations: supported. Correct async mapping and bridge distinction; replaces Alpha status with a general version caveat.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag

How does Swift export expose Kotlin suspend functions?

Search query: How does Swift export expose Kotlin suspend functions?

Swift export exposes Kotlin `suspend` functions (and suspend functional types) as Swift `async` functions, so you call them with `try await` [1][2].

Example:

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

```swift
// Swift
let msg = try await hello()
```

Additional details:
- This structured-concurrency support is part of Swift export, which went Alpha in Kotlin 2.4.0 [2].
- By default, the coroutine context uses `Dispatchers.Default`; to run exported code on another dispatcher, switch with `withContext()` in Kotlin [5].
- This is distinct from the older Objective-C interop path, where suspend functions appear as completion handlers, with experimental direct `async` support [4].

Version/availability caveats: the exact docs snapshot release for [1] and [5] is unknown, and feature availability isn't guaranteed solely by release version [1][2][5].

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 404–425; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **correct** — Distinguishes Swift export from Objective-C completion handlers and states Alpha status.

Material error: False. Citations: mixed. All expected facts covered, but the Alpha-in-2.4.0 claim cites [2], whose packed text contains concurrency support, not the status transition.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-rerank

How does Swift export expose Kotlin suspend functions?

Search query: How does Swift export expose Kotlin suspend functions?

Swift export exposes Kotlin `suspend` functions as Swift `async` functions, so you call them from Swift with `await` [1][3]. Suspend functional types are exported as async counterparts as well [1][2].

Kotlin:
```kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

Swift:
```swift
let msg = try await hello()
```
[1][2]

Per the mapping table, `suspend fun` → `async` [3].

Notes/caveats:
- These snippets describe Swift export; the older Swift/Objective-C interop presents suspend functions as completion-handler functions, with Swift `async` support described as experimental and limited [4].
- By default, calling a Kotlin suspend function from Swift runs on `Dispatchers.Default`; use `withContext(...)` in Kotlin to switch dispatchers [5].
- Availability may depend on feature status and version; the references do not establish compatibility beyond what they state [1][2].

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 110–140; Interoperability with Swift using Swift export > Mappings
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **partial** — Distinguishes Swift export from Objective-C completion handlers, but omits Alpha status.

Material error: False. Citations: supported. Correct async/try-await mapping and Objective-C distinction; omits the explicit Alpha status.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-filter

How does Swift export expose Kotlin suspend functions?

Search query: How does Swift export expose Kotlin suspend functions?

Swift export exposes Kotlin `suspend` functions (and suspend functional types) as Swift `async` functions, so you call them from Swift with `try await`:

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

```swift
// Swift
let msg = try await hello()
```

This mapping is documented for Swift export [1][5], and in the 2.4.0 release notes it's described as support for structured concurrency [2]. By default, calling a Kotlin suspending function from Swift creates a coroutine context using `Dispatchers.Default`; use `withContext()` in Kotlin to switch dispatchers [4].

Note: this is distinct from Objective-C interop, where suspend functions are exposed as callback/completion-handler functions, with an experimental Swift `async` option since Swift 5.5 [3]. The provided references do not specify exact availability or version compatibility beyond what is stated, and release versions are not guarantees of feature availability.

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [4] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 15–23; Interoperability with Swift using Swift export

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **partial** — Distinguishes Swift export from Objective-C completion handlers, but omits Alpha status.

Material error: False. Citations: supported. Correct mapping and bridge distinction; a general availability caveat does not state Alpha status.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q14 — rag-rerank-filter

How does Swift export expose Kotlin suspend functions?

Search query: How does Swift export expose Kotlin suspend functions?

Swift export maps Kotlin `suspend fun` to Swift `async` functions, so you can call them with `try await` [3].

For example:

```kotlin
// Kotlin
suspend fun hello(): String {
    delay(1000)
    return "Hello Swift! This is Kotlin."
}
```

```swift
// Swift
let msg = try await hello()
```

Suspend functional types are also exported as Swift async counterparts [1][2]. This is part of Swift export (as opposed to legacy Objective-C interop, where suspend functions are exposed via completion handlers) [4].

By default, calling an exported suspend function from Swift uses a coroutine context based on `Dispatchers.Default`; you can switch dispatchers with `withContext()` in Kotlin [5].

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 110–140; Interoperability with Swift using Swift export > Mappings
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

- f1: **correct** — Maps suspend functions and functional types to Swift async counterparts.
- f2: **correct** — Shows let msg = try await hello().
- f3: **partial** — Distinguishes Swift export from Objective-C completion handlers, but omits Alpha status.

Material error: False. Citations: supported. Correct mapping and bridge distinction, but omits Alpha status.

Reference excerpt: [reports/task1/sources/native-swift-export.txt](../../reports/task1/sources/native-swift-export.txt).

### q16 — rag-rerank-rewrite

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: Kotlin declare existing JavaScript API external without implementation

Use `external` declarations. Kotlin assumes the implementation is provided externally (by JavaScript or an npm dependency) and generates no code for it, so `external` declarations cannot have a body [2].

```kotlin
external fun alert(message: Any?): Unit

external class Node {
    val firstChild: Node
    fun append(child: Node): Node
    fun removeChild(child: Node): Node
}

external val window: Window

external interface JQueryAjaxSettings {
    var async: Boolean
    var cache: Boolean
    var complete: (JQueryXHR, String) -> Unit
}
```

Key rules:

- `external` is only allowed on package-level declarations; you can't declare an external member of a non-external class [2].
- The modifier is inherited by nested declarations, so members inside an `external class` need no `external` themselves [2][4].
- `external interface` describes the shape of a JS object; it's compile-time only, with no runtime type information [3][5].
- In Kotlin/Wasm, external interfaces typically extend `JsAny`, and external classes do too [3][4]. This `JsAny` bound is Wasm-specific; on Kotlin/JS the examples don't use it [2][5].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 47–67; Use JavaScript code from Kotlin > external modifier
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [5] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 126–166; Use JavaScript code from Kotlin > external modifier > external interfaces

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **correct** — Explicitly says no Kotlin body and no generated JS implementation.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Rewriting plus reranking retrieves the designated JS section and covers no-body/no-code rules, package scope and inherited external. Optional definedExternally guidance is absent.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-filter-rewrite

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: Kotlin declare existing JavaScript API external without implementation

Use the `external` modifier on a Kotlin declaration whose implementation lives in JavaScript. The compiler assumes the implementation is provided externally and doesn't generate JS code for it [5].

Key rules:
- `external` is only allowed on package-level declarations (or nested inside an `external` class/interface); you can't declare an external member of a non-external class [5].
- External declarations can't have a body [5].
- For functions, classes, properties, and interfaces that describe a JS API, mark them `external` [5].

Example:
```kotlin
external fun alert(message: Any?): Unit

external class Node {
    val firstChild: Node
    fun append(child: Node): Node
}

external val window: Window
```
Note the member declarations inside `Node` don't need `external` — the modifier is inherited by nested declarations [5].

For describing a plain JS object's shape, use an `external interface` [2][4]:
```kotlin
external interface User {
    val name: String
    val age: Int
}

external fun createUser(name: String, age: Int): User
```
External interfaces are a compile-time-only concept without runtime type information [2], which is useful for expressing structural types like JS settings objects [4].

For class constructors, `external class Rectangle(height: Double, width: Double) : JsAny { ... }` maps to a JS class [3].

Caveat: the exact base type to extend (e.g., `JsAny` in Wasm vs. JS target specifics) and available syntax vary by target and version; the sources are normalized snapshots with unknown release/availability, so verify against the docs for your Kotlin version and target.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [4] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 126–166; Use JavaScript code from Kotlin > external modifier > external interfaces
- [5] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 47–67; Use JavaScript code from Kotlin > external modifier

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **correct** — Explicitly says no Kotlin body and no generated JS implementation.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Covers the designated JS external rules and identifies JsAny as target-specific. Optional definedExternally guidance is absent.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-rerank-filter-rewrite

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: Kotlin declare existing JavaScript API external without implementation

Use an `external` declaration — Kotlin then treats the implementation as provided externally (by JS or an npm dependency) and doesn't generate JS for it [2].

```kotlin
external fun alert(message: Any?): Unit

external class Rectangle(height: Double, width: Double) {
    val height: Double
    val width: Double
    fun area(): Double
}

external val window: Window

external interface User {
    val name: String
    val age: Int
}
```

Key rules:
- `external` declarations can't have a body [2].
- Nested declarations of an `external` class inherit the modifier — don't repeat it on members [2][4].
- `external` is only allowed on package-level declarations; you can't declare an `external` member of a non-`external` class [2].
- For Wasm, external interfaces describing JS object shapes typically extend `JsAny` [3]; on JS, external interfaces are used to describe duck-typed JS objects [5].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 47–67; Use JavaScript code from Kotlin > external modifier
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [5] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 126–166; Use JavaScript code from Kotlin > external modifier > external interfaces

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **correct** — Explicitly says no Kotlin body and no generated JS implementation.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Correctly separates Wasm examples from Kotlin/JS and covers package/nesting/no-body rules. Optional definedExternally guidance is absent.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use Kotlin **`external` declarations** to describe an existing JavaScript API’s shape without providing an implementation. External JavaScript code isn’t visible to Kotlin by default, so you describe its API this way [1].

Example:

```kotlin
external fun alert(message: String)

external class Console {
    fun log(message: String)
}

fun main() {
    alert("Hello from Kotlin")
    Console().log("Hi")
}
```

For callable JavaScript objects, you can annotate a member function of an external declaration with `@nativeInvoke` so Kotlin calls it as a JavaScript function [4]:

```kotlin
import kotlin.js.nativeInvoke

@OptIn(ExperimentalWasmJsInterop::class)
external class JsAction {
    @nativeInvoke
    operator fun invoke(data: String)
}
```

The reference material does not show a general `external` syntax example beyond stating that external declarations describe JavaScript APIs [1]; this example follows that description but is not directly quoted from the provided sources. Availability and exact syntax can vary by Kotlin version and target; the sources do not establish general version compatibility.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), lines 79–98; Use Kotlin code from JavaScript > Package structure > @JsExport annotation
- [3] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), lines 170–194; Use Kotlin code from JavaScript > Package structure > @JsNoRuntime annotation
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [5] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 705–719; What's new in Kotlin 2.3.20 > Kotlin/JS > Implementing Kotlin interfaces from JavaScript/TypeScript > How to enable

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **partial** — Describes an externally implemented API and/or bodyless declarations, but does not fully explain the compiler no-implementation rule.
- f3: **missing** — Omits package-level scope, inherited external and definedExternally guidance.

Material error: False. Citations: supported. Correct external mechanism, but no explicit compiler/no-body rules or package/nesting/default-argument guidance. The optional @nativeInvoke example introduces a Wasm-specific feature without clearly separating targets; its availability warning is generic.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-rerank

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

In Kotlin/Wasm, you describe an existing JavaScript API using **`external` declarations** — they declare the shape/signature without providing a body, so Kotlin can call the JavaScript code [1][2].

Common forms:

**External function**
```kotlin
external fun greet(name: String)
```
JavaScript:
```js
function greet (name) {
    console.log("Hello, " + name + "!");
}
```
External functions have no bodies and are called like regular Kotlin functions [2].

**External class**
```kotlin
external class Rectangle(height: Double, width: Double) : JsAny {
    val height: Double
    val width: Double
    fun area(): Double
}
```
All declarations inside an external class are implicitly external [4].

**External interface** (for describing the shape of a JavaScript object)
```kotlin
external interface User : JsAny {
    val name: String
    val age: Int
}

external fun createUser(name: String, age: Int): User
```
External interfaces have no runtime type information and are compile-time-only [5].

**Importing from a JavaScript module**

By default, external declarations correspond to the JavaScript global scope. To import from a specific module, annotate the file with `@JsModule`:
```kotlin
@file:JsModule("./users.mjs")

external val maxUsers: Int

external class User : JsAny {
    constructor(username: String)
    val username: String
}
```
All external declarations in the file are then imported from that module [3].

**Version caveats:** The cited docs are Kotlin/Wasm JS interop snapshots with unknown release/availability; feature availability may differ across versions and platforms (e.g., Kotlin/JS has its own `external` semantics) [1][2][3][4][5].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 17–33; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript functions
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 180–206; Interoperability with JavaScript > Use JavaScript code in Kotlin > JavaScript modules
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **partial** — Describes an externally implemented API and/or bodyless declarations, but does not fully explain the compiler no-implementation rule.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Accurately scopes examples to Wasm, so it avoids making JsAny universal, but it does not supply the designated Kotlin/JS rules. No package-level or definedExternally guidance.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-filter

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use **external declarations**: they describe the JavaScript API's shape to the Kotlin compiler but have no Kotlin body [1].

Example — a global JS function:

```kotlin
external fun greet(name: String)

fun main() {
    greet("Alice")
}
```

For a JS module, annotate the file with `@JsModule` so its externals are imported from that module [4]:

```kotlin
@file:JsModule("./users.mjs")

external val maxUsers: Int

external class User : JsAny {
    constructor(username: String)
    val username: String
}
```

For a callable JS object, `@nativeInvoke` on an external class/interface member maps Kotlin calls to direct JS calls — but it's a temporary, experimental solution that currently emits a compiler warning [2].

Version caveat: the snapshot doesn't state release/availability, so exact syntax and experimental status may differ by version [1][2][4].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 17–33; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript functions
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 180–206; Interoperability with JavaScript > Use JavaScript code in Kotlin > JavaScript modules
- [5] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 1–5; Use JavaScript code from Kotlin

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **partial** — Describes an externally implemented API and/or bodyless declarations, but does not fully explain the compiler no-implementation rule.
- f3: **missing** — Omits package-level scope, inherited external and definedExternally guidance.

Material error: True. Citations: supported. Presents the Wasm JsAny module example as general Kotlin/JS guidance without identifying its target. The generic version caveat does not explain this platform restriction.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-rerank-filter

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use `external` declarations: they describe an existing JavaScript API's shape to Kotlin and have no Kotlin bodies [1][2].

```kotlin
external fun greet(name: String)          // existing JS function

external class Rectangle(height: Double, width: Double) : JsAny {
    val height: Double
    val width: Double
    fun area(): Double                    // members are implicitly external
}

external interface User : JsAny {         // describes a JS object shape
    val name: String
    val age: Int
}
```

So instead of implementing the API, you only declare it, then call it like ordinary Kotlin code [2]. By default, external declarations refer to the JavaScript global scope; annotate the file with `@JsModule("...")` to import declarations from a specific module instead [3].

Note: this guidance is from the Kotlin/Wasm JS interop docs; release and availability are unknown in the provided snapshot, so version compatibility isn't guaranteed [1][2][3].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 17–33; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript functions
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 180–206; Interoperability with JavaScript > Use JavaScript code in Kotlin > JavaScript modules
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **partial** — Describes an externally implemented API and/or bodyless declarations, but does not fully explain the compiler no-implementation rule.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Clearly notes that its source and examples are Wasm-specific; core external guidance is useful, but package-level and definedExternally rules are omitted.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q16 — rag-rewrite

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Search query: Kotlin declare existing JavaScript API external without implementation

Use `external` declarations to describe the existing JavaScript API's shape to Kotlin; the implementation stays in JavaScript [1].

Examples:

- External function:
```kotlin
external fun createUser(name: String, age: Int): User
```
[2]

- External interface (compile-time-only shape description, no runtime type info):
```kotlin
external interface User : JsAny {
    val name: String
    val age: Int
}
```
[2]

- External class:
```kotlin
external class Rectangle(height: Double, width: Double) : JsAny {
    val height: Double
    val width: Double
    fun area(): Double
}
```
All declarations inside an external class are implicitly external [3].

Caveat: The `JsAny` supertype and interface restrictions shown are from the Kotlin/Wasm JS interop docs [2][3]. The general Kotlin/JS external-interface docs describe external interfaces without requiring `JsAny` [4], so exact syntax may depend on the target (Wasm vs. JS) and version.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 72–89; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External interfaces
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 47–70; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript classes
- [4] [Use JavaScript code from Kotlin](https://kotlinlang.org/docs/js-interop.html), lines 126–166; Use JavaScript code from Kotlin > external modifier > external interfaces
- [5] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), lines 79–98; Use Kotlin code from JavaScript > Package structure > @JsExport annotation

- f1: **correct** — Identifies external declarations for APIs implemented in JavaScript.
- f2: **partial** — Describes an externally implemented API and/or bodyless declarations, but does not fully explain the compiler no-implementation rule.
- f3: **partial** — Covers some package/nested external behavior but omits definedExternally.

Material error: False. Citations: supported. Correct mechanism and explicit Wasm/JS distinction. Nested external behavior is covered, but no package-level or optional-argument rule.

Reference excerpt: [reports/task1/sources/js-interop.txt](../../reports/task1/sources/js-interop.txt).

### q17 — rag-filter-rewrite

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: Kotlin Wasm hold reference Kotlin object JavaScript side interoperability

Use `JsReference<T>`: a Kotlin object is passed to JavaScript as an opaque reference via `toJsReference()`, and JavaScript must call back into exported Kotlin functions to operate on it with `get()` [1].

Example:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

On the JavaScript side the reference behaves like an empty frozen object and isn't directly usable; you need exported functions like `setUserName` that unwrap it via `get()` [1]. This differs from Kotlin/JS, which allows non-external Kotlin class types in external declarations and doesn't require `JsReference` for this purpose [2].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete pattern, with correct target distinction and no unsupported direct property access.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-rerank-filter-rewrite

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: Kotlin Wasm hold reference Kotlin object JavaScript side interoperability

Use `JsReference<T>` together with `toJsReference()` and `get()` [1].

- Create a reference from a Kotlin object with `toJsReference()`, which produces `JsReference<User>` in this example [1].
- Pass or return that `JsReference` to JavaScript; it behaves like an empty frozen JavaScript object and is an opaque reference to the Kotlin object [1].
- Access the underlying Kotlin object only by unwrapping it in Kotlin with `get()` [1].
- In Kotlin/Wasm type correspondence, `JsReference` is listed as an opaque reference to a Kotlin object [3].

Small example from the reference data:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

The reference data does not specify a release version or availability, so version caveats are unknown [1].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete pattern with source-supported opaque-reference semantics.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Use `JsReference<T>`: it's an opaque handle to a Kotlin value that JavaScript can store and pass back, but not inspect directly [1].

On the Kotlin side, create one with `toJsReference()` and return it to JS:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}
```

In JavaScript this behaves like an empty frozen object — you can't touch the Kotlin object's fields through it [1]. To actually operate on the object, export a Kotlin function that takes the `JsReference` and unwraps it with `get()`:

```kotlin
@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

From JS:

```js
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

So the JS side only ever holds the `JsReference`, and all real access goes through exported Kotlin functions that call `get()` [1][5].

Version note: this comes from the current Kotlin/Wasm JS-interop docs without a stated release, so exact API details may vary across Kotlin versions.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 99–117; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External objects
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 164–176; Interoperability with JavaScript > Use JavaScript code in Kotlin > Kotlin functions with JavaScript code
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete source-supported handle creation and access pattern, including JS usage.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-rerank

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Use `JsReference<T>`: Kotlin/Wasm can pass Kotlin values to JavaScript as opaque references using the `JsReference` type [1]. JavaScript cannot directly access these references—they behave like empty frozen JavaScript objects—so you unwrap them on the Kotlin side via `get()` [1].

Example:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

On the JavaScript side:

```js
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

This is a Kotlin/Wasm-specific mechanism: Kotlin/JS, by contrast, permits non-external Kotlin class types in external declarations without requiring `JsReference` [2]. Version/release support is not specified in the retrieved data.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 164–176; Interoperability with JavaScript > Use JavaScript code in Kotlin > Kotlin functions with JavaScript code

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete pattern, with correct Kotlin/Wasm versus Kotlin/JS distinction.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-filter

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Kotlin/Wasm exposes a Kotlin object to JavaScript as an opaque reference using the `JsReference<T>` type [1]. You create one with `toJsReference()` on the Kotlin side and return it to JavaScript.

On the JavaScript side, the reference is not directly usable — it behaves like an empty frozen JavaScript object [1]. To operate on it, you export additional Kotlin functions that accept the `JsReference<T>` and call `get()` to unwrap the underlying Kotlin object [1].

Example (from the docs):

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

JavaScript usage [2]:

```js
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

In the type correspondence table, `JsReference` maps to an "opaque reference to Kotlin object" [4]. Note this is specific to Kotlin/Wasm; Kotlin/JS differs in that it allows non-external Kotlin class types in external declarations [3].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete source-supported pattern; retained evidence includes the JS usage example and type correspondence.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-rerank-filter

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Kotlin/Wasm uses the `JsReference<T>` type to hold an opaque reference to a Kotlin object from the JavaScript side [1][3].

To create one, call `.toJsReference()` on a Kotlin object and expose it to JavaScript via a `@JsExport` function [1]:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}
```

On the JavaScript side, a `JsReference` behaves like an empty frozen JavaScript object — you cannot inspect or mutate it directly [1]. To interact with the underlying Kotlin object, export additional functions that accept the reference and unwrap it with `.get()` [1]:

```kotlin
@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

From JavaScript:

```js
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

In the WASM-to-JS type correspondence table, `JsReference` maps to "Opaque reference to Kotlin object" [3].

**Version caveat:** these docs have an unknown release and availability; the `JsReference` API may differ across Kotlin versions, so verify against your target version.

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete pattern, with clear separation between the JS handle and Kotlin object access.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-rewrite

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: Kotlin Wasm hold reference Kotlin object JavaScript side interoperability

Use `JsReference<T>`: Kotlin objects passed to JavaScript are wrapped as opaque references that behave like empty frozen JavaScript objects, and you unwrap them back in Kotlin via `get()` [1].

Example from the docs: expose a Kotlin class with `@JsExport`, return `JsReference<User>` from `toJsReference()`, then accept that reference in another exported function and call `get()` to access the underlying Kotlin object [1]:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

Key points:
- `JsReference` corresponds to an "Opaque reference to Kotlin object" in the interop type table [3].
- The reference is not directly usable in JavaScript — it looks like an empty frozen JS object; you must export functions to operate on it [1].
- Kotlin/Wasm requires `JsReference<>` to pass Kotlin objects to JavaScript, unlike Kotlin/JS [2].

Caveats: the provided snapshot has unknown release/availability, and Kotlin/Wasm is Beta, so behavior may change [4].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 1–7; Interoperability with JavaScript
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 99–117; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External objects

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. All core facts correct. Prose loosely says expose a class with @JsExport; the actual code correctly exports factory/helper functions, not the class.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q17 — rag-rerank-rewrite

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Search query: Kotlin Wasm hold reference Kotlin object JavaScript side interoperability

Kotlin/Wasm uses the `JsReference` type to hold an opaque reference to a Kotlin object on the JavaScript side [1][3].

A `JsReference` wraps a Kotlin object so it can be passed to JavaScript as an opaque handle. On the JavaScript side, it behaves like an empty frozen object and can't be used directly — you must unwrap it on the Kotlin side via `get()` [1].

Example:

```kotlin
class User(var name: String)

@JsExport
fun createUser(name: String): JsReference<User> {
    return User(name).toJsReference()
}

@JsExport
fun setUserName(user: JsReference<User>, name: String) {
    user.get().name = name
}
```

Per the type correspondence table, `JsReference` corresponds to "Opaque reference to Kotlin object" [3]. Note this is a Kotlin/Wasm mechanism; Kotlin/JS instead allows non-external Kotlin class types in external declarations [2]. Kotlin/Wasm is Beta and may change [4].

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 393–410; Interoperability with JavaScript > Kotlin/Wasm and Kotlin/JS interoperability differences
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 280–298; Interoperability with JavaScript > Type correspondence
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 1–7; Interoperability with JavaScript
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), lines 99–117; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External objects

- f1: **correct** — Identifies JsReference<T> as an opaque handle to a Kotlin object.
- f2: **correct** — Creates the reference with toJsReference() and returns it from @JsExport createUser.
- f3: **correct** — Explains that JS cannot inspect the object and demonstrates exported setUserName unwrapping with get().

Material error: False. Citations: supported. Complete pattern and correct target distinction; Beta caveat is supported by the introductory chunk.

Reference excerpt: [reports/task1/sources/wasm-js-interop.txt](../../reports/task1/sources/wasm-js-interop.txt).

### q18 — rag-rerank-filter-rewrite

When did the common Kotlin UUID API become stable?

Search query: Kotlin UUID API stability version

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became **Stable in Kotlin 2.4.0** [1][3][5]. It was introduced in Kotlin 2.0.20 and gradually improved while Experimental, then stabilized in 2.4.0 [1].

Caveat: the V4 and V7 UUID generation functions remain **Experimental** and still require opt-in [1]. Also, per the snapshot guidance, the release version listed is not a guarantee of compatibility for a given snippet.

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [5] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1–27; What's new in Kotlin 2.4.0

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **correct** — States experimental introduction in Kotlin 2.0.20, distinct from stability.

Material error: False. Citations: supported. All expected facts covered with direct support in [1].

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag

When did the common Kotlin UUID API become stable?

Search query: When did the common Kotlin UUID API become stable?

The common `kotlin.uuid.Uuid` API became Stable in **Kotlin 2.4.0**. The only exceptions are the functions for generating V4 and V7 UUIDs, which remain Experimental and still require opt-in [1].

Previously it was Experimental: introduced in 2.0.20, then improved across 2.1.20 and 2.3.0 (parsing, comparability, V4/V7 generation) [2][3][4].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [3] [What's new in Kotlin 2.1.20](https://kotlinlang.org/docs/whatsnew2120.html), lines 323–365; What's new in Kotlin 2.1.20 > Standard library > Changes in UUID parsing, formatting, and comparability
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 658–684; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing > New functions to generate v4 and v7 UUIDs

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **correct** — States experimental introduction in Kotlin 2.0.20, distinct from stability.

Material error: False. Citations: supported. Correct stabilization release and exceptions, with experimental introduction distinguished from later improvements.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-rerank

When did the common Kotlin UUID API become stable?

Search query: When did the common Kotlin UUID API become stable?

The common Kotlin `kotlin.uuid.Uuid` API became **Stable in Kotlin 2.4.0** [1][3]. The only exceptions are the V4 and V7 UUID generation functions, which remain Experimental and require opt-in [1].

Note: Kotlin 2.0.20 introduced the experimental UUID class, and later 2.x releases improved it incrementally before it graduated to Stable in 2.4.0 [1].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [4] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1–27; What's new in Kotlin 2.4.0
- [5] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 582–584; What's new in Kotlin 2.3.0 > Standard library

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **correct** — States experimental introduction in Kotlin 2.0.20, distinct from stability.

Material error: False. Citations: supported. All facts covered concisely, including experimental introduction in 2.0.20.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-filter

When did the common Kotlin UUID API become stable?

Search query: When did the common Kotlin UUID API become stable?

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became Stable in **Kotlin 2.4.0** [1][4]. Exception: the V4 and V7 UUID generation functions remain Experimental and still require opt-in [1].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 582–584; What's new in Kotlin 2.3.0 > Standard library
- [4] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [5] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 21–25; What's new in Kotlin 2.3.0

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **missing** — Does not discuss experimental introduction.

Material error: False. Citations: supported. Answers the core stability question and exceptions correctly, but omits the 2.0.20 introduction/history fact.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-rerank-filter

When did the common Kotlin UUID API become stable?

Search query: When did the common Kotlin UUID API become stable?

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became Stable in **Kotlin 2.4.0** [1][3]. It was previously Experimental. The only parts that remain Experimental and require opt-in are the V4 and V7 UUID generation functions [1].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [4] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1–27; What's new in Kotlin 2.4.0
- [5] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 582–584; What's new in Kotlin 2.3.0 > Standard library

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **partial** — Mentions earlier Experimental status but omits the 2.0.20 introduction.

Material error: False. Citations: supported. Correct stability and exceptions; says previously Experimental but omits the introduction release.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-rewrite

When did the common Kotlin UUID API become stable?

Search query: Kotlin UUID API stability version

The common Kotlin `kotlin.uuid.Uuid` API became **Stable in Kotlin 2.4.0** [1]. The only exceptions are the V4 and V7 generation functions, which remain Experimental and require opt-in [1].

Previously, in Kotlin 2.3.0, the UUID support was still Experimental but planned for future stabilization [2].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 658–684; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing > New functions to generate v4 and v7 UUIDs
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 686–708; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing > Support for generating v7 UUIDs for specific timestamps

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **partial** — Mentions earlier Experimental status but omits the 2.0.20 introduction.

Material error: False. Citations: supported. Correct stability and exceptions; mentions experimental status in 2.3.0 but not introduction in 2.0.20.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-rerank-rewrite

When did the common Kotlin UUID API become stable?

Search query: Kotlin UUID API stability version

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became **Stable in Kotlin 2.4.0** [1][3]. The only parts that remain Experimental and still require opt-in are the functions for generating **V4 and V7 UUIDs** [1].

For context: the API was introduced experimentally in Kotlin 2.0.20, and Kotlin 2.3.0 added V4/V7 generation and null-returning parsing while still Experimental [1][4].

Note the usual caveat: this reflects the Kotlin 2.4.0 documentation snapshot; release versions are not a guarantee of feature availability in every specific build or toolchain.

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [5] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1–27; What's new in Kotlin 2.4.0

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **correct** — States experimental introduction in Kotlin 2.0.20, distinct from stability.

Material error: False. Citations: supported. All expected facts covered, with introduction distinguished from stabilization.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q18 — rag-filter-rewrite

When did the common Kotlin UUID API become stable?

Search query: Kotlin UUID API stability version

The common Kotlin standard library UUID API (`kotlin.uuid.Uuid`) became **Stable in Kotlin 2.4.0** [1][4].

Caveat: the V4 and V7 UUID generation functions remain **Experimental** and still require opt-in [1]. Earlier, this API was introduced in Kotlin 2.0.20 and was Experimental as of Kotlin 2.3.0 [2].

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [3] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 366–368; What's new in Kotlin 2.4.0 > Standard library
- [4] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 35–59; What's new in Kotlin 2.4.0 > New features
- [5] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1–27; What's new in Kotlin 2.4.0

- f1: **correct** — States Stable in Kotlin 2.4.0.
- f2: **correct** — Explicitly retains Experimental/opt-in status for V4 and V7 generation.
- f3: **correct** — States experimental introduction in Kotlin 2.0.20, distinct from stability.

Material error: False. Citations: supported. All expected facts covered, including 2.0.20 introduction and later experimental status.

Reference excerpt: [reports/task1/sources/whatsnew24.txt](../../reports/task1/sources/whatsnew24.txt).

### q19 — rag

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Kotlin 2.4.20 added support for exporting suspending lambda expressions as JavaScript `async` functions in Kotlin/JS [1][2]. Previously, declarations containing suspending lambdas could not be exported from Kotlin/JS libraries [1].

The compiler now bridges Kotlin `suspend` functions and JavaScript's native `async`/`await` model, which is useful for mixed Kotlin/TypeScript codebases [1]. To enable it, add the `-Xsuspend-lambda-exporting` compiler option, then mark declarations with `@JsExport` [1][3]:

```kotlin
// build.gradle.kts
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

From TypeScript, the suspending lambda appears as a regular async function [3]:

```ts
const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
```

Note this is distinct from the earlier Kotlin 2.3.0 change that allowed exporting suspend functions themselves with `@JsExport` [4].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 396–402; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [5] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), lines 133–159; Use Kotlin code from JavaScript > Package structure > @JsExport annotation > Export suspending lambdas

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct; correctly distinguishes earlier suspend-function export from lambda export.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-rerank

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

In Kotlin 2.4.20, you can export suspending lambda expressions as JavaScript `async` functions from Kotlin/JS. The compiler automatically bridges Kotlin's `suspend` functions to JavaScript's native `async`/`await` model, which is useful for mixed Kotlin/TypeScript codebases [1][3].

**Enable the feature** with the compiler option `-Xsuspend-lambda-exporting` [1]:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

**Mark the declaration** with `@JsExport` [3]:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

On the TypeScript side, the suspending lambda appears as a regular async function [3]:

```typescript
// TypeScript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
```

Previously there was no way to export declarations containing suspending lambdas from Kotlin/JS libraries [1]. This feature is listed among the 2.4.20 release highlights for Kotlin/JS [2][5]; note that the source describes the listed pre-stable features (including this one) as Beta, Alpha, or Experimental status [4].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts and code pattern follow the saved release notes.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-filter

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Kotlin 2.4.20 lets you export suspending lambda expressions from Kotlin/JS as native JavaScript `async` functions. Previously, declarations containing suspending lambdas couldn't be exported from Kotlin/JS libraries; now the compiler automatically bridges Kotlin suspend functions and JavaScript's `async`/`await` model, which is useful for mixed Kotlin/TypeScript codebases [1].

It's a pre-stable feature, listed among the release's Beta/Alpha/Experimental features [4].

**Enabling it** — add the compiler option to your `build.gradle.kts` [1]:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

**Usage** — mark the declaration with `@JsExport`. On the TypeScript side, the suspending lambda appears as a regular async function [3]:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

```typescript
// TypeScript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
```

Caveat: the reference material comes from the Kotlin 2.4.20 release notes [2][5], and release version alone isn't a guarantee of feature availability or compatibility in other versions.

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct; retained release-note sections include the flag and usage.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-rerank-filter

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Kotlin 2.4.20 added support for exporting suspending lambda expressions as JavaScript `async` functions [1][2].

Previously, declarations containing suspending lambdas could not be exported from Kotlin/JS libraries. Now the compiler automatically bridges Kotlin's `suspend` functions and JavaScript's native `async`/`await` model, which helps mixed Kotlin/TypeScript codebases [1].

To enable it, add this compiler option:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```
[1]

Then mark the declaration with `@JsExport`. On the TypeScript side, the suspending lambda appears as a regular `async` function [3]:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

```typescript
// TypeScript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
```
[3]

Note: this is documented specifically for Kotlin 2.4.20, where it is listed among the pre-stable features (Beta/Alpha/Experimental status) [4]. The release version is not a guarantee of feature availability in other versions.

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct with direct source support.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-rewrite

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: Kotlin 2.4.20 suspending lambdas export JavaScript changes

Kotlin 2.4.20 adds support for exporting suspending lambda expressions as JavaScript `async` functions in Kotlin/JS [1][2]. Previously, declarations containing suspending lambdas could not be exported from Kotlin/JS libraries [1].

The compiler automatically bridges Kotlin `suspend` functions with JavaScript's native `async`/`await` model, which is useful for mixed Kotlin/TypeScript codebases [1]. This feature is listed among the pre-stable features (Beta/Alpha/Experimental status) in the release [5].

To enable it, add the `-Xsuspend-lambda-exporting` compiler option in `build.gradle.kts` [1][4]:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

Then mark declarations with `@JsExport` [3][4]:

```kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

From TypeScript, the suspending lambda appears as a regular async function [3]:

```typescript
const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
```

Note: the sources describe this as a Kotlin 2.4.20 feature, but availability details should be checked in the individual feature status [1][5].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), lines 133–159; Use Kotlin code from JavaScript > Package structure > @JsExport annotation > Export suspending lambdas
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct; mentions the pre-stable status without guessing a specific maturity level.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-rerank-rewrite

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: Kotlin 2.4.20 suspending lambdas export JavaScript changes

Kotlin 2.4.20 added support for exporting suspending lambdas as JavaScript `async` functions in Kotlin/JS [1][2]. Previously, declarations containing suspending lambdas couldn't be exported from Kotlin/JS libraries; now the compiler bridges Kotlin's `suspend` functions and JavaScript's native `async`/`await` model [1].

The feature is enabled with the `-Xsuspend-lambda-exporting` compiler option [1]:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

Then mark declarations with `@JsExport`. On the TypeScript side, the suspending lambda appears as a regular async function [3]:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

```typescript
// TypeScript
const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
```

Note that the source describes this as one of the pre-stable features in the release (Beta/Alpha/Experimental statuses are listed together in the "New features" section), and it's listed under Kotlin/JS improvements [2][4]. The release version (2.4.20) does not by itself guarantee snippet compatibility across other versions.

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct with the exact flag and export pattern.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-filter-rewrite

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: Kotlin 2.4.20 suspending lambdas export JavaScript changes

In Kotlin 2.4.20, suspending lambda expressions can now be exported as JavaScript `async` functions [1]. Previously, declarations containing suspending lambdas could not be exported from Kotlin/JS libraries [1]. The compiler now bridges Kotlin `suspend` functions and JavaScript's native `async`/`await` model, which is useful for mixed Kotlin/TypeScript codebases [1].

To enable it, add the `-Xsuspend-lambda-exporting` compiler option [1]:

```kotlin
kotlin {
    js {
        compilations.all {
            compileTaskProvider.configure {
                compilerOptions {
                    freeCompilerArgs.add("-Xsuspend-lambda-exporting")
                }
            }
        }
    }
}
```

Then mark declarations with `@JsExport` [3]:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

On the TypeScript side, the suspending lambda appears as a regular async function [3]:

```typescript
const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
```

This is listed as one of the Kotlin/JS improvements in 2.4.20 [2][4][5]. Note that availability status is per-feature in the source, and the release version does not guarantee snippet compatibility [1][5].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct with the exact flag and export pattern.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q19 — rag-rerank-filter-rewrite

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Search query: Kotlin 2.4.20 suspending lambdas export JavaScript changes

Kotlin 2.4.20 added support for exporting suspending lambda expressions as JavaScript async functions [1][2][4][5].

Details:
- Previously, declarations containing suspending lambdas could not be exported from Kotlin/JS libraries; the compiler now bridges Kotlin `suspend` functions and JavaScript's `async`/`await` model [1].
- Enabling it requires the compiler option `-Xsuspend-lambda-exporting` in `build.gradle.kts` [1].
- Export the relevant declarations with `@JsExport`; on the TypeScript side, the suspending lambda appears as a regular async function [3].

Example from the docs:

```kotlin
// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

```typescript
// TypeScript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
```

This is useful for mixed Kotlin/TypeScript codebases [1]. The sources list it among pre-stable features (Beta, Alpha, or Experimental status) [4].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 1–23; What's new in Kotlin 2.4.20

- f1: **correct** — Identifies Kotlin 2.4.20 support for suspending lambdas exported as JS async functions.
- f2: **correct** — Names the exact -Xsuspend-lambda-exporting flag.
- f3: **correct** — Uses @JsExport and explains/shows the async-await bridge.

Material error: False. Citations: supported. All expected facts correct, concisely states flag and shows export usage.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-rerank

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Cross-language inheritance in Swift export is the Kotlin 2.4.20 feature that permits Swift implementations to extend exported Kotlin types [1].

It's designed for the reverse import pattern: you declare a Kotlin superclass and interface, then implement the interface in Swift and pass the Swift object to Kotlin functions accepting that interface [1]. It's useful for integrating pure Swift libraries that can't be directly imported into Kotlin [1].

Example (Kotlin side) [1]:

```kotlin
interface CryptoProvider {
    fun hashMD5(input: String): String
}

fun processHash(provider: CryptoProvider, input: String): String = provider.hashMD5(input)

open class SwiftBase
```

Caveat: this is from the Kotlin 2.4.20 "What's new" snapshot; per the source, check individual feature status for availability, and release version is not a snippet compatibility guarantee [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [4] [What's new in Kotlin 2.2.20](https://kotlinlang.org/docs/whatsnew2220.html), lines 432–452; What's new in Kotlin 2.2.20 > Kotlin Multiplatform > Swift export available by default
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. Correct feature and reverse-import pattern; Kotlin example shows open base class and interface-dispatch function.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-filter

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

The feature is **cross-language inheritance in Swift export**, introduced in Kotlin 2.4.20 [1]. It lets Swift types inherit from exported Kotlin superclasses and implement exported Kotlin interfaces, supporting the "reverse import" pattern where Kotlin declares a contract and Swift provides the implementation [1].

For example, Kotlin declares an interface and an open base class:

```kotlin
// Kotlin
interface CryptoProvider {
    fun hashMD5(input: String): String
}

open class SwiftBase
```

Then Swift can subclass `SwiftBase` and implement `CryptoProvider`, passing the Swift object to Kotlin functions that accept the interface [1].

Caveat: availability details are governed by the individual feature status in the source; the release version itself is not a compatibility guarantee [1][2].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. Correct open base/interface pattern and passing the Swift instance to Kotlin; dispatch is conveyed by Swift implementing the contract.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-rerank-filter

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

The feature is **cross-language inheritance in Swift export**, introduced in Kotlin 2.4.20 [1]. It lets Swift code inherit from exported Kotlin open classes (and implement Kotlin interfaces), supporting the reverse import pattern where a Kotlin contract is implemented on the Swift side [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **missing** — Does not describe passing the Swift instance to Kotlin for dispatch.

Material error: False. Citations: supported. Names the correct feature and open-class/interface pattern, but omits passing the Swift instance back to Kotlin for dispatch.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-rewrite

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Kotlin 2.4.20 Swift extend exported Kotlin types interoperability feature

Cross-language inheritance in Swift export permits Swift implementations to extend exported Kotlin types [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [4] [What's new in Kotlin 2.2.20](https://kotlinlang.org/docs/whatsnew2220.html), lines 432–452; What's new in Kotlin 2.2.20 > Kotlin Multiplatform > Swift export available by default
- [5] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 588–590; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **missing** — Does not explain the superclass/interface pattern.
- f3: **missing** — Does not describe passing the Swift instance to Kotlin for dispatch.

Material error: False. Citations: supported. Correctly answers which feature in one sentence, but omits the richer expected pattern and dispatch details. This is incompleteness against the suite, not a wrong direct answer.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-rerank-rewrite

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Kotlin 2.4.20 Swift extend exported Kotlin types interoperability feature

The feature is **cross-language inheritance in Swift export**, introduced in Kotlin 2.4.20. It allows a Swift implementation to inherit from an exported Kotlin superclass and implement a Kotlin interface, with the Swift object then passed back to Kotlin functions accepting that interface. This is described as useful for the reverse import pattern, e.g. using pure Swift libraries such as CryptoKit [1].

Note that Swift export itself is currently in Alpha [5], and the feature's availability should be checked in the source's individual feature status; release version alone isn't a compatibility guarantee [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [4] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 300–300; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), lines 1–13; Interoperability with Swift using Swift export

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **partial** — Describes superclass/interface implementation but omits the open-base requirement.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. Correct inheritance/interface and pass-back pattern, but does not explicitly say the Kotlin base must be open.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-filter-rewrite

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Kotlin 2.4.20 Swift extend exported Kotlin types interoperability feature

Cross-language inheritance in Swift export, introduced in Kotlin 2.4.20, permits Swift implementations to extend exported Kotlin types [1][2]. It supports the reverse import pattern: you declare a Kotlin superclass (e.g., `open class SwiftBase`) and a Kotlin interface, then implement that interface in Swift and pass the Swift object to Kotlin functions accepting it [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. Correct feature, open superclass/interface, and passing the Swift implementation to Kotlin consumers.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag-rerank-filter-rewrite

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Kotlin 2.4.20 Swift extend exported Kotlin types interoperability feature

The feature is **cross-language inheritance in Swift export**, introduced in Kotlin 2.4.20 [1]. It lets a Swift class inherit from an exported Kotlin base class and implement exported Kotlin interfaces [1].

A typical use case is the reverse-import pattern: define a contract in Kotlin, implement it in Swift, then pass the Swift object to Kotlin functions accepting that interface [1].

Example sketch from the source:

```kotlin
// Kotlin
interface CryptoProvider {
    fun hashMD5(input: String): String
}

fun processHash(provider: CryptoProvider, input: String): String =
    provider.hashMD5(input)

open class SwiftBase
```

On the Swift side, a class can then inherit `SwiftBase` and conform to `CryptoProvider` [1].

Caveat: this is documented for Kotlin 2.4.20; the guide snapshot does not establish behavior for other versions [1].

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. All expected facts covered with a Kotlin interface-dispatch example and explicit Swift inheritance.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q20 — rag

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Search query: Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

The feature is **cross-language inheritance in Swift export** [1].

Kotlin 2.4.20 introduced support for cross-language inheritance, which lets Swift declarations inherit from exported Kotlin superclasses (and implement Kotlin interfaces). This is aimed at the reverse import pattern: define a contract in Kotlin, then provide platform-specific Swift implementations—useful when certain Swift libraries can't be imported into Kotlin directly [1].

```kotlin
interface CryptoProvider {
    fun hashMD5(input: String): String
}

fun processHash(provider: CryptoProvider, input: String): String =
    provider.hashMD5(input)

open class SwiftBase
```

On the Swift side, a class can inherit from `SwiftBase` and implement `CryptoProvider`, and the resulting Swift object can be passed to Kotlin functions like `processHash` [1].

Caveat: availability/status is a pre-stable feature in the 2.4.20 snapshot [5], and release version alone doesn't guarantee feature availability or compatibility.

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [4] [What's new in Kotlin 2.2.20](https://kotlinlang.org/docs/whatsnew2220.html), lines 432–452; What's new in Kotlin 2.2.20 > Kotlin Multiplatform > Swift export available by default
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), lines 39–59; What's new in Kotlin 2.4.20 > New features

- f1: **correct** — Names cross-language inheritance in Swift export for the requested 2.4.20 feature.
- f2: **correct** — Describes the Kotlin open superclass and interface implemented/extended by Swift.
- f3: **correct** — Passes the Swift implementation to Kotlin functions accepting its interface.

Material error: False. Citations: supported. All expected facts covered, including open base class and calling back into Swift through the Kotlin interface.

Reference excerpt: [reports/task1/sources/whatsnew2420.txt](../../reports/task1/sources/whatsnew2420.txt).

### q21 — rag-filter

What is the internal production API endpoint for my company payroll service?

Search query: What is the internal production API endpoint for my company payroll service?

I can't answer that. I have no evidence about your company's internal payroll service, and I wouldn't have a way to know a private internal endpoint regardless — retrieved references don't contain it, and internal endpoints aren't something to guess or disclose.

If you need it, check your company's internal developer documentation, service registry/discovery (e.g., Consul, Kubernetes service DNS, Backstage catalog), or ask your platform/DevOps team.



Material error: False. Citations: none. Acknowledges no evidence and invents no endpoint; adds unnecessary disclosure-policy language.

### q21 — rag-rerank-filter

What is the internal production API endpoint for my company payroll service?

Search query: What is the internal production API endpoint for my company payroll service?

I can't provide that. I have no evidence about your company's payroll service, and internal production endpoints aren't something I could supply from general Kotlin knowledge anyway — they're environment- and organization-specific.

If you're working in a Kotlin codebase, the endpoint is likely defined somewhere in configuration or constants. A few places worth checking:

```kotlin
// Common patterns
const val PAYROLL_BASE_URL = "..."          // in a Constants/Config object
BuildConfig.PAYROLL_ENDPOINT                // generated from build config
System.getenv("PAYROLL_API_URL")            // environment variable
```

Other places to look:
- `application.conf` / `application.yml` / `.env` files
- Your company's internal service registry or API catalog
- The payroll service's own repo or deployment manifests (Helm values, k8s ConfigMaps)
- Asking the team that owns the payroll service

If you can share a redacted snippet of your config setup, I can help you write the Kotlin code to read the endpoint correctly.



Material error: False. Citations: none. Acknowledges missing organization-specific evidence; configuration names are clearly illustrative, not claimed actual values.

### q21 — rag-rewrite

What is the internal production API endpoint for my company payroll service?

Search query: internal production API endpoint company payroll service

I can't help with that. Finding or revealing an internal production API endpoint for a payroll service isn't something I can assist with—that kind of information is sensitive and typically restricted to authorized personnel through official channels.

A few notes on why:

- **Internal endpoints are not public information.** Payroll systems often process sensitive personal and financial data, so their endpoints are usually guarded by access controls, network restrictions, and auditing.
- **The reference data I was given is irrelevant to this request.** It covers Kotlin inline functions, extensions visibility, and Gradle-related deprecations in Kotlin 2.2.x/2.3.x [1][2][3][4][5]. None of it addresses your company's infrastructure, and no source can supply an internal endpoint.
- **I have no access to your organization's systems.** Even setting policy aside, I have no way to know your company's internal endpoints.

If you're working on a legitimate integration task, the appropriate path is to ask your platform, DevOps, or security team, or check your internal developer portal/service catalog and API documentation. They can point you to the sanctioned endpoint and the proper auth flow.

- [1] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions
- [2] [Compatibility guide for Kotlin 2.2.x](https://kotlinlang.org/docs/compatibility-guide-22.html), lines 560–574; Compatibility guide for Kotlin 2.2.x > Tools > Deprecate kotlinArtifacts API
- [3] [Extensions](https://kotlinlang.org/docs/extensions.html), lines 478–484; Extensions > Extensions and visibility modifiers
- [4] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 325–337; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate testApi configuration
- [5] [Compatibility guide for Kotlin 2.2.x](https://kotlinlang.org/docs/compatibility-guide-22.html), lines 432–448; Compatibility guide for Kotlin 2.2.x > Tools > Deprecate KotlinCompilation.source API


Material error: False. Citations: supported. Correctly identifies irrelevant retrieved documents and no infrastructure access. Adds an unnecessary broad policy refusal.

### q21 — rag-rerank-rewrite

What is the internal production API endpoint for my company payroll service?

Search query: internal production API endpoint company payroll service

I can't help with that. I don't have access to your company's internal systems, and my instructions are to answer Kotlin questions — not to supply internal API endpoints.

The retrieved reference material is Kotlin documentation (inline functions, extensions, Gradle plugin deprecations) [1][2][3][4][5] and contains nothing about payroll services. So there's no supporting evidence here even if this were in scope.

If you're looking for that endpoint, try your internal developer portal, service catalog, or the payroll team directly.

Happy to get back to Kotlin if you have a question — e.g., why public inline functions can't touch private/internal declarations [1], or how `@PublishedApi` works around that [1].

- [1] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions
- [2] [Compatibility guide for Kotlin 2.2.x](https://kotlinlang.org/docs/compatibility-guide-22.html), lines 560–574; Compatibility guide for Kotlin 2.2.x > Tools > Deprecate kotlinArtifacts API
- [3] [Extensions](https://kotlinlang.org/docs/extensions.html), lines 478–484; Extensions > Extensions and visibility modifiers
- [4] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 325–337; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate testApi configuration
- [5] [Compatibility guide for Kotlin 2.2.x](https://kotlinlang.org/docs/compatibility-guide-22.html), lines 432–448; Compatibility guide for Kotlin 2.2.x > Tools > Deprecate KotlinCompilation.source API


Material error: False. Citations: supported. Correctly identifies missing payroll evidence. The claim that instructions prohibit such help is stronger than the actual instruction to answer Kotlin questions.

### q21 — rag-filter-rewrite

What is the internal production API endpoint for my company payroll service?

Search query: internal production API endpoint company payroll service

I can't answer that. The retrieval returned no usable evidence, so I have nothing to base an answer on — and I won't invent an endpoint or citations.

More importantly, even with retrieval, internal production endpoints for a payroll service aren't something I'd have in Kotlin documentation or general reference material. That information would live in your organization's private systems, such as:

- Internal API catalogs / service registry (e.g., Backstage, an internal developer portal)
- The payroll service's own repo (`openapi.yaml`, `application.conf`, `application.yml`, env-specific config)
- Secret/config managers (Vault, AWS SSM Parameter Store, Kubernetes ConfigMaps)
- Your platform or payroll team's runbook/wiki

If you're trying to *call* such an endpoint from Kotlin, I can help with the general shape without knowing your host:

```kotlin
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.net.URI

val client = HttpClient.newHttpClient()
val request = HttpRequest.newBuilder()
    .uri(URI.create("${System.getenv("PAYROLL_BASE_URL")}/api/v1/payroll"))
    .header("Authorization", "Bearer ${System.getenv("PAYROLL_TOKEN")}")
    .GET()
    .build()

val response = client.send(request, HttpResponse.BodyHandlers.ofString())
```

Note the endpoint is read from configuration/env, not hardcoded — that's the right pattern for internal services, and it's why I can't (and shouldn't) supply a literal URL.

If you can paste your service's OpenAPI spec or config key names, I can help you wire it up correctly.



Material error: False. Citations: none. Acknowledges empty evidence; provides explicitly illustrative client code. The /api/v1/payroll suffix is an invented example path, not an established endpoint, and should not be copied as real configuration.

### q21 — rag-rerank-filter-rewrite

What is the internal production API endpoint for my company payroll service?

Search query: internal production API endpoint company payroll service

I can’t determine that from the available evidence. No relevant reference data survived retrieval, so I don’t have a supported source for your company’s internal payroll API endpoint.

If you’re looking for it in a Kotlin project, common places to check are:

- `application.conf`, `application.yml`, or `application.properties`
- environment variables or secrets/config maps
- a dedicated `ApiClient`/`PayrollService` class
- generated API docs or service discovery configuration

Example pattern:

```kotlin
val payrollBaseUrl = System.getenv("PAYROLL_API_BASE_URL")
    ?: error("Missing PAYROLL_API_BASE_URL")
```

I can’t confirm any specific endpoint or hostname without your project’s configuration or documentation.



Material error: False. Citations: none. Clearly acknowledges empty evidence and refuses to infer a hostname; example only reads an environment variable.

### q21 — rag

What is the internal production API endpoint for my company payroll service?

Search query: What is the internal production API endpoint for my company payroll service?

I can't help with that. There is no information about your company's payroll service in the reference data provided, and I wouldn't share internal production endpoints even if there were.

If you have a Kotlin question, I'm happy to help.

- [1] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 420–422; Expected and actual declarations > What's next?
- [3] [Extensions](https://kotlinlang.org/docs/extensions.html), lines 478–484; Extensions > Extensions and visibility modifiers
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 889–901; What's new in Kotlin 2.2.0 > New experimental build tools API
- [5] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 325–337; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate testApi configuration


Material error: False. Citations: none. Explicitly states the references contain no payroll endpoint and invents none. Adds an unnecessary categorical refusal to share even authorized internal endpoints.

### q21 — rag-rerank

What is the internal production API endpoint for my company payroll service?

Search query: What is the internal production API endpoint for my company payroll service?

I don't have that information, and it isn't something the Kotlin reference data covers. The retrieved sources are Kotlin documentation (inline functions, extensions, expect/actual, build tools, compatibility guides) and contain nothing about your company's payroll service or any internal endpoints [1][2][3][4][5].

For an internal endpoint like that, check with your own organization's documentation, infrastructure/API catalog, or platform team. I can't guess or fabricate one.

If you have a Kotlin question — including how to model or call an existing API safely — I'm happy to help with that.

- [1] [Inline functions](https://kotlinlang.org/docs/inline-functions.html), lines 167–175; Inline functions > Restrictions for public API inline functions
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), lines 420–422; Expected and actual declarations > What's next?
- [3] [Extensions](https://kotlinlang.org/docs/extensions.html), lines 478–484; Extensions > Extensions and visibility modifiers
- [4] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 889–901; What's new in Kotlin 2.2.0 > New experimental build tools API
- [5] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 325–337; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate testApi configuration


Material error: False. Citations: supported. Explains that the cited Kotlin snippets contain no payroll endpoint and directs the user to internal documentation.

### q22 — rag-rerank-filter

What breaking changes will Kotlin 3.0 ship with?

Search query: What breaking changes will Kotlin 3.0 ship with?

I can't give you a reliable answer from the retrieved reference data: no evidence survived retrieval, so the corpus is insufficient. I won't invent citations.

What I can say without those sources (and with the important caveat that this may be out of date):

- As of my knowledge, there has been no announced Kotlin 3.0 release or finalized list of its breaking changes. JetBrains has not published a "Kotlin 3.0 breaking changes" document that I'm aware of.
- Kotlin's current release line is 2.x (with language version flags like 2.0, 2.1, etc.). Major-version language changes are typically staged through the language-version and API-version mechanisms, and deprecations are usually announced well in advance.
- Historic examples of *staged* breaking changes (not confirmed for 3.0) include removing previously deprecated features, tightening type inference, and changing default behavior behind `-language-version`. But I cannot confirm any specific items for 3.0.

If Kotlin 3.0 plans exist, the authoritative places to check are the Kotlin blog, the "Kotlin roadmap" page, and YouTrack. Note that version numbers don't guarantee feature availability, and any specific claim needs a dated source.



Material error: False. Citations: none. Acknowledges empty corpus evidence and qualifies unverified release-status background as potentially outdated knowledge; no concrete 3.0 commitments invented.

### q22 — rag-rewrite

What breaking changes will Kotlin 3.0 ship with?

Search query: Kotlin 3.0 breaking changes

Kotlin 3.0 has not been released, and the supplied reference data contains no announced Kotlin 3.0 breaking changes — the snapshots only cover 2.1.20, 2.3.0, and 2.3.20 [1][2][3][4]. So there is insufficient evidence to answer what Kotlin 3.0 will ship with.

What the available data *does* show is a direction of travel you can extrapolate from cautiously:

- Old language-version support is being pruned: as of 2.3.0, the compiler no longer supports `-language-version=1.8`, and `-language-version=1.9` is unsupported on non-JVM platforms; language feature sets older than 2.0 (except 1.9 on JVM) aren't supported [1]. A 3.0 release would plausibly raise that floor again, but nothing in the data confirms this.
- Plugin/tooling migrations are being forced: the Kotlin Multiplatform Android target moves to Google's `com.android.kotlin.multiplatform.library` plugin, `withJava()` is being phased out in favor of default Java source sets, `kotlin-android-extensions` produces a configuration error, and `kotlin("jvm")` alongside `kotlin-dsl` can trigger unsupported-plugin warnings [1][2].
- API deprecations accumulate silently: `kotlin.incremental.classpath.snapshot.enabled` was removed outright [2].

None of these are 3.0 commitments. For actual 3.0 plans, you'd need the official Kotlin roadmap or the Compatibility guide — neither of which is present in the reference data here.

- [1] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 743–757; What's new in Kotlin 2.3.0 > Breaking changes and deprecations
- [2] [What's new in Kotlin 2.1.20](https://kotlinlang.org/docs/whatsnew2120.html), lines 435–441; What's new in Kotlin 2.1.20 > Breaking changes and deprecations
- [3] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 888–894; What's new in Kotlin 2.3.20 > Breaking changes and deprecations
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 1–19; What's new in Kotlin 2.3.0


Material error: False. Citations: supported. Identifies corpus insufficiency and labels extrapolation as unconfirmed. Its opening assertion that 3.0 has not been released is not established by the corpus; the cited 2.x details are supported.

### q22 — rag-rerank-rewrite

What breaking changes will Kotlin 3.0 ship with?

Search query: Kotlin 3.0 breaking changes

No reliable answer can be given from the supplied reference data. The snippets cover breaking changes in Kotlin 2.3.0, 2.1.20, and 2.3.20 [1][2][3], but none state what Kotlin 3.0 will ship with. Kotlin 3.0 is not mentioned in the provided sources, so any specific list would be speculation.

What the sources do show is the kind of changes seen in recent releases (for example, dropped older language-version support in 2.3.0 [1], Kotlin Multiplatform Android target plugin migration [1], and the `withJava()` phase-out in 2.1.20 [2]), but extrapolating these to Kotlin 3.0 would not be supported by the evidence.

For authoritative information, consult the official Kotlin roadmap or the Kotlin Compatibility guide once JetBrains publishes Kotlin 3.0 details.

- [1] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 743–757; What's new in Kotlin 2.3.0 > Breaking changes and deprecations
- [2] [What's new in Kotlin 2.1.20](https://kotlinlang.org/docs/whatsnew2120.html), lines 435–441; What's new in Kotlin 2.1.20 > Breaking changes and deprecations
- [3] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 888–894; What's new in Kotlin 2.3.20 > Breaking changes and deprecations
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 1–19; What's new in Kotlin 2.3.0


Material error: False. Citations: supported. Correctly distinguishes supported historical changes from unsupported future plans; no concrete 3.0 claims.

### q22 — rag-filter-rewrite

What breaking changes will Kotlin 3.0 ship with?

Search query: Kotlin 3.0 breaking changes

I don't have reliable information about Kotlin 3.0's contents — and importantly, no such release has been announced with a defined breaking-change list as of my knowledge cutoff.

Key caveats:

- **Kotlin 3.0 doesn't exist yet** (as a shipped or officially scheduled release). JetBrains has released 1.x and 2.x lines; anything labeled "3.0" would be speculative.
- **Breaking changes are tied to specific releases, not major version numbers.** Kotlin uses a structured deprecation policy: features are deprecated in one version, raise warnings/errors in a later one, and are removed according to announced cycles. So "what breaks in 3.0" would only be answerable from an official roadmap or release notes.
- **Release versions ≠ feature availability.** Even if a 3.0 were planned, I couldn't infer its breaking changes from the version number alone.

If you're asking in a practical sense — "what should I watch for in a future major Kotlin release?" — the pattern historically is:

- Removal of previously deprecated APIs (after the announced deprecation cycle).
- Language-feature graduations and possible behavior tightening (e.g., stricter type inference, changes to experimental APIs marked for stabilization or removal).
- Toolchain/target changes (JVM target, Kotlin/Native, WASM, K2 compiler migration effects).

But I can't cite a concrete Kotlin 3.0 breaking-change list because no authoritative source for one exists in my knowledge.

If you have a specific Kotlin release notes page or roadmap document, paste it and I'll summarize the actual announced changes. Otherwise, treat any "Kotlin 3.0 breaking changes" list you see as unverified.



Material error: False. Citations: none. Acknowledges lack of reliable knowledge and provides no invented change list or citations. Makes stronger unverified release-status claims despite its knowledge-cutoff caveat; empty retrieval alone cannot establish release status.

### q22 — rag-rerank-filter-rewrite

What breaking changes will Kotlin 3.0 ship with?

Search query: Kotlin 3.0 breaking changes

Insufficient evidence: the retrieved corpus contains no usable reference data, so I can't cite any specific Kotlin 3.0 breaking changes.

More fundamentally, there is no announced Kotlin 3.0 release. As of my knowledge, JetBrains has not published a roadmap or changelog for a 3.0 version, so any list of "breaking changes in Kotlin 3.0" would be speculation. Current major-version planning has centered on Kotlin 2.x (e.g., the K2 compiler frontend stabilizing in 2.0), not a 3.0.

Version caveat: I can't verify the latest release status here, and my knowledge has a cutoff. For authoritative information, check:
- The official Kotlin roadmap: https://kotlinlang.org/docs/roadmap.html
- Kotlin release notes: https://kotlinlang.org/docs/releases.html
- YouTrack for deprecation/removal plans

If you're actually asking about breaking changes in a specific Kotlin 2.x release (e.g., 2.0's K2 migration, language feature removals, or `-language-version` deprecations), I can help with that—but only if you confirm the version, since I can't ground claims against the (empty) retrieved evidence.



Material error: False. Citations: none. Explicitly acknowledges empty evidence and lack of live verification; release-status background is qualified as knowledge-cutoff dependent.

### q22 — rag

What breaking changes will Kotlin 3.0 ship with?

Search query: What breaking changes will Kotlin 3.0 ship with?

The reference data doesn't cover Kotlin 3.0 at all. It only covers Kotlin 2.3.0 [1][3][4][5] and 2.3.20 [2], neither of which states anything about a planned Kotlin 3.0 release or its breaking changes.

**Insufficient evidence to answer.** The sources contain no information about:

- Whether Kotlin 3.0 is planned or scheduled
- What breaking changes it might include
- Any roadmap, migration plan, or deprecation targeting a 3.x release

What the sources *do* show is the kind of breaking changes landing in the 2.3.x line, which may be relevant context but is **not** a statement about 3.0:

- Starting with Kotlin 2.3.0, the compiler no longer supports `-language-version=1.8`, and no longer supports `-language-version=1.9` on non-JVM platforms; language feature sets older than 2.0 (except 1.9 on JVM) are unsupported [3].
- Kotlin Multiplatform Android target support has moved to Google's `com.android.kotlin.multiplatform.library` plugin, with `androidTarget` renamed to `android` [3].
- With AGP 9.0.0's built-in Kotlin support, using `kotlin-android` produces a configuration error [3].

If you're looking for authoritative 3.0 plans, the Kotlin roadmap/JetBrains announcements would be the place to check — but I can't confirm anything about them from the supplied references.

- [1] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 1–19; What's new in Kotlin 2.3.0
- [2] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 1–19; What's new in Kotlin 2.3.20
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 743–757; What's new in Kotlin 2.3.0 > Breaking changes and deprecations
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 235–237; What's new in Kotlin 2.3.0 > Kotlin/Native
- [5] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 33–35; What's new in Kotlin 2.3.0 > Language


Material error: False. Citations: supported. Explicitly separates known 2.x source facts from unknown 3.0 plans and invents no future breaking-change list.

### q22 — rag-rerank

What breaking changes will Kotlin 3.0 ship with?

Search query: What breaking changes will Kotlin 3.0 ship with?

Announced breaking changes for Kotlin 3.0 cannot be stated on the supplied evidence: the references cover 2.3.0, 2.3.20, and 2.4.0 only [1][2][3][4][5], and none lists a 3.0 breaking-change set. Kotlin is generally backward-compatible with Kotlin 1.0 for language features [4], but that does not preclude future removals; concrete 3.0 plans would need the official 3.0 roadmap or release notes.

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), lines 1141–1149; What's new in Kotlin 2.4.0 > Breaking changes and deprecations
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 1–19; What's new in Kotlin 2.3.0
- [3] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 1–19; What's new in Kotlin 2.3.20
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 743–757; What's new in Kotlin 2.3.0 > Breaking changes and deprecations
- [5] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), lines 235–237; What's new in Kotlin 2.3.0 > Kotlin/Native


Material error: False. Citations: supported. Clearly states that no supplied source establishes a 3.0 breaking-change list.

### q22 — rag-filter

What breaking changes will Kotlin 3.0 ship with?

Search query: What breaking changes will Kotlin 3.0 ship with?

I can't give you a reliable answer: no usable reference data survived retrieval, and I won't invent citations or specifics.

What I can say without a source is limited and version-caveated:

- As of my knowledge cutoff, there is no announced Kotlin "3.0" release with a published, finalized breaking-change list. JetBrains has historically shipped breaking changes in major language/compiler versions (e.g., 1.x → 2.0 with the new K2 compiler frontend), but the contents of any hypothetical 3.0 are not something I can confirm.
- Kotlin's own guidance is that breaking changes are typically gated behind language-version/API-version flags and documented per release in the "What's new" and compatibility guides, rather than pre-announced as a fixed 3.0 bundle.

If you want a concrete answer, the authoritative sources to check are:
- the Kotlin "What's new" pages per release,
- the Kotlin evolution/compatibility guide (language version and API version policy),
- JetBrains' Kotlin roadmap/YouTrack announcements.

If you can provide the specific reference data (release notes, compatibility docs), I'll pin the answer to it with citations. Otherwise, treat any "Kotlin 3.0 breaking changes" list you see as unofficial until JetBrains publishes release notes.



Material error: False. Citations: none. Explicitly acknowledges empty evidence and qualifies background as knowledge-cutoff dependent. No specific future change list or invented citation.

### q23 — rag-rewrite

How do I configure a PostgreSQL replication slot for Debezium?

Search query: configure PostgreSQL replication slot for Debezium

The retrieved reference data doesn't cover PostgreSQL, Debezium, or replication slots — it only contains Kotlin release notes about Node.js settings, name-based destructuring, and a Gradle DSL deprecation [1][2][3][4][5]. I can't answer your PostgreSQL/Debezium question from these sources, and I won't guess at specifics.

For accurate guidance, consult the Debezium documentation for the PostgreSQL connector, which describes the `slot.name`, `publication.name`, and related settings for logical replication slots.

As a general orientation (verify against current Debezium docs for your version):

- Debezium's PostgreSQL connector uses the `pgoutput` logical decoding plugin, which requires a replication slot and a publication.
- You can let Debezium create the slot automatically, or pre-create it with `SELECT * FROM pg_create_logical_replication_slot('my_slot', 'pgoutput');`.
- Configure via connector properties such as `slot.name` and `publication.name`.

Because I have no trusted source here for exact property names, defaults, or version behavior, treat the above as orientation only and confirm details in the official Debezium PostgreSQL connector documentation.

- [1] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 824–840; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [2] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 140–169; What's new in Kotlin 2.3.20 > New features > Name-based destructuring > How to enable
- [3] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 607–623; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate create() function in NodeJsExec DSL
- [4] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 331–360; What's new in Kotlin 2.3.20 > Language > Changes to overload resolution for context parameters > Name-based destructuring > How to enable
- [5] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 842–842; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings


Material error: False. Citations: supported. Explicitly separates general orientation from irrelevant Kotlin sources; no false claim that the corpus supports PostgreSQL configuration.

### q23 — rag-rerank-rewrite

How do I configure a PostgreSQL replication slot for Debezium?

Search query: configure PostgreSQL replication slot for Debezium

The retrieved reference data covers Kotlin topics (Wasm Node.js settings, name-based destructuring, NodeJsExec DSL deprecation), not Debezium or PostgreSQL. I have insufficient evidence in the supplied sources to answer your question about configuring a PostgreSQL replication slot for Debezium.

To answer accurately, I'd need documentation on Debezium's PostgreSQL connector and PostgreSQL's logical replication slot mechanics. If you can supply those references, I can cite them; otherwise, consult the official Debezium PostgreSQL connector documentation and PostgreSQL's logical decoding / replication slot docs.

- [1] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 824–840; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [2] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 140–169; What's new in Kotlin 2.3.20 > New features > Name-based destructuring > How to enable
- [3] [Compatibility guide for Kotlin 2.3.x](https://kotlinlang.org/docs/compatibility-guide-23.html), lines 607–623; Compatibility guide for Kotlin 2.3.x > Tools > Deprecate create() function in NodeJsExec DSL
- [4] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 331–360; What's new in Kotlin 2.3.20 > Language > Changes to overload resolution for context parameters > Name-based destructuring > How to enable
- [5] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 842–842; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings


Material error: False. Citations: none. Acknowledges insufficient evidence and requests appropriate documentation; does not invent configuration details.

### q23 — rag-filter-rewrite

How do I configure a PostgreSQL replication slot for Debezium?

Search query: configure PostgreSQL replication slot for Debezium

I can't answer this from the retrieved reference data — no evidence survived retrieval, so I have nothing to cite. I also won't invent citations.

That said, I can give a general orientation from my own knowledge (Kotlin is irrelevant here; this is a PostgreSQL/Debezium config question). Treat the specifics as unverified against your versions:

**What Debezium needs**

For the standard `pgoutput` plugin (PostgreSQL 10+), Debezium does **not** require you to pre-create a replication slot. The connector creates one automatically, named via `slot.name` (default `debezium`), when it starts streaming.

```json
{
  "name": "inventory-connector",
  "config": {
    "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
    "database.hostname": "postgres",
    "database.port": "5432",
    "database.user": "debezium",
    "database.password": "…",
    "database.dbname": "inventory",
    "topic.prefix": "dbserver1",
    "plugin.name": "pgoutput",
    "slot.name": "debezium_inventory",
    "publication.autocreate.mode": "filtered"
  }
}
```

**If you want to create the slot manually**

```sql
SELECT * FROM pg_create_logical_replication_slot('debezium_inventory', 'pgoutput');
-- inspect
SELECT slot_name, plugin, slot_type, active, restart_lsn FROM pg_replication_slots;
-- remove when unused
SELECT pg_drop_replication_slot('debezium_inventory');
```

**Requirements/caveats to verify for your setup**

- `wal_level = logical` in `postgresql.conf`, plus a restart.
- The connector role needs `REPLICATION` and `LOGIN`; for `pgoutput` it also needs appropriate table grants (often superuser or a role with the needed privileges, depending on version).
- With `pgoutput`, a *publication* is required — Debezium can autocreate one per `publication.autocreate.mode`, or you create it and set `publication.name`.
- The `decoderbufs` plugin (older approach) has different requirements.

Version-specific behavior (default slot naming, publication autocreate semantics, required privileges) varies across Debezium and PostgreSQL releases, so confirm against the docs for the exact versions you run.



Material error: False. Citations: none. Explicitly labels instructions as unverified general knowledge. Slot default and manual-creation guidance agree with the external spot-check; this is not an exhaustive validation of deployment instructions.

### q23 — rag-rerank-filter-rewrite

How do I configure a PostgreSQL replication slot for Debezium?

Search query: configure PostgreSQL replication slot for Debezium

I don't have reference data available for this question, so I'll answer from general knowledge and note where you should verify against current docs.

## Setting up a logical replication slot for Debezium

Debezium's PostgreSQL connector requires a **logical replication slot** (`pgoutput` output plugin) plus a publication. Key steps:

**1. Configure PostgreSQL** (`postgresql.conf`):
```
wal_level = logical
max_replication_slots = 10   # at least 1 per connector
max_wal_senders = 10         # at least 1 per connector
```

**2. Create a replication user with sufficient privileges:**
```sql
CREATE ROLE debezium WITH LOGIN REPLICATION PASSWORD 'secret';
GRANT SELECT ON ALL TABLES IN SCHEMA public TO debezium;
-- Or grant on the specific tables you capture
```

**3. Create a publication** (for `pgoutput`):
```sql
CREATE PUBLICATION dbz_publication FOR TABLE public.orders, public.customers;
-- or FOR ALL TABLES
```

**4. Let Debezium create the slot** — normally you do **not** create the slot manually. Configure the connector and Debezium will run `CREATE_REPLICATION_SLOT ... LOGICAL pgoutput` on startup:
```json
{
  "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
  "database.hostname": "pg",
  "database.port": "5432",
  "database.user": "debezium",
  "database.password": "secret",
  "database.dbname": "mydb",
  "topic.prefix": "myserver",
  "plugin.name": "pgoutput",
  "publication.name": "dbz_publication",
  "slot.name": "dbz_slot"
}
```

**Manual slot creation** (only if you must pre-create it, e.g. for permissions or infrastructure-as-code):
```sql
SELECT * FROM pg_create_logical_replication_slot('dbz_slot', 'pgoutput');
```

## Important caveats

- **Slot consumption**: An idle logical slot prevents WAL from being recycled, which can fill disk. Monitor `pg_replication_slots` and drop unused slots (`pg_drop_replication_slot`).
- **Naming**: Debezium's `slot.name` must match the created slot; default is derived from `topic.prefix` (`<prefix>_slot` in newer versions).
- **Permissions**: On PostgreSQL 15+ (including managed services like RDS/Aurora), you may need additional grants — sometimes `rds_replication` on AWS, or superuser/`REPLICATION` role on self-hosted.
- **`pgoutput` vs `decoderbufs`/`wal2json`**: `pgoutput` is built in (PostgreSQL 10+) and is the recommended plugin; it requires a publication.
- **Managed cloud PostgreSQL** often restricts manual slot creation and requires the connector to create it.

I could not cite specific documentation here because no reference data was provided. Please verify `wal_level`, slot naming, and privilege requirements against the Debezium version and PostgreSQL version you're running — these details have changed across releases.



Material error: True. Citations: none. Clearly separates general knowledge, but invents a topic-prefix-derived slot default; see external check. Broad managed-service restrictions are unverified.

### q23 — rag

How do I configure a PostgreSQL replication slot for Debezium?

Search query: How do I configure a PostgreSQL replication slot for Debezium?

The retrieved reference data does not contain information about configuring PostgreSQL replication slots for Debezium. The sources cover Kotlin/Wasm Node.js settings [1][3], name-based destructuring [2][4], and Kotlin documentation updates [5] — none of which address PostgreSQL or Debezium.

I can't answer this from the supplied references. To get an accurate answer, consult the Debezium documentation for the PostgreSQL connector (`slot.name`, `publication.name`, `plugin.name` parameters) or the PostgreSQL replication documentation (`pg_create_logical_replication_slot`, `max_replication_slots`).

If you have a Kotlin-related question, I'm happy to help with that instead.

- [1] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 824–840; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [2] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 140–169; What's new in Kotlin 2.3.20 > New features > Name-based destructuring > How to enable
- [3] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 842–842; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [4] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 331–360; What's new in Kotlin 2.3.20 > Language > Changes to overload resolution for context parameters > Name-based destructuring > How to enable
- [5] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 1144–1146; What's new in Kotlin 2.2.0 > Documentation updates > New and revamped pages


Material error: False. Citations: supported. Correctly identifies the irrelevant Kotlin evidence and declines a sourced answer; offers documentation/property pointers without presenting a configuration as corpus-supported.

### q23 — rag-rerank

How do I configure a PostgreSQL replication slot for Debezium?

Search query: How do I configure a PostgreSQL replication slot for Debezium?

The reference data concerns Kotlin configuration (Kotlin/Wasm Node.js settings and name-based destructuring), not PostgreSQL, Debezium, or replication slots. It provides no information on configuring a PostgreSQL replication slot for Debezium, so I can't answer from these sources.

I can answer briefly from general knowledge, but treat it as unverified against the supplied material:

Debezium's PostgreSQL connector normally manages its own replication slot. You can influence it with connector properties rather than configuring the slot manually:

- `plugin.name` — set to `pgoutput` (required for logical replication with PostgreSQL 10+).
- `slot.name` — the logical replication slot name Debezium creates/uses (default derived from the connector name; must be unique per connector and valid, lowercase, ≤63 chars).
- `publication.name` — the publication Debezium uses (default `dbz_publication`); set `publication.autocreate.mode` to control creation (`all_tables`, `filtered`, `disabled`).
- `slot.drop.on.stop` — whether to drop the slot when the connector stops (default `false`).
- `snapshot.mode` — controls when the initial snapshot runs.

Manual slot creation is generally unnecessary; Debezium creates the slot automatically when it starts streaming (subject to permissions). Note that a slot is only actually created once the connector reaches the streaming phase, not at snapshot time.

Version caveats and uncertainty: exact property names, defaults, and behaviors vary across Debezium versions and PostgreSQL versions, and I can't confirm current values without up-to-date Debezium/PostgreSQL documentation. If you need precise, authoritative steps (including required `wal_level`, `max_replication_slots`, and role privileges), consult the Debezium PostgreSQL connector docs and PostgreSQL logical replication docs for your specific versions.

- [1] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 824–840; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [2] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 140–169; What's new in Kotlin 2.3.20 > New features > Name-based destructuring > How to enable
- [3] [What's new in Kotlin 2.1.0](https://kotlinlang.org/docs/whatsnew21.html), lines 842–842; What's new in Kotlin 2.1.0 > Kotlin/Wasm > Subproject-specific Node.js settings
- [4] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), lines 331–360; What's new in Kotlin 2.3.20 > Language > Changes to overload resolution for context parameters > Name-based destructuring > How to enable
- [5] [What's new in Kotlin 2.2.0](https://kotlinlang.org/docs/whatsnew22.html), lines 1144–1146; What's new in Kotlin 2.2.0 > Documentation updates > New and revamped pages


Material error: True. Citations: none. Clearly separates general knowledge, but gives an incorrect slot-name default; see the post-generation external check. Its confident claim that slots are never created during snapshots is also unverified.

### q23 — rag-filter

How do I configure a PostgreSQL replication slot for Debezium?

Search query: How do I configure a PostgreSQL replication slot for Debezium?

I don't have any reference material on this topic in the retrieved corpus, so I can't cite sources. What follows is general knowledge, and you should verify it against the official Debezium and PostgreSQL docs for your versions.

## Summary

For Debezium's PostgreSQL connector, you don't create the replication slot manually in most cases — the connector creates it for you using the `slot.name` property, provided the connecting role has `REPLICATION` privileges. Manual creation is possible but usually unnecessary and easy to get wrong.

## Key configuration

In the Debezium PostgreSQL connector config:

```properties
connector.class=io.debezium.connector.postgresql.PostgresConnector
database.hostname=localhost
database.port=5432
database.user=debezium
database.password=secret
database.dbname=inventory
plugin.name=pgoutput          # or decoderbufs / wal2json
slot.name=debezium_inventory  # slot Debezium will create/use
publication.name=dbz_publication
```

The connector issues `CREATE_REPLICATION_SLOT` over the replication protocol. You generally should **not** pre-create the slot with `pg_create_logical_replication_slot`, because a manually created slot may not match what the connector expects, and it will hold WAL even when the connector is stopped.

## Prerequisites on the PostgreSQL side

- `wal_level = logical` in `postgresql.conf` (requires restart).
- A role with `REPLICATION` and the right table privileges:
  ```sql
  ALTER ROLE debezium WITH REPLICATION LOGIN;
  GRANT SELECT ON ALL TABLES IN SCHEMA public TO debezium;
  ```
- For the `pgoutput` plugin (default), a publication must exist — Debezium can create it if the role has permission, or you create it:
  ```sql
  CREATE PUBLICATION dbz_publication FOR TABLE inventory.customers;
  ```
- Ensure `max_replication_slots` and `max_wal_senders` are sufficient (defaults are often too low in older versions).

## Inspecting and cleaning up

```sql
SELECT * FROM pg_replication_slots WHERE slot_name = 'debezium_inventory';
SELECT pg_drop_replication_slot('debezium_inventory');
```

If you stop a connector permanently, drop its slot — an idle logical slot retains WAL and can fill the disk. Also watch `restart_lsn` and `confirmed_flush_lsn` to gauge lag.

## Version caveats

- Plugin support and defaults (particularly `pgoutput` availability and publication handling) vary across Debezium and PostgreSQL versions.
- Snapshot/streaming behavior and slot name rules have changed over releases.
- I cannot confirm specifics from the retrieved corpus — treat all of the above as needing verification against your exact Debezium and PostgreSQL versions.



Material error: True. Citations: none. Honest about missing sources, but incorrectly labels pgoutput as the default, lists wal2json as supported, and discourages valid manual creation; see external check.

### q23 — rag-rerank-filter

How do I configure a PostgreSQL replication slot for Debezium?

Search query: How do I configure a PostgreSQL replication slot for Debezium?

I don't have any retrieved reference data to cite here, so I'll answer from general knowledge and flag uncertainty where it matters.

## Short answer

You don't configure the replication slot directly for Debezium in most cases. Debezium creates and manages the slot itself on the PostgreSQL server when the connector starts. You control it indirectly through connector configuration properties, and you must ensure the database user has the right privileges.

## Server-side prerequisites

- `wal_level = logical` in `postgresql.conf` (requires a restart).
- Enough `max_replication_slots` and `max_wal_senders` for your connectors.
- A role with `REPLICATION` and `LOGIN` privileges.
- For the initial snapshot, the role also needs `SELECT` on the captured tables and (for older PG versions) table ownership or superuser, or use `PUBLICATION`-based setup on PG 10+.

```sql
ALTER ROLE debezium WITH REPLICATION LOGIN;
```

## Connector-side configuration

Key properties in the Debezium PostgreSQL connector:

| Property | Meaning |
|---|---|
| `plugin.name` | `pgoutput` (native, PG 10+), or `decoderbufs`/`wal2json` (legacy) |
| `slot.name` | Name of the slot Debezium will create/use |
| `publication.name` | Publication for `pgoutput` (default `dbz_publication`) |
| `publication.autocreate.mode` | `all_tables` (default) or `disabled` |
| `slot.drop.on.stop` | Whether to drop the slot when the connector stops |
| `snapshot.mode` | e.g. `initial`, `never` |

Example (JSON for Kafka Connect):

```json
{
  "name": "inventory-connector",
  "config": {
    "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
    "database.hostname": "postgres",
    "database.port": "5432",
    "database.user": "debezium",
    "database.password": "dbz",
    "database.dbname": "inventory",
    "topic.prefix": "dbserver1",
    "plugin.name": "pgoutput",
    "slot.name": "debezium_inventory",
    "publication.autocreate.mode": "all_tables",
    "table.include.list": "public.customers,public.orders"
  }
}
```

## Things that commonly bite people

- **Replication slot name length**: PostgreSQL truncates identifiers to 63 characters; keep slot names short and unique per connector.
- **Slot leak / WAL bloat**: an idle or stopped connector keeps the slot, preventing WAL recycling. Use `slot.drop.on.stop=true` in dev, and monitor `pg_replication_slots` in prod.
- **Permissions on PG < 10**: `pgoutput` doesn't exist; you need a logical decoding plugin installed and superuser-level access.
- **Publication autocreation** requires the user to be able to `CREATE PUBLICATION` (owner or superuser), and it only works if `wal_level=logical`.

## Version caveats

Logical decoding, `pgoutput`, and publication behavior differ across PostgreSQL 9.4 → 10 → 11 → 13+. Debezium connector versions also vary in defaults (e.g., slot/publication defaults changed over 1.x → 2.x → 3.x). I can't confirm exact per-version defaults from the corpus here — check the Debezium documentation for the specific connector version you're running before relying on any default.

If you tell me your PostgreSQL version, Debezium version, and deployment mode (Kafka Connect, embedded, Debezium Server), I can be more precise.



Material error: True. Citations: none. Acknowledges empty evidence, but lists unsupported wal2json as an option; see external check. Other version-specific operational details are not exhaustively verified.
