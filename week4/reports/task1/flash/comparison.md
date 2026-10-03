# RAG answer comparison

Ten independent pairs, DeepSeek generation only. Judgments were authored separately by the implementing assistant against saved source sections. This is illustrative, not a broad reliability claim.

Index SHA-256: `8249a981e64ba378fac6690ca46299545fa68f054cfb5ccbe5f311119104040d`

Settings: `{"approach": "none", "compression": false, "context_strategy": "full", "model": "deepseek-v4-flash", "rag_budget": 6000, "rag_limit": 5, "rag_strategy": "semantic", "reasoning": "low"}`

Selection: Ten hardest answerable questions by reviewer judgment, selected before generation: version sensitivity and interop/multiplatform edge cases, including the q16 retrieval miss from Task0. Not a measured universal difficulty ranking. Original questions unchanged. Out-of-corpus controls q21-q23 are excluded.

| Question | Why selected |
| --- | --- |
| q01 | Current context-parameter syntax, scope resolution and ambiguity. |
| q11 | Matching declarations across a multiplatform source-set hierarchy. |
| q12 | Actualization with a foreign platform type and compatibility constraints. |
| q13 | Exception propagation versus process termination across language boundaries. |
| q14 | Direct Swift export differs from the older Objective-C bridge. |
| q16 | Both Task0 chunkers missed the designated Kotlin/JS source section, retrieving Wasm neighbors. |
| q17 | Opaque references across the Wasm/JavaScript boundary. |
| q18 | Exact stabilization release with experimental exceptions. |
| q19 | A version-specific compiler flag and async bridge introduced in 2.4.20. |
| q20 | New cross-language inheritance and reverse-import requirements in 2.4.20. |

My Flash review found **8 improvements, 1 regression in completeness and 1 mixed result** with RAG. The largest gains fix UUID stabilization, replace unsupported Wasm lifetime/API advice with the documented JsReference pattern, and supply the 2.4.20 lambda export flag. On q12, RAG omits the essential existing-type compatibility condition that plain Flash explains; unlike Pro RAG, it does not assert that aliases cannot have members. Numbered citations resolve, but source support is imperfect, notably the Alpha attribution in q14 and the harness-caveat attribution in q19. These judgments apply to this selected run only.

| Question | RAG outcome | My comparison |
| --- | --- | --- |
| q01 | improved | Plain knows the newer syntax but centers the old receiver form and is uncertain about current calls. RAG explains named context parameters and same-scope ambiguity directly from the guide. |
| q11 | mixed | Plain is more complete on matching signatures but adds unsupported rules and questionable examples. RAG avoids those claims and explicitly acknowledges its missing signature evidence; neither dominates completeness and grounding. |
| q12 | regressed in completeness | Plain explains that existing constructors/members must satisfy expect. RAG omits this essential condition, although it gives package placement and source caveats. Both include some overly broad extra statements; this is a completeness regression, not Pro RAG’s stronger false member prohibition. |
| q13 | improved | RAG explicitly includes subclasses, the always-present suspend error parameter and unannotated CancellationException propagation. Plain omits these qualifications and adds unverified NSError implementation details. |
| q14 | improved | Both know the direct async mapping. RAG also covers suspending functional types and the correct Objective-C async-callability distinction; plain incorrectly treats documented Flow mapping as planned. RAG retains an Alpha citation gap. |
| q16 | improved | Both give the basic external answer. RAG avoids plain’s unsupported JsGlobal and false only-external-can-lack-a-body rule. Its Wasm nativeInvoke example still needs clearer target labeling and neither answer covers all declaration rules. |
| q17 | improved | Plain names JsReference but uses an unsupported constructor and mandatory release() advice. RAG gives toJsReference(), get() and the documented default-import JS example without invented lifetime management. |
| q18 | improved | Plain wrongly says stable from 2.1.0 with no opt-in. RAG gives 2.4.0 and the V4/V7 experimental exceptions. Plain alone includes the correct 2.0.20 introduction background. |
| q19 | improved | Plain declines to verify the release and gives an incompatible old-style wrapper example. RAG states the change, required flag and working pattern from the snapshot; the extra version-caveat citation is misattributed. |
| q20 | improved | Both identify inheritance and interfaces. RAG completes the reverse-import pattern by passing the Swift object back into Kotlin and grounds the pre-stable caveat in supplied source text. |

## Method and limits

Same pre-recorded expected facts and saved Task0 sources as the Pro run. Content and citation support assessed before paired and model outcomes. Missing includes partial coverage; incorrect means contradicted by the source. Unsupported does not automatically mean false. Examples were not compiled.

Each question ran independently through the rebuilt `week4/advent-agent rag compare` path and the normal DeepSeek adapter. Both arms received identical base instructions, `deepseek-v4-flash`, low reasoning and no temperature override. Only RAG received evidence from `week4/.venv/bin/kotlin-index`, using semantic retrieval, 5 results and a 6,000-character whole-chunk budget. There was no session history, memory, project instruction, lifecycle or MCP context; expected facts were withheld. Mode order alternated. Content was assessed before assigning paired outcomes. Reviews are bound to the entire saved answer record by SHA-256.

The saved corpus is the reference standard, not a claim about live documentation today. Source pages can contain omissions or inconsistencies. Completeness checks include some useful caveats beyond the literal short question; missing such a caveat is distinguished from an incorrect core answer. No examples were compiled. Latencies below measure provider calls only and do not include local retrieval.

Estimated generation cost: $0.010546. Embedding work excluded.

| Mode | Answers | Provider seconds | Total tokens | Estimated USD |
| --- | ---: | ---: | ---: | ---: |
| plain | 10 | 115.31 | 24740 | 0.006808 |
| rag | 10 | 36.67 | 19351 | 0.003737 |

Observed cost and latency differences are not controlled performance estimates; answer lengths, reasoning and cache usage vary.

## Answers and individual reviews

## q01 — plain

How can a function require an implicit service and how is that service resolved?

## Short answer

In Kotlin this is done with **context receivers** (older) / **context parameters** (newer): a function can declare types it requires in a `context(...)` clause, and the compiler resolves them statically from the surrounding scope — it is *not* runtime dependency lookup.

## Context receivers (Kotlin 1.6.20 – 2.1.x, `-Xcontext-receivers`)

```kotlin
class Logger { fun log(m: String) = println(m) }

context(Logger)
fun greet() {
    log("hi")            // Logger is an implicit receiver here
}

fun main() {
    with(Logger()) {
        greet()          // OK: Logger is in the enclosing implicit context
    }
    // greet()           // compile error: no Logger in context
}
```

## How resolution works

* **Compile time only.** When the compiler sees `greet()`, it must find a value of the required context type in the *caller's* context: either the caller itself declares it in its own `context(...)` clause, or it's available as an enclosing implicit receiver (e.g. `with(logger) { ... }`).
* Resolution follows the same rules as extension/implicit receivers: innermost/most specific applicable candidate wins; ambiguity or no candidate is a **compile error**.
* **At runtime** the context value is just an extra parameter appended to the function's signature (on the JVM a normal trailing parameter), so there is no service-locator lookup or reflection cost.

## Version caveat

* Context receivers were replaced by **context parameters**, e.g. `context(logger: Logger) fun greet()`, available as a preview in Kotlin 2.2.x behind `-Xcontext-parameters`; context receivers were deprecated in favour of them.
* I'm not fully certain of the final call-site semantics for context parameters (whether the caller may pass them positionally like ordinary arguments, with the "implicit" part applying only inside the body). Check the release notes for your exact version.

## If you meant DI, not the compiler

For runtime DI (Koin `by inject()`, Spring `@Autowired`, etc.) the "service" is resolved by a container at runtime, not by the compiler — a different mechanism with different failure modes. The Kotlin-language answer to "a function requires an implicit service" is context receivers/parameters.

Review source: [reports/task1/sources/context-parameters.txt](../sources/context-parameters.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Declare context(name: Service) on the function for an implicit dependency. | satisfied | Names the newer context(logger: Logger) syntax in its version caveat, although most of the answer demonstrates the old receiver form. |
| Resolution happens at the call site by matching context values by type. | satisfied | Explains static call-site resolution by required type. |
| Multiple compatible values at the same scope level cause ambiguity. | missing | Mentions ambiguity generally, but not the current rule for multiple compatible values at the same scope level. |

Unsupported/material claims: The JVM trailing-parameter ABI claim and exact version/flag timeline are not established by the designated source. The answer acknowledges uncertainty about current parameter call-site semantics; the full snapshot explains experimental explicit named context arguments at lines 94–148.

Citation review: Not applicable: plain mode has no supplied references.

## q01 — rag

How can a function require an implicit service and how is that service resolved?

## Requiring an implicit service

Declare a **context parameter** with the `context` keyword followed by a `name: Type` list on the function (or property) [5]:

```kotlin
interface UserService {
    fun log(message: String)
}

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")   // `users` is available implicitly
}
```

The service is not passed as an ordinary argument; callers provide it via the surrounding context. Context parameters replaced the older experimental *context receivers* feature [5].

## How resolution works

Resolution happens **at the call site**, by **type**: Kotlin searches the current scope for matching context values [3]. You supply them with a `context(...) { ... }` block:

```kotlin
fun main() {
    val users = object : UserService {
        override fun log(message: String) = println(message)
    }
    context(users) {
        outputMessage("Looking up the first user")   // resolves `users` by type
    }
}
```

If **multiple compatible values** of the same type exist at the same scope level, the compiler reports an **ambiguity error** [3]:

```kotlin
context(serviceA, serviceB) {
    outputMessage("...")   // error: ambiguous UserService context
}
```

You can also name a context parameter `_` when you don't need to reference it directly; it still satisfies callees' context requirements, and the value is retrievable explicitly via `contextOf<T>()` [1].

## Version caveat

The cited documentation pages do not state the Kotlin release in which context parameters became available or their stability/opt-in status [1][3][5]. Context parameters are noted as the successor to context receivers, which were experimental [5], but the reference data does not confirm the current stability of context parameters — check the current docs for your Kotlin version.

Retrieved references:

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 48–59; Context parameters
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 475–509; What's new in Kotlin 2.4.0 > Standard library > New map fallback functions to distinguish null values and missing keys
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 61–92; Context parameters > Context parameters resolution
- [4] [Extensions](https://kotlinlang.org/docs/extensions.html), normalized lines 206–221; Extensions > Extension functions > Anonymous extension functions
- [5] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 1–46; Context parameters

Review source: [reports/task1/sources/context-parameters.txt](../sources/context-parameters.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Declare context(name: Service) on the function for an implicit dependency. | satisfied | Uses context(users: UserService) and identifies replacement of receivers. |
| Resolution happens at the call site by matching context values by type. | satisfied | Explicitly gives call-site type-based matching and a context block. |
| Multiple compatible values at the same scope level cause ambiguity. | satisfied | Explains same-scope ambiguity with multiple compatible services. |

Unsupported/material claims: The broad statement that callers do not pass the service as an ordinary argument should be limited to implicit calls: the complete snapshot also documents experimental explicit context arguments, absent from the retrieved chunks. Otherwise the explanation matches the guide and clearly qualifies version uncertainty.

Citation review: All used references [1], [3], [5] resolve and support the named syntax, contextOf, resolution and ambiguity claims. The statement about missing version/stability evidence accurately describes those supplied chunks, without inventing a version.

## q11 — rag

Where should expect and actual declarations live and which parts must match?

- **`expect` declarations live in the common source set** (e.g. `commonMain`). They are standard Kotlin constructs marked `expect` and must not contain an implementation. [1][2]
- **`actual` declarations live in the platform-specific source set(s)** (e.g. `jvmMain`, `nativeMain`, etc.). They are marked `actual` and typically provide the platform-specific implementation. [1]
- They must be in the **same package** as the corresponding expected declaration. [1][2]
- Every expected declaration must have a **matching actual declaration for every platform/target**; the compiler matches and merges them per platform. [2]
- Actual declarations may also live in an **intermediate source set** shared by several targets, such as `iosMain` for `iosArm64Main` and `iosSimulatorArm64Main`, instead of in each leaf platform source set. [2]
- The documented matching requirement is the **same construct** in the **same package**; expected declarations cannot have implementation. [1][2]

```kotlin
// commonMain
expect fun platformName(): String

// jvmMain
actual fun platformName(): String = System.getProperty("os.name")
```

The supplied excerpt explicitly calls out “same construct” and “same package” [1][2]; it does not spell out every signature/type-matching detail beyond that.

Retrieved references:

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 1–5; Expected and actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 196–218; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](../sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| expect belongs in common code and has no implementation. | satisfied | States common source set and no implementation. |
| actual implementations belong in platform or shared intermediate source sets. | satisfied | Explicitly covers intermediate shared source sets instead of duplicating actuals in leaf source sets. |
| Corresponding declarations need matching signatures and the same package, covering each target. | missing | Covers package and target matching, but explicitly declines signature/type detail because the supplied excerpts omit it. The full source flags mismatched declaration signatures at line 35. |

Unsupported/material claims: No material unsupported claim found. It is appropriately transparent about evidence limits, but still incomplete against the full review source.

Citation review: Used [1] and [2] identify the correct supplied rules and compilation chunks. They support placement, package, no-body, target coverage and intermediate-source-set claims. The acknowledged signature gap is real.

## q11 — plain

Where should expect and actual declarations live and which parts must match?

## Where they live

- Both must be in **the same Gradle module** (same compilation unit family). You cannot put an `expect` in one module and its `actual` in another.
- They must be in **the same package** (same fully qualified name).
- `expect` goes in a **common source set** (`commonMain`).
- `actual` goes in a **platform source set** (`androidMain`, `jvmMain`, `iosMain`, `jsMain`, …), and there must be exactly one `actual` per platform compilation. The expect must be visible from the actual's source set, and vice versa.
- `expect`/`actual` are soft modifiers, not keywords.

> Version note: recent Kotlin versions allow `actual` declarations in **intermediate source sets** (e.g. `appleMain`) rather than only the leaf platform sets when using the hierarchical model. Verify against your Kotlin version before relying on it.

## What must match

| Aspect | Rule |
|---|---|
| Name & package | Must be identical |
| Declaration kind | class / object / function / property / typealias must agree |
| Parameters | Same count, types, order, and **names** (common code may use named arguments, so mismatches are diagnosed); same receiver, vararg, extension receiver |
| Return type | Must match exactly |
| Type parameters | Count, names, and constraints must match |
| Modifiers | `suspend`, `operator`, `infix`, `inline`, `external`, `annotation`, … must agree; visibility must be compatible (just make it identical) |
| Default arguments | Declare them on the **expect**; the actual must not introduce its own |
| Class members | Every member declared in the expect class needs a matching `actual` member; constructors/properties declared in the expect need matching `actual constructor` / `actual val` |

## What does *not* need to match

- The bodies / implementations.
- Extra members in the actual class (they're simply invisible to common code).
- Private implementation details.

## Example

```kotlin
// commonMain/kotlin/com/example/Platform.kt
package com.example

expect fun format(value: Int, radix: Int = 10): String

expect class Clock {
    fun now(): Long
}
```

```kotlin
// androidMain/kotlin/com/example/Platform.kt
package com.example

// default value lives only on the expect
actual fun format(value: Int, radix: Int): String = Integer.toString(value, radix)

actual class Clock {
    actual fun now(): Long = System.currentTimeMillis()
    fun extra(): Unit {}   // fine: not visible in common code
}
```

If the expect class takes constructor properties, they need the `actual` keyword too:

```kotlin
expect class Id(val value: String)

actual class Id actual constructor(actual val value: String)
```

## Two special cases

- **`actual typealias`** is a legal way to fulfill an `expect class` by aliasing a platform type: `actual typealias Id = java.util.UUID`.
- **`expect annotation class`** requires `@OptionalExpectation` if some target platforms don't provide an `actual`.

Uncertainty: the exact set of diagnostics for parameter-name and visibility mismatches varies between the old frontend and K2; the safe rule is "make everything identical, and omit defaults from the actual."

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](../sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| expect belongs in common code and has no implementation. | satisfied | Places expect in commonMain and shows body-free declarations; the prohibition on expect implementations is not stated explicitly. |
| actual implementations belong in platform or shared intermediate source sets. | satisfied | Includes platform sets and intermediate appleMain. |
| Corresponding declarations need matching signatures and the same package, covering each target. | satisfied | Explicitly covers package/name, signature details and one actual per compilation. |

Unsupported/material claims: Several extra claims exceed the source: mutual source-set visibility, exact modifier/name rules, and the soft-modifier terminology. The phrase that bodies need not match is misleading because expect cannot have an implementation. The Id/UUID alias example is not justified as compatible with the preceding constructor/property contract. These examples were not compiled.

Citation review: Not applicable: plain mode has no supplied references.

## q12 — plain

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

## Short answer

Yes. You don't have to write a wrapper class — an `expect class` can be actualized with an `actual typealias` pointing at an existing platform type, **provided that platform type already exposes the API (constructors + members) your `expect` declaration promises.**

## Example

```kotlin
// commonMain
expect class AtomicRef<T>(value: T) {
    fun get(): T
    fun set(value: T)
    fun compareAndSet(expected: T, new: T): Boolean
}

// jvmMain
actual typealias AtomicRef<T> = java.util.concurrent.atomic.AtomicReference<T>
```

Only `commonMain` code sees the three declared members; the JVM source set gets the real `AtomicReference`.

## Caveats

- **Constructor matching matters.** The platform type must have a constructor with the same signature as the one declared in `expect`. `java.time.Instant`, for example, has no public constructor — you can't `actual typealias` it against `expect class Instant(epochMs: Long)`, even though the behavior exists via `Instant.ofEpochMilli`. In that case you'd write a small wrapper class instead.
- **Members must exist with matching signatures.** You can't satisfy an `expect` member that the platform type lacks (extension functions won't help — the actualizer must be a real member). Extra members the platform type has are typically fine and simply aren't visible from common code.
- **Only works for classes.** `expect fun`, `expect val`, `expect interface`, etc. are separate mechanisms.
- **Same module/package**, of course, and the alias lives in the platform source set.
- **Version note:** with Kotlin 2.0+, using `expect`/`actual` *classes* emits a Beta warning; you can suppress it with the `-Xexpect-actual-classes` compiler flag. The `actual typealias` mechanism itself has been supported and documented for a long time (Kotlin 1.3/1.4 era), but it's worth confirming against your specific compiler version.

If the platform type doesn't line up exactly, prefer a thin `actual class` wrapper over extending the surface of the alias.

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](../sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use actual typealias in the platform source set to satisfy an expected class with an existing type. | satisfied | Uses actual typealias AtomicRef<T> in jvmMain. |
| The existing type must meet the expected declaration requirements; this is an alias, not a wrapper. | satisfied | Explicitly requires the existing constructor/member API to match and explains that the alias cannot add missing members. |
| The actual typealias belongs in the same package as expect; the aliased platform class may live elsewhere. | satisfied | States same package and platform alias placement; the java.util.concurrent target demonstrates the aliased type can be elsewhere. |

Unsupported/material claims: The blanket only-classes restriction excluding expect interfaces and the historical version timeline are not established by the reviewed section. The full guide supports the Beta class warning and suppression flag, but not every additional version claim.

Citation review: Not applicable: plain mode has no supplied references.

## q12 — rag

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes — via `actual typealias`, you can point the expected declaration directly at an existing platform type rather than declaring a wrapper class. The alias must be in the same package as the expected declaration (the referred class itself can live elsewhere) [3]:

```kotlin
// commonMain
expect class MyDate

// jvmMain
actual typealias MyDate = java.time.LocalDate
```

If the platform type references other types (e.g. `LocalDate` uses the `Month` enum), those also need expected declarations in common code [3]. Visibility can also be widened this way: an internal expected class can be satisfied by an existing public platform type, using `@Suppress("ACTUAL_WITHOUT_EXPECT")` on the alias [4].

Two alternatives that fall short of what you asked:

- Inheriting from a platform base class. The actual is still a Kotlin class you declare, but it inherits and reuses the platform type's functionality (`actual class CommonIdentity : Identity()`), so it's often preferable to a wrapper that delegates [1][2].
- Just wrapping the platform type — the case the docs implicitly push you away from; the general rule is to prefer standard constructs (e.g. interfaces) over expected/actual declarations, and expected/actual *classes* are Beta, emitting a compiler warning unless you add `-Xexpect-actual-classes` [5].

Version caveat: these references come from unversioned doc snapshots (release and availability unknown), so I can't confirm when `actual typealias` or the `ACTUAL_WITHOUT_EXPECT` suppression was introduced or whether behavior changed across versions. Verify against the Kotlin version you target.

Retrieved references:

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 235–251; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 220–233; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](../sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use actual typealias in the platform source set to satisfy an expected class with an existing type. | satisfied | Uses actual typealias to point to an existing LocalDate without a wrapper. |
| The existing type must meet the expected declaration requirements; this is an alias, not a wrapper. | missing | Never states that the existing type must meet the expected constructor/member requirements. A name-only example does not explain this essential compatibility condition. |
| The actual typealias belongs in the same package as expect; the aliased platform class may live elsewhere. | satisfied | Explicitly says the alias shares the expected package while the referred class can be elsewhere. |

Unsupported/material claims: Generalizes the particular Month/LocalDate example into a rule that referenced types also need expect declarations; the source gives that example, not a universal rule for all referenced types. The suggestion the docs push away from wrappers is editorial inference. Suppression guidance omits the complete source’s Kotlin 2.0 caveat, although the answer explicitly acknowledges version uncertainty.

Citation review: All [1]–[5] labels resolve. They support the alias/package example, inheritance alternative, suppression example and Beta caveat. [3] supports the specific Month dependency but not the universal formulation. [4] omits the visibility/version caveat from the complete section.

## q13 — rag

How are Kotlin exceptions exposed to Swift and Objective-C callers?

According to the provided Kotlin/Native Swift/Objective-C interop guide (release/availability unknown), Kotlin exceptions are exposed as follows [1]:

- All Kotlin exceptions are **unchecked**, while Swift only has checked errors handled at compile time [1].
- To let Swift/Objective-C callers receive a Kotlin exception, mark the Kotlin function with `@Throws` and list the expected exception classes [1].
- Non-`suspend` functions with or inheriting `@Throws` are exposed as:
  - `NSError*`-producing methods in Objective-C
  - `throws` methods in Swift [1]
- `suspend` functions always expose an `NSError*` / `Error` parameter in the completion handler [1].
- If the thrown Kotlin exception is one of the `@Throws` classes or a subclass, it is propagated as an `NSError` [1].
- Any other Kotlin exception reaching Swift/Objective-C is unhandled and causes program termination [1].
- `suspend` functions without `@Throws` propagate only `CancellationException` as `NSError` [1].
- Non-`suspend` functions without `@Throws` do not propagate Kotlin exceptions at all [1].
- The reverse translation is not implemented yet: Swift/Objective-C error-throwing methods are not imported into Kotlin as exception-throwing [1].

Example:

```kotlin
@Throws(IOException::class)
fun writeToFile() {
    throw IOException()
}
```

This is exposed to Swift as a `throws` function and to Objective-C as an `NSError*`-producing method [1]. See the interopedia for an example [2].

Retrieved references:

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), normalized lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Calling Java from Kotlin](https://kotlinlang.org/docs/java-interop.html), normalized lines 585–593; Calling Java from Kotlin > Checked exceptions
- [5] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 33–39; Interoperability with Swift/Objective-C > Using Kotlin in Swift/Objective-C > Hide Kotlin declarations from Objective-C and Swift

Review source: [reports/task1/sources/native-objc-interop.txt](../sources/native-objc-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| @Throws on non-suspend functions yields Swift throws and Objective-C NSError-producing methods. | satisfied | Gives non-suspend @Throws mapping into NSError-producing methods and Swift throws. |
| Only listed exception classes and subclasses propagate as NSError; other escaping Kotlin exceptions terminate the program. | satisfied | Specifies listed types/subclasses and termination for other escaping exceptions. |
| Suspend completion handlers always have an error parameter; without @Throws only CancellationException propagates, while non-suspend exceptions do not propagate. | satisfied | Covers the always-present suspend error parameter, CancellationException-only propagation without @Throws, and no non-suspend propagation. |

Unsupported/material claims: Core rules match the snapshot. The unqualified IOException example needs a Native-compatible definition/import; the supplied IOException code is from the Java interop page. The broad checked/unchecked introductory distinction is copied from the source and not independently validated here.

Citation review: All citations [1], [2] resolve. [1] supports the propagation rules; [2] merely points to an interopedia example, which was not inspected. Neither establishes Native availability of IOException.

## q13 — plain

How are Kotlin exceptions exposed to Swift and Objective-C callers?

# Kotlin exceptions across the Objective-C/Swift boundary

This applies to **Kotlin/Native** (i.e., Kotlin Multiplatform targeting Apple platforms) — the only backend that exposes Kotlin directly to Obj-C/Swift.

## Default behavior: no mapping, process aborts

Kotlin exceptions are unchecked, and Objective-C/Swift have no equivalent. By default, if an exception escapes a Kotlin function into the Apple interop layer, it is **not** converted to `NSError`. The runtime treats it as an uncaught Kotlin exception and **terminates the program** (typically logged as `Uncaught Kotlin exception: ...`).

## Opt-in mapping with `@Throws`

Annotating a Kotlin function with `@Throws(...)` changes the generated Objective-C signature to use the Cocoa error convention:

```kotlin
@Throws(IOException::class)
fun readConfig(path: String): String { /* ... */ }
```

Generated Obj-C:

```objc
- (NSString *)readConfigWithPath:(NSString *)path error:(NSError **)error;
```

Swift sees it as a throwing method:

```swift
do {
    let s = try ConfigKt.readConfig(path: "a.txt")
} catch let error as NSError {
    // error.domain == "KotlinException"
    // error.userInfo["KotlinException"] is the Kotlin throwable object
}
```

The thrown Kotlin `Throwable` is wrapped in an `NSError` with domain `KotlinException`; the original object is placed in `userInfo["KotlinException"]` (bridged to the actual Kotlin class when it's exported).

## Important caveats

- **Only the listed types are converted.** If a function annotated `@Throws(IOException::class)` throws something else (e.g. `IllegalStateException`), that exception is *not* caught and still crashes the process. Broadly, list the base exception class you intend to catch.
- **Only direct throws.** Exceptions thrown later from Kotlin callbacks/lambdas invoked after the function has returned are not routed into the `NSError**` out-parameter.
- **Suspend functions.** When a `suspend` Kotlin function is annotated with `@Throws`, the Obj-C/Swift completion-handler form delivers the error through its `NSError *` argument instead of crashing.
- **No Obj-C `NSException` bridging.** Kotlin does not map to/from `NSException`; only `NSError`-style errors are produced via `@Throws`.
- **Version note.** The `@Throws` → `NSError**` behavior (including the `KotlinException` domain and `userInfo` key) has been stable across recent Kotlin/Native releases, but details of how exceptions in callbacks are surfaced have evolved — verify against the Kotlin/Native version you're using.

I don't have reference material supplied here, so the above reflects my general knowledge of Kotlin/Native interop rather than a cited source; double-check the "interoperability with Swift/Objective-C" docs for your Kotlin version.

Review source: [reports/task1/sources/native-objc-interop.txt](../sources/native-objc-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| @Throws on non-suspend functions yields Swift throws and Objective-C NSError-producing methods. | satisfied | Shows annotated NSError out-parameter and Swift try/catch mapping. |
| Only listed exception classes and subclasses propagate as NSError; other escaping Kotlin exceptions terminate the program. | missing | Covers listed exception types and termination for other types, but does not explicitly cover subclasses. |
| Suspend completion handlers always have an error parameter; without @Throws only CancellationException propagates, while non-suspend exceptions do not propagate. | incorrect | Default-termination discussion and suspend caveat omit the unannotated CancellationException exception and fail to say all suspend completion handlers expose errors. |

Unsupported/material claims: NSError domain/userInfo structure, callback exception details and cross-version stability assertions are not established by the reviewed section. The IOException snippet needs a suitable Native type. The answer explicitly notes its lack of evidence, but that does not verify these extras.

Citation review: Not applicable: plain mode has no supplied references.

## q14 — plain

How does Swift export expose Kotlin suspend functions?

## Short answer

With the **Swift export** (the experimental direct Swift API, distinct from the older Objective‑C interop), a Kotlin `suspend` function becomes a **native Swift `async throws` function** that you call with `await` / `try await`. The continuation and `CoroutineContext` are not exposed.

```kotlin
// Kotlin
suspend fun loadName(id: Int): String
```

```swift
// Swift (Swift export)
let name = try await Shared.loadName(id: 1)   // func loadName(id: Int32) async throws -> String
```

Notes:
- It is `throws` because Kotlin suspend calls can complete exceptionally (cancellation, uncaught exceptions); there is no other error channel.
- Cancelling the surrounding Swift `Task` propagates cancellation into the Kotlin coroutine.
- The coroutine still runs on a Kotlin dispatcher; you don't get to choose a Swift executor for it.

## Contrast with Objective‑C interop (the default, stable path)

When Kotlin is exported to Swift *via the generated Objective‑C framework*, suspend functions are **not** `async`. They become a function taking a completion handler:

```swift
// ObjC interop today
func loadName(id: Int32, completionHandler: @escaping (String?, Error?) -> Void)
```

You typically wrap that in your own `withCheckedThrowingContinuation` to get `async` ergonomics. The result is nullable because cancellation delivers `nil` result plus an error.

## Version caveats

- **Swift export is experimental.** It was introduced around Kotlin 2.0/2.1 and became more usable in 2.2.x, but the generated API shape, the Gradle DSL (the `swiftExport { ... }` block, module naming, the experimental opt‑in flag) and support coverage are still changing between releases. Pin your Kotlin version and check that release's docs.
- Swift export currently covers a subset of Kotlin features; **`Flow`, `StateFlow`, etc., and full generic/sealed-class fidelity are not on par with ObjC interop** — some of these are planned (e.g. mapping `Flow` onto `AsyncSequence`) but I would not rely on a specific mapping without verifying against your Kotlin version's release notes.
- If you must ship today and need broad coverage, the Objective‑C interop + completion‑handler bridging remains the safe choice.

I'm confident about the `suspend -> async throws` mapping and about the completion‑handler shape for ObjC interop; I'm less certain about the exact current DSL names and the precise error type Swift export uses for cancellation, so verify those against the docs for your Kotlin release.

Review source: [reports/task1/sources/native-swift-export.txt](../sources/native-swift-export.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Swift export maps Kotlin suspend functions and suspending functional types to Swift async counterparts. | missing | Correctly maps suspend functions to async/try await, but does not mention suspending functional types, part of the pre-recorded fact. |
| Swift calls the exported suspend function with try await, as shown in the saved guide. | satisfied | Shows try await Shared.loadName(id: 1). |
| Swift export is Alpha; distinguish its direct Swift mapping from the Objective-C completion-handler bridge. | incorrect | Distinguishes direct export from Objective-C but calls the feature experimental instead of the snapshot Alpha status, and categorically says the Objective-C path is not async despite the guide’s Swift 5.5 async callability caveat. |

Unsupported/material claims: Says Flow-to-AsyncSequence support is planned, contradicting the saved guide lines 404–418. The specific introduction timeline and Swift Task cancellation claim are not verified by the reviewed section. Version uncertainty is acknowledged but does not correct the categorical outdated statements.

Citation review: Not applicable: plain mode has no supplied references.

## q14 — rag

How does Swift export expose Kotlin suspend functions?

**Short answer:** With Swift export, Kotlin `suspend` functions (and suspend functional types) are exported as Swift `async` counterparts, so you call them with `await` [1][2].

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
The same before/after example appears in both the Swift export docs [1] and the 2.4.0 release notes, which describe "seamlessly call[ing] suspending Kotlin code from Swift" with suspend functions and suspend functional types mapped to "Swift's idiomatic async counterparts" [2].

**Runtime behavior:** by default, the coroutine context created when you call a suspend function from Swift uses `Dispatchers.Default`; use `withContext()` in Kotlin to switch dispatchers [5].

**Version caveats and distinctions:**
- Swift export's improved concurrency / structured-concurrency support is described as going **Alpha** in Kotlin 2.4.0 [2]; the other Swift export pages carry no release/availability information in the provided snapshots [1][5]. Treat availability as unverified for your target version.
- This differs from the older Objective-C interop path, where suspend functions are presented as callback/completion-handler functions, with Swift `async` callability being "highly experimental" and subject to limitations [4].
- Don't confuse this with the Kotlin/JS `@JsExport` suspend-function export, which requires `-Xenable-suspend-function-exporting` and surfaces as JavaScript `async` functions returning `Promise` [3] — a different target and opt-in mechanism.

Retrieved references:

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), normalized lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 404–425; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), normalized lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

Review source: [reports/task1/sources/native-swift-export.txt](../sources/native-swift-export.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Swift export maps Kotlin suspend functions and suspending functional types to Swift async counterparts. | satisfied | Explicitly maps functions and suspend functional types to Swift async. |
| Swift calls the exported suspend function with try await, as shown in the saved guide. | satisfied | Uses the documented try await hello() example. |
| Swift export is Alpha; distinguish its direct Swift mapping from the Objective-C completion-handler bridge. | satisfied | States Alpha in 2.4.0 and carefully separates Objective-C completion handlers with experimental async callability. |

Unsupported/material claims: The complete 2.4.0 source at lines 588–590 confirms Alpha status, but the retrieved [2] starts later. No material contradiction found; dispatcher and JS distinctions are supported by the relevant full sections.

Citation review: All [1]–[5] labels resolve. [1]/[2] support async mapping, [5] dispatcher, [4] Objective-C distinction, [3] JS flag. Alpha status is not in the cited [2] chunk, just as in the Pro RAG answer; it is supported only by the complete source.

## q16 — rag

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use Kotlin **`external` declarations** to describe an existing JavaScript API. They tell the compiler the API exists, but you provide no Kotlin implementation [1].

Example:

```kotlin
external fun alert(message: String)

external class JsDate {
    fun getTime(): Double
}

fun main() {
    alert("Hello from Kotlin")
    val time = JsDate().getTime()
}
```

For a JavaScript object that is callable like a function, use `@nativeInvoke` on a member function of an `external` class/interface [4]:

```kotlin
import kotlin.js.nativeInvoke

@OptIn(ExperimentalWasmJsInterop::class)
external class JsAction {
    @nativeInvoke
    operator fun invoke(data: String)
}

fun main() {
    val action = JsAction()
    action("Run task")
}
```

Caveats: `@nativeInvoke` is currently a temporary solution, and using it reports a compiler warning [4]. The provided docs do not specify version compatibility or availability beyond the snapshots cited; release versions are not feature-availability guarantees.

Retrieved references:

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 79–98; Use Kotlin code from JavaScript > Package structure > @JsExport annotation
- [3] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 170–194; Use Kotlin code from JavaScript > Package structure > @JsNoRuntime annotation
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [5] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), normalized lines 705–719; What's new in Kotlin 2.3.20 > Kotlin/JS > Implementing Kotlin interfaces from JavaScript/TypeScript > How to enable

Review source: [reports/task1/sources/js-interop.txt](../sources/js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use external on declarations whose implementation is supplied by JavaScript. | satisfied | Recommends external declarations for an existing JS API. |
| Do not provide a Kotlin body; the compiler emits no implementation and JavaScript must supply it. | satisfied | Explains no Kotlin implementation and shows body-free functions/members. |
| external is package-level and inherited by nested members; optional JS arguments can use definedExternally. | missing | Does not explain package-level placement, inherited external status or definedExternally defaults. |

Unsupported/material claims: The nativeInvoke example comes from Kotlin/Wasm and is not clearly labeled as such in the answer, creating a target-confusion risk. JsDate is an illustrative declaration that requires an actual matching JS implementation; it is not a documented binding to built-in Date. Otherwise the core advice is supported.

Citation review: Used [1], [4] resolve. [1] is only the short external-declarations introduction. [4] directly supports the callable-object example and temporary/warning caveat, but belongs to Wasm. The blanket lack-of-version-evidence caveat accurately reflects supplied metadata.

## q16 — plain

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use the `external` modifier. An `external` declaration tells the Kotlin/JS compiler that the implementation exists in JavaScript — Kotlin will emit a call to it and won't generate a body. It works for functions, properties, classes, and objects.

```kotlin
external fun alert(message: String)

external class Foo {
    val bar: Int
    fun baz(x: String): Boolean
}

external object MyApi {
    fun doThing(a: Int): Int
}
```

Rules and caveats:

- `external` members cannot have a body.
- For an `external class`, all members must also be `external`.
- Only `external` declarations can be top-level without a body.
- Extension functions/properties on `external` types are fine.

### Naming and location

Kotlin keeps the declared name by default, but JS is often namespaced or minified, so use annotations:

```kotlin
@JsModule("moment")            // import from npm module
external fun moment(): Moment

@JsName("apply")               // JS name differs from Kotlin name
external fun applyTo(target: Any)

@JsGlobal("jQuery")            // global variable (needs @JsNonModule for npm-less globals)
external val jQuery: JQuery

@JsQualifier("mylib.util")     // prefix for plain (non-module) externals
external fun helper(x: Int)
```

- `@JsModule("name")` — resolves via ES module import.
- `@JsNonModule` — allows using the declaration from plain `<script>` globals when no module is available.
- `@JsGlobal` — resolves against the global object (window, etc.).

### Typing the result

If you don't want to model the JS value, use `dynamic`:

```kotlin
external fun fetchData(): dynamic
// or
external val config: dynamic
```

`dynamic` disables compile-time checking on that value. It's convenient but loses type safety; prefer real `external` types when you can.

### Version notes

- `-Xir-*` (IR) vs. legacy backend doesn't change `external` semantics, but naming/import behavior has had fixes across 1.4–1.9; if you hit odd resolution issues, check the exact Kotlin version.
- `@JsModule` on functions/classes was added around Kotlin 1.1; `@JsNonModule` came later (1.3+) for the non-module fallback. On recent versions (2.x) these are stable.

Basic pattern: mark it `external`, add `@JsModule` (or `@JsGlobal`/`@JsQualifier`) if the JS name isn't a plain global, and define as much or as little structure as you need.

Review source: [reports/task1/sources/js-interop.txt](../sources/js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use external on declarations whose implementation is supplied by JavaScript. | satisfied | Uses external for JavaScript implementations. |
| Do not provide a Kotlin body; the compiler emits no implementation and JavaScript must supply it. | satisfied | Says Kotlin emits no body and gives external declarations. |
| external is package-level and inherited by nested members; optional JS arguments can use definedExternally. | missing | Does not explain the package-level restriction or definedExternally. Says class members must also be external without clarifying that this is inherited automatically. |

Unsupported/material claims: The claimed JsGlobal annotation and version timeline are unsupported by the saved sources. The statement only external declarations can be top-level without bodies is contradicted by the expect declarations in the same corpus. Additional module/naming examples were not compiled.

Citation review: Not applicable: plain mode has no supplied references.

## q17 — plain

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

## The mechanism: `JsReference`

In Kotlin/Wasm, JS cannot freely hold a Kotlin object across calls, because the Wasm GC doesn't know about references kept only on the JS side. The bridge for this is `kotlin.js.JsReference<T>`: you wrap the object, hand the (opaque) reference to JS, and JS holds *that* instead. The Kotlin object stays alive as long as the reference is unreleased.

```kotlin
// Kotlin/Wasm
import kotlin.js.ExperimentalWasmJsInterop
import kotlin.js.JsReference

class Foo(val name: String)

@OptIn(ExperimentalWasmJsInterop::class)
@JsExport
fun makeFoo(name: String): JsReference<Foo> = JsReference(Foo(name))

@OptIn(ExperimentalWasmJsInterop::class)
@JsExport
fun greet(ref: JsReference<Foo>): String = "Hello, ${ref.get().name}"
```

```js
// JavaScript
import { makeFoo, greet } from "./myModule.mjs";

const ref = makeFoo("world"); // JS now owns a reference
greet(ref);                   // pass it back when needed
```

Key points:

- **JS sees an opaque handle**, not a normal object — you can store it, pass it around, and hand it back to Kotlin, but you can't read its fields from JS.
- **You must call `release()`** on the Kotlin side when JS is done with it, otherwise the object is effectively leaked (the Wasm GC can't collect it while a live reference exists).
- **Primitives and strings** need no wrapping; only objects do.

## Caveats

- `JsReference` and `@ExperimentalWasmJsInterop` are **experimental** (introduced with the Kotlin/Wasm JS interop work around Kotlin 1.9.20) and the surface has shifted across versions — verify the exact method names (`get()`, `release()`) against the docs for your compiler version.
- Only applicable in **Kotlin/Wasm**, not JS (Kotlin/JS has no such barrier).

## Portable alternative: a handle table

If you'd rather not depend on the experimental API (or you're targeting multiple backends), keep Kotlin objects in a `Map<Int, Foo>` inside Wasm and expose only integer handles:

```kotlin
private val registry = mutableMapOf<Int, Foo>()
private var nextId = 0

@JsExport
fun createFoo(name: String): Int {
    val id = nextId++
    registry[id] = Foo(name)
    return id
}

@JsExport
fun greet(id: Int): String = "Hello, ${registry.getValue(id).name}"

@JsExport
fun dispose(id: Int) { registry.remove(id) }
```

This is more verbose but works on any Kotlin/Wasm version and makes lifetime management explicit on both sides.

Review source: [reports/task1/sources/wasm-js-interop.txt](../sources/wasm-js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use JsReference<T> to pass an opaque handle to a Kotlin object into JavaScript. | satisfied | Correctly names JsReference<T> and an opaque handle instead of exposing fields directly. |
| Create the handle with toJsReference() and expose it through an exported function. | missing | Uses JsReference(Foo(name)) rather than the documented toJsReference() factory; the proposed constructor is not established by the source. |
| JavaScript cannot directly access the Kotlin object; exported Kotlin helpers unwrap the handle using get(). | satisfied | Uses exported helpers receiving the handle and get(), and says JS cannot read object fields. |

Unsupported/material claims: The mandatory release() API, leak/GC explanation and exact historical/experimental claims are not supported by the guide. Named ESM imports conflict with its default-export-object mapping. The alternative integer registry does not establish the claimed all-versions guarantee. Correctly identifying JsReference does not make this sample reliable.

Citation review: Not applicable: plain mode has no supplied references.

## q17 — rag

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Use `JsReference<T>` together with `toJsReference()`.

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

From JavaScript, the returned value is an opaque reference: it behaves like an empty frozen JavaScript object. To operate on it, export more Kotlin functions that accept `JsReference<User>` and unwrap it with `get()` [1].

```javascript
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

So Kotlin/Wasm holds the Kotlin object behind a `JsReference`; JavaScript only holds the opaque handle, and Kotlin-side exported functions mediate access [1][5].

Version note: the cited guide snapshot has unknown release and availability, so verify against your Kotlin version.

Retrieved references:

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 99–117; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External objects
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 164–176; Interoperability with JavaScript > Use JavaScript code in Kotlin > Kotlin functions with JavaScript code
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type

Review source: [reports/task1/sources/wasm-js-interop.txt](../sources/wasm-js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use JsReference<T> to pass an opaque handle to a Kotlin object into JavaScript. | satisfied | Uses JsReference<T> and describes an opaque empty frozen object. |
| Create the handle with toJsReference() and expose it through an exported function. | satisfied | Creates the exported handle with User(name).toJsReference(). |
| JavaScript cannot directly access the Kotlin object; exported Kotlin helpers unwrap the handle using get(). | satisfied | Calls get() inside an exported helper and uses that helper from JS. |

Unsupported/material claims: No material unsupported claim found; version availability is explicitly qualified.

Citation review: Used [1], [5] resolve and directly support the Kotlin handle operations and default-import JavaScript helper example.

## q18 — rag

When did the common Kotlin UUID API become stable?

The common Kotlin `kotlin.uuid.Uuid` API became **Stable in Kotlin 2.4.0**. The only exceptions are the functions for generating V4 and V7 UUIDs, which remain Experimental and still require opt-in [1].

Retrieved references:

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [3] [What's new in Kotlin 2.1.20](https://kotlinlang.org/docs/whatsnew2120.html), normalized lines 323–365; What's new in Kotlin 2.1.20 > Standard library > Changes in UUID parsing, formatting, and comparability
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 658–684; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing > New functions to generate v4 and v7 UUIDs

Review source: [reports/task1/sources/whatsnew24.txt](../sources/whatsnew24.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| The common kotlin.uuid.Uuid API became Stable in Kotlin 2.4.0. | satisfied | States stable in Kotlin 2.4.0. |
| V4 and V7 generation functions remain Experimental and require opt-in. | satisfied | Retains Experimental/opt-in caveats for V4 and V7 generators. |
| The API was introduced experimentally in Kotlin 2.0.20; introduction and stabilization differ. | missing | Omits experimental introduction in 2.0.20; this is background completeness rather than a wrong core date answer. |

Unsupported/material claims: No material unsupported claim found.

Citation review: The sole reference [1] resolves and directly supports both stability and exceptions.

## q18 — plain

When did the common Kotlin UUID API become stable?

The common `kotlin.uuid.Uuid` API became **stable in Kotlin 2.1.0** (released November 2024).

It was first introduced as an experimental API in **Kotlin 2.0.20** with `@ExperimentalUuidApi`; from 2.1.0 onward you no longer need that opt-in for the stable API.

Example on Kotlin 2.1.0+:

```kotlin
import kotlin.uuid.Uuid

val id = Uuid.random()
val parsed = Uuid.parse("550e8400-e29b-41d4-a716-446655440000")
```

On Kotlin 2.0.20–2.0.x, you had to opt in:

```kotlin
@OptIn(ExperimentalUuidApi::class)
val id = Uuid.random()
```

Review source: [reports/task1/sources/whatsnew24.txt](../sources/whatsnew24.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| The common kotlin.uuid.Uuid API became Stable in Kotlin 2.4.0. | incorrect | Claims stability in 2.1.0; the complete saved source says 2.4.0. |
| V4 and V7 generation functions remain Experimental and require opt-in. | incorrect | Says no opt-in is needed from 2.1.0 and omits V4/V7 exceptions, contrary to the release-note history and 2.4.0 caveat. |
| The API was introduced experimentally in Kotlin 2.0.20; introduction and stabilization differ. | satisfied | Correctly identifies experimental introduction in 2.0.20, while getting stabilization wrong. |

Unsupported/material claims: The resulting 2.1.0+ no-opt-in advice is materially wrong against the snapshot. The specific calendar date is not verified by the reviewed section.

Citation review: Not applicable: plain mode has no supplied references.

## q19 — plain

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

I can’t verify any change in **Kotlin 2.4.20** for exporting suspending lambdas to JavaScript: that version is beyond my knowledge cutoff (June 2024), and I have no release notes for it.

As of the Kotlin/JS behavior I know, `@JsExport` does **not** support `suspend` function types directly. Suspending functions/lambdas compile to coroutine state machines taking a `Continuation`, so to expose them to JS you normally wrap them and return a `Promise`, e.g.:

```kotlin
@JsExport
fun runSuspend(block: suspend () -> Unit): Promise<Unit> =
    GlobalScope.promise { block() }
```

If Kotlin 2.4.20 did add first-class exporting of suspending lambdas, the likely documented change would be a `Promise`-based bridge in generated JS. But I can’t confirm that from my training data—check the official Kotlin 2.4.20 release notes.

Review source: [reports/task1/sources/whatsnew2420.txt](../sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 supports exporting suspending lambdas as JavaScript async functions. | missing | Declines to confirm the 2.4.20 change instead of identifying it. |
| Enable -Xsuspend-lambda-exporting. | missing | Does not provide -Xsuspend-lambda-exporting. |
| Annotate relevant declarations @JsExport; compiler bridges suspend lambdas with JS async/await. | missing | Speculates about Promise bridging but does not establish the new @JsExport async-lambda mechanism. |

Unsupported/material claims: The uncertainty is explicit, which is preferable to an invented release claim, but the example still exports a suspend-typed parameter while describing an older system that cannot export such declarations. Returning Promise does not remove that incompatible parameter. Its claimed model cutoff is self-report, not independently established metadata.

Citation review: Not applicable: plain mode has no supplied references.

## q19 — rag

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

## What changed in Kotlin 2.4.20

**Before 2.4.20:** There was no way to export declarations containing suspending lambdas from Kotlin/JS libraries [1]. Separately, exporting plain `suspend` functions via `@JsExport` became possible earlier, in Kotlin 2.3.0 [4].

**In 2.4.20:** You can export suspending lambda expressions as JavaScript `async` functions [1]. The Kotlin compiler automatically bridges Kotlin `suspend` functions and JavaScript's native `async`/`await` model, which is aimed at mixed Kotlin/TypeScript codebases [1]. This is listed among the Kotlin/JS improvements in the release [2].

### Enabling it

Add the compiler option to `build.gradle.kts` [1][5]:

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

Then mark the relevant declaration with `@JsExport` [3]:

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

### Caveats

- The reference snapshots note that availability should be checked against individual feature status in the source, and that a release version is not a snippet compatibility guarantee [1][2][3].
- The `-Xsuspend-lambda-exporting` flag is an experimental-style compiler option; treat its behavior and stability as version-dependent (the reference data does not state a stability guarantee).

Retrieved references:

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 396–402; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [5] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 133–159; Use Kotlin code from JavaScript > Package structure > @JsExport annotation > Export suspending lambdas

Review source: [reports/task1/sources/whatsnew2420.txt](../sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 supports exporting suspending lambdas as JavaScript async functions. | satisfied | Identifies 2.4.20 suspending-lambda export as JS async functions. |
| Enable -Xsuspend-lambda-exporting. | satisfied | Includes the exact -Xsuspend-lambda-exporting flag. |
| Annotate relevant declarations @JsExport; compiler bridges suspend lambdas with JS async/await. | satisfied | Shows the documented @JsExport TaskRunner and async callback. |

Unsupported/material claims: Core answer is supported. It infers experimental-style status from the flag name; actual feature status is not stated in those chunks, although the complete release summary lists it among pre-stable features. The earlier 2.3.0 comparison omits that feature’s own enabling flag.

Citation review: All [1]–[5] labels resolve and support the principal feature, flag, code and historical statements. The caveat attributed to [1]/[2]/[3] about release versions not guaranteeing compatibility comes from the harness evidence instructions, not those source chunks; this is an attribution gap.

## q20 — rag

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

**Cross-language inheritance in Swift export** is the feature you're looking for [1].

Kotlin 2.4.20 lets a Swift implementation inherit from an exported Kotlin superclass and implement a Kotlin interface, then be passed back into Kotlin code that accepts that interface — the "reverse import" pattern, useful when only pure Swift libraries (e.g. CryptoKit) can provide the implementation [1]:

```kotlin
// Kotlin
interface CryptoProvider {
    fun hashMD5(input: String): String
}

fun processHash(provider: CryptoProvider, input: String): String =
    provider.hashMD5(input)

open class SwiftBase
```

On the Swift side you subclass `SwiftBase` and conform to `CryptoProvider`, then hand that object to `processHash`. This is one of the new Swift export features in this release, alongside sealed class/interface support and automatic `Package.swift` generation for SwiftPM dependencies [2][3].

Caveat: the source lists Swift export features as pre-stable in this release (see individual feature status in the source), and Swift export itself was introduced as experimental in Kotlin 2.2.20 [4][5]. Release version alone doesn't guarantee a given snippet or feature works in your setup.

Retrieved references:

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [4] [What's new in Kotlin 2.2.20](https://kotlinlang.org/docs/whatsnew2220.html), normalized lines 432–452; What's new in Kotlin 2.2.20 > Kotlin Multiplatform > Swift export available by default
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 39–59; What's new in Kotlin 2.4.20 > New features

Review source: [reports/task1/sources/whatsnew2420.txt](../sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 introduces cross-language inheritance in Swift export. | satisfied | Names cross-language inheritance in Swift export and 2.4.20. |
| The reverse-import pattern declares a Kotlin interface and open Kotlin base class; Swift subclasses the base and implements the interface. | satisfied | Uses an open Kotlin base and Kotlin interface implemented on the Swift side. |
| Pass the Swift instance to Kotlin functions accepting the interface so Kotlin dispatches to Swift code. | satisfied | Explicitly passes the Swift instance back to processHash accepting the interface. |

Unsupported/material claims: No material unsupported claim found. Package.swift generation is mentioned alongside Swift export features, but belongs to the broader Kotlin/Native release summary rather than inheritance itself.

Citation review: All used references [1]–[5] resolve. [1] supports reverse-import steps, [2]/[3] nearby release features, [4] 2.2.20 experimental introduction and [5] pre-stable feature status.

## q20 — plain

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

It’s the **Swift export inheritance support** — usually described in the Kotlin/Native Swift export release notes as **“subclassing Kotlin classes and implementing Kotlin interfaces from Swift.”** It lets Swift types inherit from exported open Kotlin classes and conform to exported Kotlin interfaces.

Example:

```kotlin
// Kotlin
open class Greeter {
    open fun greet(): String = "Hello"
}
```

```swift
// Swift
class MyGreeter: Greeter {
    override func greet() -> String { "Hi" }
}
```

Caveat: Kotlin 2.4.20 is beyond my reliable knowledge, and the exact release-note label may vary slightly (e.g. “Swift export: inheritance” vs. “subclassing Kotlin classes from Swift”). The capability itself is Swift-side subclassing/implementation of exported Kotlin types.

Review source: [reports/task1/sources/whatsnew2420.txt](../sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 introduces cross-language inheritance in Swift export. | satisfied | Correctly describes the requested Swift inheritance capability, while qualifying uncertainty about the exact release label. |
| The reverse-import pattern declares a Kotlin interface and open Kotlin base class; Swift subclasses the base and implements the interface. | satisfied | States Swift inherits exported open classes and implements exported Kotlin interfaces; the brief example only illustrates subclassing. |
| Pass the Swift instance to Kotlin functions accepting the interface so Kotlin dispatches to Swift code. | missing | Does not explain handing the Swift instance back to Kotlin via the interface. |

Unsupported/material claims: No material unsupported setup flag is introduced. The method-override example is illustrative and was not compiled. Release uncertainty is clearly stated.

Citation review: Not applicable: plain mode has no supplied references.

