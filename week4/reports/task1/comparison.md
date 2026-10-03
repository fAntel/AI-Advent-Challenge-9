# RAG answer comparison

Ten independent pairs, DeepSeek generation only. Judgments were authored separately by the implementing assistant against saved source sections. This is illustrative, not a broad reliability claim.

Index SHA-256: `8249a981e64ba378fac6690ca46299545fa68f054cfb5ccbe5f311119104040d`

Settings: `{"approach": "none", "compression": false, "context_strategy": "full", "model": "deepseek-v4-pro", "rag_budget": 6000, "rag_limit": 5, "rag_strategy": "semantic", "reasoning": "low"}`

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

My review found **7 improvements, 1 regression, 1 mixed result and 1 largely unchanged result** with RAG. The strongest gains correct outdated context syntax, Swift export mapping, Wasm object handling, UUID stabilization and the required suspending-lambda export flag. RAG is not uniformly better: q12 invents a restriction on actual typealias despite citing sources. Citation labels all resolve, but some cited chunks do not support the full attached claim (notably q12 and the Alpha claim in q14). These are judgments about this one selected run against the saved corpus, not a general accuracy estimate.

| Question | RAG outcome | My comparison |
| --- | --- | --- |
| q01 | improved | Plain uses old context(Logger) receivers and misses ambiguity. RAG uses context(users: UserService), type-based call-site matching and same-scope ambiguity; all three expected facts are covered. |
| q11 | mixed | Both explain common/platform/intermediate placement. Plain explicitly covers signature matching but adds unsupported fine-grained modifier rules. RAG stays closer to source text yet omits explicit signature matching. Neither dominates on both completeness and grounding. |
| q12 | regressed | Plain correctly says an existing class must already provide compatible members. RAG falsely limits actual typealias to name-only declarations and prescribes inheritance when expect has any members. Adding same-package guidance does not offset this core error. |
| q13 | improved | RAG supplies the listed-exception/subclass restriction and CancellationException exception for suspend calls. Plain omits both and overgeneralizes unannotated-function termination. |
| q14 | improved | Plain confuses direct Swift export with callbacks and proposes unsupported @SwiftAsync/-Xswift-async controls. RAG gives native async and try await correctly. Its Alpha statement is true in the complete source but not supported by cited chunk [2]. |
| q16 | largely unchanged | Both correctly recommend external with no Kotlin implementation; both miss part of the detailed declaration rules. RAG again retrieves Wasm neighbors instead of the designated Kotlin/JS section, adding an overbroad experimental-status claim. Plain adds unverified plugin-version guidance. No clear core-answer gain. |
| q17 | improved | Plain wrongly exports a Kotlin class directly and invokes methods from JS. RAG uses JsReference, toJsReference(), opaque JS handles and exported helpers calling get(), matching the Wasm guide. |
| q18 | improved | Plain says stable in 2.0.20 and introduced in 1.9.0; the source says introduced in 2.0.20 and stable in 2.4.0. RAG gives the correct stability release and V4/V7 exceptions, though it omits introduction history. |
| q19 | improved | Both identify the lambda export feature. Plain claims it works by default; RAG includes the required -Xsuspend-lambda-exporting flag and documented @JsExport/async example. |
| q20 | improved | Both identify Swift inheritance. RAG accurately names cross-language inheritance and includes interfaces alongside the open base class; plain adds an unsupported flag and stability/default speculation. Both omit passing the Swift implementation back into Kotlin. |

## Method and limits

Saved Task0 source sections and full retained source snapshots, rather than presumed model knowledge. Expected facts were written before generation. No Kotlin/Swift examples were compiled. Missing means an expected fact is absent or only partly covered; incorrect means contradicted by the saved source. Unsupported does not automatically mean false.

Each question ran independently through the rebuilt `week4/advent-agent rag compare` path and the normal DeepSeek adapter. Both arms received identical base instructions, `deepseek-v4-pro`, low reasoning and no temperature override. Only RAG received evidence from `week4/.venv/bin/kotlin-index`, using semantic retrieval, five results and a 6,000-character whole-chunk budget. There was no session history, memory, project instruction, lifecycle or MCP context; expected facts were withheld. Mode order alternated. Content was assessed before assigning paired outcomes. Reviews are bound to the entire saved answer record by SHA-256.

The saved corpus is the reference standard, not a claim about live documentation today. Source pages can contain omissions or inconsistencies. Completeness checks include some useful caveats beyond the literal short question; missing such a caveat is distinguished from an incorrect core answer. No examples were compiled. Latencies below measure provider calls only and do not include local retrieval.

Estimated generation cost: $0.024602. Embedding work excluded.

| Mode | Answers | Provider seconds | Total tokens | Estimated USD |
| --- | ---: | ---: | ---: | ---: |
| plain | 10 | 188.91 | 18164 | 0.015547 |
| rag | 10 | 52.19 | 16282 | 0.009055 |

Observed cost and latency differences are not controlled performance estimates; answer lengths, reasoning and cache usage vary.

## Answers and individual reviews

## q01 — plain

How can a function require an implicit service and how is that service resolved?

In Kotlin, you can make a function require an implicit service using **context receivers** (experimental since Kotlin 1.6.20, still experimental as of Kotlin 2.x). The service is provided by the lexical scope at the call site and resolved statically by the compiler.

### Defining a function with an implicit service

Prefix the `fun` keyword with `context(ServiceType)`:

```kotlin
interface Logger {
    fun log(msg: String)
}

class ConsoleLogger : Logger {
    override fun log(msg: String) = println(msg)
}

// Function requires an implicit Logger
context(Logger)
fun greet(name: String) {
    log("Hello, $name!")   // 'log' is available from the context receiver
}
```

### Providing and resolving the service

You must call the function inside a scope where the required context receiver is available. The most common way is using `with`:

```kotlin
fun main() {
    val logger = ConsoleLogger()
    with(logger) {
        greet("World")   // 'logger' is supplied as the implicit Logger
    }
}
```

You can also propagate the requirement to another context receiver:

```kotlin
context(Logger)
fun runApp() {
    greet("Kotlin")   // Logger context is available here
}
```

### Resolution rules

- The compiler looks for a context receiver of the exact declared type (or a subtype) in the current lexical scope.
- If multiple context receivers are declared, all must be present in the surrounding scope.
- Resolution is **static** and **compile‑time** based on types, not runtime lookup.

### Traditional alternatives (without experimental features)

- **Extension functions** create an implicit receiver on the service type:

    ```kotlin
    fun Logger.greet(name: String) {
        log("Hello, $name!")
    }

    // Usage:
    val logger = ConsoleLogger()
    with(logger) {
        greet("World")
    }
    ```

- **Member functions** of a class that holds the service as a property.

Context receivers offer a more general and composable way to express implicit dependencies, but they require enabling the compiler flag `-Xcontext-receivers` or the corresponding Gradle option (`freeCompilerArgs`).

Review source: [reports/task1/sources/context-parameters.txt](sources/context-parameters.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Declare context(name: Service) on the function for an implicit dependency. | incorrect | Uses context(Logger) and calls context receivers the current experimental Kotlin 2.x feature. The snapshot instead introduces named context parameters (source lines 3–9). |
| Resolution happens at the call site by matching context values by type. | satisfied | Explains static call-site resolution by type in the lexical scope. |
| Multiple compatible values at the same scope level cause ambiguity. | missing | Does not mention ambiguity between compatible values at the same scope level (source line 63). |

Unsupported/material claims: Material outdated guidance: recommends -Xcontext-receivers and the old receiver syntax without identifying the replacement. The claimed historical introduction date is not established by the reviewed guide.

Citation review: Not applicable: no evidence supplied or citations requested to invented references.

## q01 — rag

How can a function require an implicit service and how is that service resolved?

A function can require an implicit service using **context parameters**: declare it with `context(name: Type)` before the function (or property) [5].

```kotlin
interface UserService { fun log(message: String) }

context(users: UserService)
fun outputMessage(message: String) {
    users.log("Log: $message")
}
```

Resolution happens **at the call site** by searching the current scope for a matching value **by type**. If exactly one `UserService` is in scope, it is used implicitly; if multiple compatible values exist at the same scope level, the compiler reports an ambiguity error [3]. You can provide the value with a `context(...)` block [3][5].

```kotlin
val serviceA = object : UserService { ... }

context(serviceA) {
    outputMessage("Hello")   // serviceA is resolved as UserService
}
```

If you don’t need to name the parameter, use `_` and retrieve it explicitly via `contextOf<T>()` when needed [1].

Note: context parameters replace the older experimental “context receivers” feature [5].

Retrieved references:

- [1] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 48–59; Context parameters
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 475–509; What's new in Kotlin 2.4.0 > Standard library > New map fallback functions to distinguish null values and missing keys
- [3] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 61–92; Context parameters > Context parameters resolution
- [4] [Extensions](https://kotlinlang.org/docs/extensions.html), normalized lines 206–221; Extensions > Extension functions > Anonymous extension functions
- [5] [Context parameters](https://kotlinlang.org/docs/context-parameters.html), normalized lines 1–46; Context parameters

Review source: [reports/task1/sources/context-parameters.txt](sources/context-parameters.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Declare context(name: Service) on the function for an implicit dependency. | satisfied | Uses context(users: UserService), matching source lines 9–21. |
| Resolution happens at the call site by matching context values by type. | satisfied | Explicitly says resolution happens at the call site by type. |
| Multiple compatible values at the same scope level cause ambiguity. | satisfied | Explicitly explains ambiguity at the same scope level. |

Unsupported/material claims: No material unsupported factual claim found. The serviceA example contains an ellipsis and is illustrative, not runnable as printed.

Citation review: All used references [1], [3], [5] identify supplied chunks. [5] supports named syntax and replacement of receivers; [3] supports resolution/ambiguity and context blocks; [1] supports _ and contextOf. Unrelated chunks [2] and [4] are correctly unused.

## q11 — rag

Where should expect and actual declarations live and which parts must match?

Expected and actual declarations are used in Kotlin Multiplatform to provide platform-specific implementations for common code [3].

- **Where they live**:  
  - The `expect` declaration lives in the **common source set** (e.g., `commonMain`). It has no implementation.  
  - The `actual` declaration lives in **platform-specific source sets** (e.g., `jvmMain`, `nativeMain`) or in an **intermediate shared source set** (e.g., `iosMain` shared between `iosArm64Main` and `iosSimulatorArm64Main`) [1][2].

- **Which parts must match**:  
  - The declarations must be the **same construct** (function, property, class, interface, enum, or annotation) [1].  
  - They must be in the **same package**, e.g., `org.mygroup.myapp.MyType` [2].  
  - Every `expect` declaration must have a matching `actual` declaration in **every platform-specific source set** [2].  
  - Expected declarations must contain **no implementation** [1][2].

For example:

```kotlin
// commonMain
expect class Identity() {
    val userName: String
}

// jvmMain
actual class Identity {
    actual val userName: String = System.getProperty("user.name") ?: "None"
}
```

During compilation, the compiler merges each `expect` with its corresponding `actual` for the target platform [2]. Note: expected/actual classes are in **Beta**, so minor migration steps may still be needed [4]. Expected annotation classes are also supported, sometimes via `actual typealias` to a platform type [5].

Retrieved references:

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 7–15; Expected and actual declarations > Rules for expected and actual declarations
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 17–27; Expected and actual declarations > Rules for expected and actual declarations
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 1–5; Expected and actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 196–218; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 389–404; Expected and actual declarations > Advanced use cases > Expected annotation classes

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| expect belongs in common code and has no implementation. | satisfied | States common source set and no implementation. |
| actual implementations belong in platform or shared intermediate source sets. | satisfied | Explicitly includes intermediate iosMain shared by two iOS targets. |
| Corresponding declarations need matching signatures and the same package, covering each target. | missing | Covers package and target coverage, but never explicitly explains matching signatures, which the complete source mentions at line 35. Its example demonstrates one signature only. |

Unsupported/material claims: No material unsupported claim found. The every-platform-source-set wording is qualified by the earlier intermediate-source-set explanation. The Beta caveat is supported by the supplied class section.

Citation review: References [1]–[5] all resolve to supplied chunks and support the associated claims about placement, package matching, compiler merging, Beta status and annotation aliases. Lack of retrieved signature detail is a coverage gap, not a fabricated citation.

## q11 — plain

Where should expect and actual declarations live and which parts must match?

Expect declarations live in common source sets, usually `commonMain`, and are marked with `expect`.  
Actual declarations live in platform-specific source sets such as `jvmMain`, `jsMain`, `iosMain`, or in intermediate shared source sets like `nativeMain`, and are marked with `actual`.  
There must be one `actual` declaration for every platform target that uses the common code.

### What must match

- **Fully qualified name**: same package and same simple name.
- **Kind**: class, interface, object, function, or property must be the same kind.
- **Type signature**:
  - Functions: same parameter types, return type, and receiver/context receivers.
  - Properties: same type and mutability (`val` vs `var`).
- **Generic type parameters**: same number, same bounds/constraints.
- **Visibility**: `actual` must be at least as visible as `expect` (less restrictive or equal).  
  Example: `expect internal fun foo()` may be implemented by `actual public fun foo()`, but not by `actual private fun foo()`.
- **Contract-relevant modifiers**: `suspend`, `operator`, `infix`, `tailrec`, etc. must match if they affect call sites.
- **Default parameter values**: `expect` may declare defaults; `actual` must not repeat them.
- **Class members and constructors**: members that correspond to `expect` members must be marked `actual`; constructors must match and be marked `actual` if declared in `expect`.

Example:

```kotlin
// commonMain
package demo

expect class Platform() {
    val name: String
}

expect fun greet(name: String = "World"): String
```

```kotlin
// jvmMain
package demo

actual class Platform actual constructor() {
    actual val name: String = "JVM"
}

actual fun greet(name: String): String = "Hello, $name"
```

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| expect belongs in common code and has no implementation. | satisfied | Places expect in commonMain; its expect function and class examples have no implementation, although the no-body rule is not stated explicitly. |
| actual implementations belong in platform or shared intermediate source sets. | satisfied | Includes platform and intermediate shared source sets. |
| Corresponding declarations need matching signatures and the same package, covering each target. | satisfied | Explicitly lists matching package/name, parameter/return types and target coverage. |

Unsupported/material claims: The broad claim that contract-relevant modifiers including tailrec must match is not established by the reviewed source. Several finer signature/default/modifier rules go beyond the saved matching-signature guidance; this review does not treat them as independently verified compiler rules.

Citation review: Not applicable: plain answer has no supplied evidence.

## q12 — plain

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes. In Kotlin Multiplatform you can satisfy an `expect class` with an existing platform type by using an **`actual typealias`** instead of writing a wrapper class.

For example:

**Common code:**
```kotlin
expect class AtomicInt(value: Int) {
    fun get(): Int
    fun set(value: Int)
}
```

**JVM actual:**
```kotlin
actual typealias AtomicInt = java.util.concurrent.atomic.AtomicInteger
```

This works as long as the platform class provides constructors and member functions/properties that match the `expect class` declarations. The compiler will report an error if something is missing or incompatible.

### Important limitations
- You cannot add missing behavior via extension functions/properties — the existing platform class must already have the required API.
- If the platform type’s API doesn’t match (e.g., different method names, missing constructors, final class that can’t be extended), you’ll need to write an `actual class` that wraps it.

This feature has been available since early Kotlin Multiplatform support and remains stable in current Kotlin versions.

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use actual typealias in the platform source set to satisfy an expected class with an existing type. | satisfied | Explicitly recommends actual typealias in the JVM implementation and demonstrates AtomicInteger. |
| The existing type must meet the expected declaration requirements; this is an alias, not a wrapper. | satisfied | Requires matching constructors and members and says missing/incompatible API fails. |
| The actual typealias belongs in the same package as expect; the aliased platform class may live elsewhere. | missing | Does not mention that the alias must share the expect declaration package. |

Unsupported/material claims: Claims the feature remains stable, whereas the complete saved guide labels expected/actual classes Beta (lines 196–198). The final-class limitation is too broad unless inheritance is actually required; aliasing itself does not require subclassing.

Citation review: Not applicable: plain answer has no supplied evidence.

## q12 — rag

Can I satisfy a common expected class using an existing platform type instead of wrapping it?

Yes — you can satisfy a common `expect class` with an existing platform type using an **`actual typealias`** instead of wrapping or inheriting [3].

Example:

```kotlin
// commonMain
expect class Month

// jvmMain
actual typealias Month = java.time.Month
```

Now `Month` in common code *is* `java.time.Month` on the JVM — no wrapper class is generated [3].

Key requirements and caveats:

- The `typealias` must be declared in the **same package** as the expected declaration, while the referred platform class can live elsewhere [3].
- If the expected class has an API (constructors, properties, functions) beyond a plain name, `typealias` cannot provide those members; in that case you'd need an `actual class` that inherits from the platform class [1][2], which is a form of wrapping.
- Visibility can be expanded in the actual declaration, but this requires suppression:
  ```kotlin
  @Suppress("ACTUAL_WITHOUT_EXPECT")
  public actual typealias Messenger = MyMessenger
  ```
  [4]

So: for simple name-mapping to an existing platform type, `actual typealias` is the intended mechanism.

Retrieved references:

- [1] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 235–251; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [2] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 253–267; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes > Inheritance from platform classes
- [3] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 330–337; Expected and actual declarations > Advanced use cases > Using type aliases to satisfy actual declarations
- [4] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 351–356; Expected and actual declarations > Advanced use cases > Expanded visibility in actual declarations
- [5] [Expected and actual declarations](https://kotlinlang.org/docs/multiplatform/multiplatform-expect-actual.html), normalized lines 220–233; Expected and actual declarations > Different approaches for using expected and actual declarations > Expected and actual classes

Review source: [reports/task1/sources/multiplatform-expect-actual.txt](sources/multiplatform-expect-actual.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use actual typealias in the platform source set to satisfy an expected class with an existing type. | satisfied | Recommends actual typealias and shows commonMain/jvmMain placement. |
| The existing type must meet the expected declaration requirements; this is an alias, not a wrapper. | incorrect | Says any API beyond a plain name requires an actual class. The complete source explicitly aliases MyDate with getYear/getMonth/getDayOfMonth members to LocalDate (lines 322–333). Existing compatible members do satisfy expect. |
| The actual typealias belongs in the same package as expect; the aliased platform class may live elsewhere. | satisfied | Explicitly states same-package alias with platform type elsewhere. |

Unsupported/material claims: The claimed restriction to simple name mapping is false against the reference. It also says visibility expansion requires suppression without the Kotlin 2.0 caveat at source line 343; retrieved [4] omits that caveat. Example changes the source expect enum class Month into expect class Month without explaining compatibility.

Citation review: All numeric labels resolve. [3] supports alias and package placement. [1]/[2] show inheritance as an option but do NOT support the claimed prohibition on aliases with members. [4] contains a suppression example but cannot establish that suppression is always required in the current version. Citation presence therefore overstates support.

## q13 — rag

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin exceptions are exposed to Swift/Objective-C callers through the `@Throws` annotation [1].

- **Non-`suspend` functions** with `@Throws` are exported as **`NSError*`-producing methods in Objective-C** and as **`throws` methods in Swift** [1].
- **`suspend` functions** always receive an **`NSError*`/`Error` parameter in the completion handler**, regardless of `@Throws` [1].
- When a Kotlin function throws an exception that is an instance of a class listed in `@Throws` (or a subclass), it is propagated as an `NSError` to the Swift/ObjC caller [1].
- Other Kotlin exceptions reaching Swift/ObjC are considered unhandled and **cause program termination** [1].
- `suspend` functions **without** `@Throws` propagate only `CancellationException` as `NSError`; non-`suspend` functions without `@Throws` do **not** propagate Kotlin exceptions at all [1].
- The reverse translation (Swift/ObjC error-throwing methods imported into Kotlin as exception-throwing) is **not yet implemented** [1].

Example Kotlin declaration:

```kotlin
@Throws(IOException::class)
fun writeToFile() {
    throw IOException()
}
```

From Swift, this becomes a `throws` function; from Objective-C, it produces an `NSError*` out-parameter [1].

Retrieved references:

- [1] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 218–228; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [2] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 230–230; Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
- [3] [Calling Kotlin from Java](https://kotlinlang.org/docs/java-to-kotlin-interop.html), normalized lines 409–437; Calling Kotlin from Java > Checked exceptions
- [4] [Calling Java from Kotlin](https://kotlinlang.org/docs/java-interop.html), normalized lines 585–593; Calling Java from Kotlin > Checked exceptions
- [5] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 33–39; Interoperability with Swift/Objective-C > Using Kotlin in Swift/Objective-C > Hide Kotlin declarations from Objective-C and Swift

Review source: [reports/task1/sources/native-objc-interop.txt](sources/native-objc-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| @Throws on non-suspend functions yields Swift throws and Objective-C NSError-producing methods. | satisfied | Separates non-suspend @Throws mapping into NSError and Swift throws. |
| Only listed exception classes and subclasses propagate as NSError; other escaping Kotlin exceptions terminate the program. | satisfied | Explicitly states listed classes/subclasses propagate and other exceptions terminate. |
| Suspend completion handlers always have an error parameter; without @Throws only CancellationException propagates, while non-suspend exceptions do not propagate. | satisfied | Explicitly covers completion-handler error parameter, CancellationException-only propagation without @Throws, and no non-suspend propagation. |

Unsupported/material claims: Core explanation is supported. The IOException example needs a Native-compatible exception definition/import; the supplied IOException snippet came from the JVM page, so the printed example is not independently validated as Native code.

Citation review: Every citation is [1], which identifies the supplied Errors and exceptions section and supports the associated propagation rules. The general mapping applied to the example is supported, but [1] does not establish availability of an IOException type on Native.

## q13 — plain

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Kotlin exceptions are exposed to Swift and Objective-C callers as `NSError` objects, but only for functions that are explicitly marked as able to throw.

- **Functions annotated with `@Throws`** (or **suspend functions**) are imported into Objective-C with an extra `NSError**` parameter, and into Swift as `throws` functions using `try`.
- **Functions without `@Throws`** are treated as non‑throwing; if such a function actually throws a Kotlin exception, it results in a fatal error (program termination) rather than being propagated as an `NSError`.
- The original Kotlin exception details (class name, message, etc.) are available inside the `NSError`'s `userInfo` dictionary.

### Example

Kotlin:
```kotlin
@Throws(IOException::class)
fun readFile(path: String): String { ... }
```

Swift:
```swift
do {
    let content = try readFile(path: "test.txt")
} catch {
    print(error.localizedDescription) // error is NSError
}
```

Objective‑C:
```objc
NSError *error = nil;
NSString *content = [FileReader readFile:@"test.txt" error:&error];
if (error) {
    NSLog(@"%@", error.localizedDescription);
}
```

For a **suspend function**, the completion handler includes an optional `NSError`:

```swift
fetchData { result, error in
    if let error = error {
        // handle error
    }
}
```

> **Note:** The mapping applies to `@Throws`‑annotated functions and `suspend` functions. Unannotated functions that throw cannot be caught from Swift/Objective‑C and lead to a crash.

Review source: [reports/task1/sources/native-objc-interop.txt](sources/native-objc-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| @Throws on non-suspend functions yields Swift throws and Objective-C NSError-producing methods. | satisfied | Correctly describes the annotated non-suspend NSError/Swift throws mapping and shows examples. |
| Only listed exception classes and subclasses propagate as NSError; other escaping Kotlin exceptions terminate the program. | missing | Omits the restriction to listed exception classes and subclasses; does not say an unlisted exception from an annotated function terminates. |
| Suspend completion handlers always have an error parameter; without @Throws only CancellationException propagates, while non-suspend exceptions do not propagate. | incorrect | Later mentions suspend completion handlers, but blanket statements that unannotated functions cannot propagate omit and contradict the CancellationException exception. Opening bullet also conflates suspend callbacks with ordinary throws mapping. |

Unsupported/material claims: NSError userInfo class/message details are not established by the reviewed section. The IOException example similarly requires an appropriate Native exception type.

Citation review: Not applicable: plain answer has no supplied evidence.

## q14 — plain

How does Swift export expose Kotlin suspend functions?

Swift export exposes Kotlin suspend functions as **completion-handler-based** functions by default, because Swift (before async/await) has no direct representation of coroutines.

### Default mapping

A suspend function:

```kotlin
suspend fun fetchData(id: Int): String
```

is exported to Swift as:

```swift
func fetchData(id: Int32, completionHandler: @escaping (String?, Error?) -> Void)
```

- The Kotlin return type becomes the **first optional parameter** of the completion handler.
- Exceptions thrown inside the suspend function become the **second parameter** of type `Error?`.
- If the suspend function returns `Unit`, the completion handler is just `(Error?) -> Void`.

### Usage in Swift (default style)

```swift
fetchData(id: 1) { result, error in
    if let error = error {
        // handle error
    } else if let result = result {
        // use result
    }
}
```

### Optional: Swift async/await

Starting with Kotlin 1.7.20 (experimental), you can opt in to export suspend functions as native `async`/`throws` Swift functions by annotating them with `@SwiftAsync` (from `kotlinx.cinterop`) or using the compiler flag `-Xswift-async`:

```kotlin
import kotlinx.cinterop.*

@SwiftAsync
suspend fun fetchData(id: Int): String
```

This becomes:

```swift
func fetchData(id: Int32) async throws -> String
```

### Limitations

- Cancellation is cooperative and must be handled manually via Kotlin coroutines.
- The completion-handler style passes `NSError`-wrapped exceptions; the async/await style uses Swift's `Error` protocol directly.

Review source: [reports/task1/sources/native-swift-export.txt](sources/native-swift-export.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Swift export maps Kotlin suspend functions and suspending functional types to Swift async counterparts. | incorrect | Calls completion handlers the default Swift export mapping; the designated direct Swift export guide maps suspend functions/types to native async. |
| Swift calls the exported suspend function with try await, as shown in the saved guide. | missing | Does not show try await usage; instead supplies a callback example and an alleged optional annotation route. |
| Swift export is Alpha; distinguish its direct Swift mapping from the Objective-C completion-handler bridge. | incorrect | Conflates direct Swift export with the Objective-C bridge and never states Alpha status. |

Unsupported/material claims: The alleged @SwiftAsync annotation, -Xswift-async flag and Kotlin 1.7.20 introduction are unsupported by the saved sources. The Objective-C guide instead discusses Swift 5.5 async calling support (lines 259–267); it does not validate these claimed Kotlin controls.

Citation review: Not applicable: plain answer has no supplied evidence.

## q14 — rag

How does Swift export expose Kotlin suspend functions?

Kotlin suspend functions (and suspend functional types) are exposed to Swift as native async functions, so Swift callers use `try await`. For example, a Kotlin `suspend fun hello(): String` is called from Swift as `let msg = try await hello()` [1][2]. By default, exported code runs on `Dispatchers.Default`; you can switch dispatchers inside Kotlin with `withContext(...)` [5]. This export is via the Swift export feature (Alpha in Kotlin 2.4.0 with improved concurrency support) [2].

Retrieved references:

- [1] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), normalized lines 391–402; Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
- [2] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 592–603; What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
- [3] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 404–425; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [4] [Interoperability with Swift/Objective-C](https://kotlinlang.org/docs/native-objc-interop.html), normalized lines 259–267; Interoperability with Swift/Objective-C > Mappings > Suspending functions
- [5] [Swift export](https://kotlinlang.org/docs/native-swift-export.html), normalized lines 420–429; Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers

Review source: [reports/task1/sources/native-swift-export.txt](sources/native-swift-export.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Swift export maps Kotlin suspend functions and suspending functional types to Swift async counterparts. | satisfied | Explicitly maps both suspend functions and functional types to native async. |
| Swift calls the exported suspend function with try await, as shown in the saved guide. | satisfied | Shows let msg = try await hello(), matching the guide. |
| Swift export is Alpha; distinguish its direct Swift mapping from the Objective-C completion-handler bridge. | satisfied | Names the Swift export feature and its Alpha status, with the direct mapping rather than callbacks. |

Unsupported/material claims: The full Kotlin 2.4.0 release section at lines 588–590 confirms Alpha status and improved concurrency. Dispatcher details match the full guide. No material factual contradiction found.

Citation review: Used [1], [2], [5] all identify supplied chunks. Async/try-await and dispatcher claims are supported. However [2] starts at line 592, after the Alpha statement at 588–590, so the Alpha/version-status attribution is not supported by that supplied chunk, even though the complete source confirms it.

## q16 — rag

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use `external` declarations. They describe the JavaScript API’s shape without providing a Kotlin implementation; the Kotlin compiler assumes the implementation already exists in the JavaScript environment [1].

Example:

```kotlin
// Describe a global JS function
external fun alert(message: String)

// Describe a JS class with a method
external class JsApi {
    fun doSomething(value: Int): String
}

fun main() {
    alert("Hello")
    val api = JsApi()
    println(api.doSomething(42))
}
```

For callable JavaScript objects (functions that are invoked like `obj()`), annotate an `operator fun invoke` with `@nativeInvoke` [4]. In Kotlin/Wasm, external interop is experimental, so you may need `@OptIn(ExperimentalWasmJsInterop::class)` on declarations or using code [4].

Retrieved references:

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 13–15; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
- [2] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 79–98; Use Kotlin code from JavaScript > Package structure > @JsExport annotation
- [3] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 170–194; Use Kotlin code from JavaScript > Package structure > @JsNoRuntime annotation
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [5] [What's new in Kotlin 2.3.20](https://kotlinlang.org/docs/whatsnew2320.html), normalized lines 705–719; What's new in Kotlin 2.3.20 > Kotlin/JS > Implementing Kotlin interfaces from JavaScript/TypeScript > How to enable

Review source: [reports/task1/sources/js-interop.txt](sources/js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use external on declarations whose implementation is supplied by JavaScript. | satisfied | Recommends external declarations and gives function/class examples. |
| Do not provide a Kotlin body; the compiler emits no implementation and JavaScript must supply it. | satisfied | Says no Kotlin implementation is provided and JavaScript must already supply it. |
| external is package-level and inherited by nested members; optional JS arguments can use definedExternally. | missing | Does not explain the package-level restriction, inherited external modifier, or definedExternally defaults. |

Unsupported/material claims: Overgeneralizes the specific ExperimentalWasmJsInterop opt-in in a nativeInvoke example into a statement that Wasm external interop is experimental. The full Wasm page labels Kotlin/Wasm Beta and describes this particular callable-object facility as temporary. It adds platform-specific material although the designated source is Kotlin/JS.

Citation review: Both [1] and [4] identify supplied Wasm chunks. [1] supports describing existing JS APIs, but is only a three-line introductory fragment, not the complete no-body rules. [4] supports the callable-object example and its opt-in; it does not establish that all external interop requires that opt-in.

## q16 — plain

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Use **external declarations** in Kotlin/JS. Mark classes, functions, properties, or objects with `external` to tell the compiler the implementation exists in JavaScript and should not be generated.

```kotlin
external fun alert(message: String)

external class URL(url: String, base: String? = definedExternally) {
    val href: String
    fun toString(): String
}

external object Math {
    fun random(): Double
    val PI: Double
}

external interface JQuery {
    fun html(html: String): JQuery
    fun append(content: String): JQuery
}

@JsModule("jquery")
external fun jQuery(selector: String): JQuery
```

- `external` members have no bodies; Kotlin won't implement them.
- `@JsModule("module")` imports from a JavaScript module; `@JsNonModule` allows fallback to a global.
- Use `@JsName("jsName")` to map Kotlin names to different JavaScript names.
- For untyped values, use `dynamic`.

```kotlin
external val process: dynamic
```

Requires Kotlin/JS plugin (Kotlin 1.x with `kotlin.js` or Kotlin 2.x with `kotlin("js")`).

Review source: [reports/task1/sources/js-interop.txt](sources/js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use external on declarations whose implementation is supplied by JavaScript. | satisfied | Correctly recommends external declarations for JS-provided implementations. |
| Do not provide a Kotlin body; the compiler emits no implementation and JavaScript must supply it. | satisfied | Explicitly says members have no bodies and Kotlin emits no implementation. |
| external is package-level and inherited by nested members; optional JS arguments can use definedExternally. | missing | Shows definedExternally and implicit external members in examples, but omits the package-level restriction and does not explain inherited external status. |

Unsupported/material claims: The Kotlin 1.x versus 2.x plugin distinction is not supported by the source and is unnecessary to answer the question. JsModule/JsNonModule specifics extend beyond the designated section; examples were not compiled, including the declared toString member.

Citation review: Not applicable: plain answer has no supplied evidence.

## q17 — plain

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

Use `@JsExport` on a Kotlin class or factory function. When called from JavaScript, the result is an **opaque reference** (a Wasm GC object) that JavaScript can store in a variable and pass back to other exported Kotlin functions. This JavaScript variable keeps the Kotlin object alive.

### Kotlin side
```kotlin
@JsExport
class Counter {
    private var value = 0
    fun increment() { value++ }
    fun getValue(): Int = value
}

@JsExport
fun createCounter(): Counter = Counter()
```

### JavaScript side
```javascript
import { Counter, createCounter } from "./module.mjs";

// Hold a reference by assigning to a JS variable
const counter = new Counter();   // or createCounter()
counter.increment();
console.log(counter.getValue()); // 1

// The variable `counter` is a reference to the Kotlin object.
// It can be passed back to any exported function that expects a Counter.
```

### Notes
- The JavaScript variable holds a strong reference to the underlying Wasm object, so the Kotlin object will not be garbage collected while the variable exists.
- Treat the object as an opaque handle; you cannot inspect its internal state directly from JavaScript except through exported members.
- This works with Kotlin/Wasm’s Wasm GC backend (current Kotlin 2.x). The feature is experimental, so API details may change.

Review source: [reports/task1/sources/wasm-js-interop.txt](sources/wasm-js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use JsReference<T> to pass an opaque handle to a Kotlin object into JavaScript. | incorrect | Replaces JsReference with directly exported Kotlin classes/objects. The full Wasm guide allows only package-level function exports and requires JsReference for Kotlin objects (lines 402–406). |
| Create the handle with toJsReference() and expose it through an exported function. | incorrect | Factory returns Counter directly, not Counter().toJsReference(). |
| JavaScript cannot directly access the Kotlin object; exported Kotlin helpers unwrap the handle using get(). | incorrect | Calls counter.increment()/getValue() from JS rather than passing an opaque handle to Kotlin helpers and unwrapping with get(). |

Unsupported/material claims: Confuses Kotlin/JS class exports with Kotlin/Wasm. The claimed strong-reference/GC lifetime guarantee is not established by the reviewed source. Named exports also conflict with the saved Wasm default-export-object description.

Citation review: Not applicable: plain answer has no supplied evidence.

## q17 — rag

How can Kotlin Wasm hold a reference to a Kotlin object on the JavaScript side?

You can hold a reference to a Kotlin object on the JavaScript side using the `JsReference` type in Kotlin/Wasm [1].

- In Kotlin, use `toJsReference()` to wrap an object and return it as a `JsReference<T>` from an `@JsExport` function.
- On the JavaScript side, the reference is opaque and behaves like an empty frozen JavaScript object [1].
- To use the object later, export other `@JsExport` functions that accept the `JsReference<T>` and call `get()` to unwrap it [1].

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

JavaScript usage:

```javascript
import UserLib from "./userlib.mjs"

let user = UserLib.createUser("Bob");
UserLib.setUserName(user, "Alice");
```

[1][5]

Note: The referenced documentation does not state the Kotlin/Wasm version, so API availability may vary by release.

Retrieved references:

- [1] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 318–338; Interoperability with JavaScript > Type correspondence > JsReference type
- [2] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 99–117; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > External objects
- [3] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 123–142; Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
- [4] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 164–176; Interoperability with JavaScript > Use JavaScript code in Kotlin > Kotlin functions with JavaScript code
- [5] [Kotlin/Wasm JavaScript interoperability](https://kotlinlang.org/docs/wasm-js-interop.html), normalized lines 340–345; Interoperability with JavaScript > Type correspondence > JsReference type

Review source: [reports/task1/sources/wasm-js-interop.txt](sources/wasm-js-interop.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Use JsReference<T> to pass an opaque handle to a Kotlin object into JavaScript. | satisfied | Uses JsReference<T> and explains an opaque empty frozen JS object. |
| Create the handle with toJsReference() and expose it through an exported function. | satisfied | Factory returns User(name).toJsReference(). |
| JavaScript cannot directly access the Kotlin object; exported Kotlin helpers unwrap the handle using get(). | satisfied | Exported setUserName accepts the handle and calls user.get(); JS calls the helper. |

Unsupported/material claims: No material unsupported claim found. Explicitly avoids a historical availability guarantee, consistent with the snapshot caveat.

Citation review: All cited labels [1], [5] resolve. [1] supplies handle creation, opacity and get(); [5] supplies the JS helper usage. They support the associated explanation and examples.

## q18 — rag

When did the common Kotlin UUID API become stable?

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became stable in **Kotlin 2.4.0** [1]. The only exceptions are the functions for generating V4 and V7 UUIDs (`Uuid.generateV4()` and `Uuid.generateV7()`), which remain experimental and still require opt-in [1].

Retrieved references:

- [1] [What's new in Kotlin 2.4.0](https://kotlinlang.org/docs/whatsnew24.html), normalized lines 370–382; What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
- [2] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 586–616; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing
- [3] [What's new in Kotlin 2.1.20](https://kotlinlang.org/docs/whatsnew2120.html), normalized lines 323–365; What's new in Kotlin 2.1.20 > Standard library > Changes in UUID parsing, formatting, and comparability
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 658–684; What's new in Kotlin 2.3.0 > Standard library > Improved UUID generation and parsing > New functions to generate v4 and v7 UUIDs

Review source: [reports/task1/sources/whatsnew24.txt](sources/whatsnew24.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| The common kotlin.uuid.Uuid API became Stable in Kotlin 2.4.0. | satisfied | Gives Kotlin 2.4.0 for stabilization. |
| V4 and V7 generation functions remain Experimental and require opt-in. | satisfied | Explicitly retains experimental status and opt-in for V4/V7 generation. |
| The API was introduced experimentally in Kotlin 2.0.20; introduction and stabilization differ. | missing | Does not mention the 2.0.20 experimental introduction. This is background completeness, not a wrong answer to the stabilization-date question. |

Unsupported/material claims: No material unsupported claim found. Exact generateV4/generateV7 names are present in supplied chunk [4].

Citation review: The used label [1] supports stabilization and experimental exceptions. It does not spell out the function names, but supplied unused [4] does; minor citation granularity gap rather than a fabricated API.

## q18 — plain

When did the common Kotlin UUID API become stable?

The common Kotlin UUID API (`kotlin.uuid.Uuid`) became stable in **Kotlin 2.0.20**, released in August 2024. It had been introduced as experimental in Kotlin 1.9.0 behind the `@ExperimentalUuidApi` annotation.

Review source: [reports/task1/sources/whatsnew24.txt](sources/whatsnew24.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| The common kotlin.uuid.Uuid API became Stable in Kotlin 2.4.0. | incorrect | Says stable in 2.0.20; the snapshot says stable in 2.4.0. |
| V4 and V7 generation functions remain Experimental and require opt-in. | missing | Omits V4/V7 experimental exceptions. |
| The API was introduced experimentally in Kotlin 2.0.20; introduction and stabilization differ. | incorrect | Claims experimental introduction in 1.9.0; source says 2.0.20. |

Unsupported/material claims: Conflates introduction and stabilization and invents an earlier introduction version relative to the source. The calendar release date is not verified by the reviewed section.

Citation review: Not applicable: plain answer has no supplied evidence.

## q19 — plain

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

In Kotlin 2.4.20, the Kotlin/JS compiler added support for **exporting functions that accept suspending lambdas** (i.e., parameters of type `suspend () -> T` or similar) with `@JsExport`.

Previously, exporting such functions failed with a compile-time error because JavaScript has no native concept of coroutines, and there was no way to represent a `suspend` lambda in the generated JS/TS signature.

Now, when you export a function with a suspending lambda parameter, the generated JavaScript/TypeScript signature accepts a **regular JavaScript function that returns a `Promise`**. At runtime, the Kotlin-generated adapter wraps the Promise-returning callback, suspends the coroutine until the Promise resolves, and then resumes execution.

Example:

```kotlin
@JsExport
fun runLater(block: suspend () -> Unit) {
    // Kotlin implementation can call block() as a normal suspend function
}
```

Generated JS usage:

```javascript
runLater(() => Promise.resolve());
```

TypeScript declaration:

```typescript
export function runLater(block: () => Promise<void>): void;
```

This removes the earlier limitation and makes it easier to call coroutine‑based Kotlin code from JavaScript. If the Kotlin version you are using is exactly 2.4.20, this behaviour should be available by default; earlier versions may have required experimental flags or did not support it at all. Always check the official release notes for any additional caveats or changes.

Review source: [reports/task1/sources/whatsnew2420.txt](sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 supports exporting suspending lambdas as JavaScript async functions. | satisfied | Correctly identifies 2.4.20 support for suspending callback parameters with Promise-returning JavaScript callbacks. |
| Enable -Xsuspend-lambda-exporting. | incorrect | Says available by default in 2.4.20, contradicting the documented -Xsuspend-lambda-exporting requirement. |
| Annotate relevant declarations @JsExport; compiler bridges suspend lambdas with JS async/await. | satisfied | Uses @JsExport and describes automatic suspend/Promise bridging. |

Unsupported/material claims: The default-enabled assertion is material and incorrect against the snapshot. Its non-suspend runLater stub does not demonstrate the coroutine context needed to invoke block; the exact generated TypeScript signature and adapter internals are not verified in the source.

Citation review: Not applicable: plain answer has no supplied evidence.

## q19 — rag

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

In Kotlin 2.4.20, Kotlin/JS gained support for exporting **suspending lambda expressions** (parameters of type `suspend () -> R`, etc.) as JavaScript **async functions** [1][5].

Previously, you could export `suspend` functions themselves (since Kotlin 2.3.0) [4], but there was no way to export declarations containing suspending lambdas from Kotlin/JS libraries [1]. With 2.4.20, the compiler automatically bridges Kotlin’s `suspend` model and JavaScript’s native `async`/`await` for such lambdas [1][3].

To enable it, add the compiler option `-Xsuspend-lambda-exporting`:

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

Then mark the relevant declarations with `@JsExport`:

```kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}
```

From TypeScript, the suspending lambda appears as a regular `async` function:

```typescript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
```

This improves mixed Kotlin/TypeScript codebases by removing manual wrapping [1][3].

Retrieved references:

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 505–523; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 424–426; What's new in Kotlin 2.4.20 > Kotlin/JS
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 525–544; What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
- [4] [What's new in Kotlin 2.3.0](https://kotlinlang.org/docs/whatsnew23.html), normalized lines 396–402; What's new in Kotlin 2.3.0 > Kotlin/JS > New export of suspend function with JsExport
- [5] [Use Kotlin code from JavaScript](https://kotlinlang.org/docs/js-to-kotlin-interop.html), normalized lines 133–159; Use Kotlin code from JavaScript > Package structure > @JsExport annotation > Export suspending lambdas

Review source: [reports/task1/sources/whatsnew2420.txt](sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 supports exporting suspending lambdas as JavaScript async functions. | satisfied | Identifies the 2.4.20 suspending-lambda async export feature. |
| Enable -Xsuspend-lambda-exporting. | satisfied | Gives -Xsuspend-lambda-exporting in compilerOptions. |
| Annotate relevant declarations @JsExport; compiler bridges suspend lambdas with JS async/await. | satisfied | Uses the documented @JsExport TaskRunner and async/await example. |

Unsupported/material claims: Core guidance is supported. The historical 2.3.0 suspend-function comparison omits that feature’s own enabling flag, which appears in the complete release section immediately after the retrieved [4] chunk. This does not change the correctly stated 2.4.20 lambda flag.

Citation review: Labels [1], [3], [4], [5] all resolve and support their main associated claims: introduction, enabling flag, bridge example and earlier suspend-function support. [4] is truncated before its own enablement caveat.

## q20 — rag

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Cross-language inheritance in Swift export [1]. This feature allows Swift implementations to inherit from exported Kotlin open classes, enabling the reverse import pattern where Swift subclasses a Kotlin superclass and implements Kotlin interfaces [1].

Retrieved references:

- [1] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 262–279; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
- [2] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 214–216; What's new in Kotlin 2.4.20 > Kotlin/Native
- [3] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 226–247; What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Sealed classes
- [4] [What's new in Kotlin 2.2.20](https://kotlinlang.org/docs/whatsnew2220.html), normalized lines 432–452; What's new in Kotlin 2.2.20 > Kotlin Multiplatform > Swift export available by default
- [5] [What's new in Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html), normalized lines 39–59; What's new in Kotlin 2.4.20 > New features

Review source: [reports/task1/sources/whatsnew2420.txt](sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 introduces cross-language inheritance in Swift export. | satisfied | Names cross-language inheritance in response to the explicitly versioned question. |
| The reverse-import pattern declares a Kotlin interface and open Kotlin base class; Swift subclasses the base and implements the interface. | satisfied | Explains Swift subclasses an exported Kotlin open class and implements Kotlin interfaces. |
| Pass the Swift instance to Kotlin functions accepting the interface so Kotlin dispatches to Swift code. | missing | Does not describe passing the Swift instance back to a Kotlin function accepting the interface. The direct feature-name question is nevertheless answered. |

Unsupported/material claims: No material unsupported claim found; answer is brief and stays within the retrieved section.

Citation review: The sole used label [1] identifies the exact supplied 2.4.20 cross-language inheritance section and supports both sentences.

## q20 — plain

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

The Kotlin 2.4.20 feature is **Kotlin/Native’s support for subclassing exported Kotlin classes from Swift** (often referred to as “Swift subclassing” or “exported Kotlin class inheritance”). It allows Swift code to inherit from Kotlin `open` classes that are exported to Swift.

If you need to enable it explicitly, use the compiler flag:

```
-Xbinary=objcExportSubclassing=true
```

In some 2.4.x versions this may be stable or enabled by default, depending on the exact release.

Example:

```kotlin
// Kotlin
open class Animal(val name: String)
```

```swift
// Swift
class Dog: Animal {
    let breed: String
    init(name: String, breed: String) {
        self.breed = breed
        super.init(name: name)
    }
}
```

*Note: My knowledge cutoff predates 2.4.20, so the exact flag/default status may vary.*

Review source: [reports/task1/sources/whatsnew2420.txt](sources/whatsnew2420.txt)

| Expected fact | Assessment | Evidence |
| --- | --- | --- |
| Kotlin 2.4.20 introduces cross-language inheritance in Swift export. | satisfied | Identifies Swift subclassing exported Kotlin open classes in 2.4.20, although it does not use the exact cross-language inheritance name. |
| The reverse-import pattern declares a Kotlin interface and open Kotlin base class; Swift subclasses the base and implements the interface. | missing | Shows a Kotlin base and Swift subclass but omits the Kotlin interface part of the documented reverse-import pattern. |
| Pass the Swift instance to Kotlin functions accepting the interface so Kotlin dispatches to Swift code. | missing | Does not pass the Swift implementation back to Kotlin through the interface. |

Unsupported/material claims: The -Xbinary=objcExportSubclassing=true flag is not supported by the saved Swift export section. Speculation that this is stable or enabled by default in some 2.4.x versions is not established by the source; a cutoff disclaimer does not substantiate it.

Citation review: Not applicable: plain answer has no supplied evidence.

