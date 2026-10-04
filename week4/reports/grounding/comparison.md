# Quotes, sources and unknown responses

Ten questions: seven answerable from the saved corpus and three out-of-corpus controls. Model `deepseek-v4-flash`, reasoning `none`; relevance threshold 2/3. Rewrite=False, rerank=False. Scores are ordinal judgments, not probabilities.

Reviews are authored separately against the saved chunks and bound to complete answer hashes. Exact substring validation establishes quote provenance, not semantic agreement. This is one illustrative run; it does not establish general reliability.

Index SHA-256: `8249a981e64ba378fac6690ca46299545fa68f054cfb5ccbe5f311119104040d`. Suite SHA-256: `702bf707eedf31aeb03e7bd08999a21736bfd41aea8e835f94e86c4f65102e5a`.

7 factual answers; sources present in 7/7, quotes present in 7/7, exact fragments in 7/7.

Meaning fully supported by quotes: 4/7. Appropriate unknown controls: 3/3. Answerable questions refused: 0.

| Question | Result | Sources | Quotes | Exact fragments | Meaning agrees |
| --- | --- | --- | --- | --- | --- |
| q01 | answered | yes | yes | yes | yes |
| q13 | answered | yes | yes | yes | yes |
| q14 | answered | yes | yes | yes | partial |
| q16 | answered | yes | yes | yes | partial |
| q18 | answered | yes | yes | yes | yes |
| q19 | answered | yes | yes | yes | partial |
| q20 | answered | yes | yes | yes | yes |
| q21 | unknown | N/A | N/A | N/A | N/A |
| q22 | unknown | N/A | N/A | N/A | N/A |
| q23 | unknown | N/A | N/A | N/A | N/A |

Recorded API attempts: 48; tokens: 100,031; estimated cost: $0.006089. Skipped answer calls: 3. Failed attempts retained: 1.

Unknown responses legitimately have no sources or quotes. They must contain no factual answer and must request clarification. API failures and malformed answers are errors, not evidence of corpus insufficiency.

## q01

How can a function require an implicit service and how is that service resolved?

Answer
A function declares an implicit dependency with the `context` keyword followed by a list of parameters written as `name: Type` [3]. For example, `context(users: UserService)` declares a context parameter on `outputMessage`, so `users` is available inside the function without being passed manually [3]. Kotlin resolves context parameters at the call site by searching the current scope for matching context values, and it matches them by type [2]. If multiple compatible values exist at the same scope level, the compiler reports an ambiguity error [2]. A context parameter can also be anonymous by using `_` as its name; it still satisfies the required context parameters of called functions but cannot be accessed by name, so you use `contextOf<T>()` to access its value explicitly [1].

Sources
[1] Context parameters — Context parameters
https://kotlinlang.org/docs/context-parameters.html (chunk_id: 2dbf892f46e37b486442c3499d22535ad4217ce70468e4594e9f09707c102a31)
[2] Context parameters — Context parameters > Context parameters resolution
https://kotlinlang.org/docs/context-parameters.html (chunk_id: b9b621ea152e9b98e5e5f36e71ea2895836b411d1695f1c448b6b91f0b722e3d)
[3] Context parameters — Context parameters
https://kotlinlang.org/docs/context-parameters.html (chunk_id: 35ffe9fe769edf13404a8225ebf9446ab9255d0223f4f0737e81a4276ebc10eb)

Quotes
Chunk 35ffe9fe769edf13404a8225ebf9446ab9255d0223f4f0737e81a4276ebc10eb:
> To declare context parameters for properties and functions, use the context keyword followed by a list of parameters, with each parameter declared as name: Type . Here is an example with a dependency on the UserService interface:
Chunk 35ffe9fe769edf13404a8225ebf9446ab9255d0223f4f0737e81a4276ebc10eb:
> context(users: UserService)
> fun outputMessage(message: String) {
Chunk b9b621ea152e9b98e5e5f36e71ea2895836b411d1695f1c448b6b91f0b722e3d:
> Kotlin resolves context parameters at the call site by searching for matching context values in the current scope. Kotlin matches them by their type. If multiple compatible values exist at the same scope level, the compiler reports an ambiguity:
Chunk 2dbf892f46e37b486442c3499d22535ad4217ce70468e4594e9f09707c102a31:
> You can use _ as a context parameter name when you don't need to refer to the parameter directly. An anonymous context parameter can still satisfy the context parameters required by called functions, but you can't access it by name. To access its value explicitly, use contextOf<T>() :

Review: The declaration syntax, UserService example, call-site/type resolution, ambiguity rule and anonymous context parameter behavior all agree with the displayed fragments. The short example quote shows the declared dependency; the prose quote explains context parameters. No unsupported version claim is added.

## q13

How are Kotlin exceptions exposed to Swift and Objective-C callers?

Answer
Kotlin exceptions are unchecked, whereas Swift has only checked errors handled at compile time, so a Kotlin method that throws and is called from Swift or Objective-C should be marked with @Throws and a list of "expected" exception classes [1]. When compiling to a Swift/Objective-C framework, non-suspend functions that have or inherit @Throws are represented as NSError*-producing methods in Objective-C and as throws methods in Swift [1]. Representations for suspend functions always have an NSError*/Error parameter in the completion handler [1].

If a Kotlin function called from Swift/Objective-C throws an exception that is an instance of one of the classes specified with @Throws or their subclasses, the exception is propagated as an NSError [1]. Other Kotlin exceptions reaching Swift/Objective-C are considered unhandled and cause program termination [1].

For functions without @Throws: suspend functions propagate only CancellationException (as NSError), while non-suspend functions don't propagate Kotlin exceptions at all [1]. The reverse translation is not implemented yet: Swift/Objective-C error-throwing methods aren't imported to Kotlin as exception-throwing [1].

The provided evidence does not state version or availability guarantees for this behavior.

Sources
[1] Interoperability with Swift/Objective-C — Interoperability with Swift/Objective-C > Mappings > Errors and exceptions
https://kotlinlang.org/docs/native-objc-interop.html (chunk_id: 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc)

Quotes
Chunk 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc:
> All Kotlin exceptions are unchecked, meaning that errors are caught at runtime. However, Swift has only checked errors that are handled at compile time. So, if Swift or Objective-C code calls a Kotlin method that throws an exception, the Kotlin method should be marked with the @Throws annotation, specifying a list of "expected" exception classes.
Chunk 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc:
> When compiling to the Swift/Objective-C framework, non- suspend functions that have or inherit the @Throws annotation are represented as NSError* -producing methods in Objective-C and as throws methods in Swift. Representations for suspend functions always have an NSError* / Error parameter in the completion handler.
Chunk 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc:
> When a Kotlin function called from Swift/Objective-C code throws an exception which is an instance of one of the classes specified with @Throws or their subclasses, the exception is propagated as an NSError . Other Kotlin exceptions reaching Swift/Objective-C are considered unhandled and cause program termination.
Chunk 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc:
> suspend functions without @Throws propagate only CancellationException (as NSError ). Non- suspend functions without @Throws don't propagate Kotlin exceptions at all.
Chunk 6395dce2f8c3c5d8b8dd9e9585788c35597612b6f05b537e6c446784e5cb1bbc:
> Note that the opposite reversed translation is not implemented yet: Swift/Objective-C error-throwing methods aren't imported to Kotlin as exception-throwing.

Review: The quoted Errors and exceptions section supports the @Throws mapping, NSError/throws representations, suspend completion-handler error parameter, subclass propagation, termination for other exceptions and CancellationException exception. The answer follows the saved documentation, including its simplified description of Swift error handling. The final uncertainty statement does not invent a version guarantee.

## q14

How does Swift export expose Kotlin suspend functions?

Answer
Swift export exposes Kotlin suspending functions and suspending functional types as Swift's async counterparts, so you call them with `try await` [1]. The mapping is `suspend fun` → `async` [5]. For example, a Kotlin `suspend fun hello(): String` is callable in Swift as `let msg = try await hello()` [2]. By default, calling a Kotlin suspending function from Swift creates a coroutine context using `Dispatchers.Default` and executes the exported code there; to use a different dispatcher, switch the coroutine context in Kotlin with `withContext()` [3].

Sources
[1] Swift export — Interoperability with Swift using Swift export > Mappings > Concurrency > Suspending functions
https://kotlinlang.org/docs/native-swift-export.html (chunk_id: d215495afcdd769cd1fb1c4a44a0beaffc529dcfeef4a86dab2b04a83a81fdac)
[2] What's new in Kotlin 2.4.0 — What's new in Kotlin 2.4.0 > Kotlin/Native > Swift export goes Alpha with improved concurrency support > Support for structured concurrency
https://kotlinlang.org/docs/whatsnew24.html (chunk_id: 9cc69b9e67e807b3e56af4c1484461a34bc05b4698b2c1387b80e9b4d16c5fbe)
[3] Swift export — Interoperability with Swift using Swift export > Mappings > Concurrency > Coroutine dispatchers
https://kotlinlang.org/docs/native-swift-export.html (chunk_id: 2941c6f4c0e605d779a13f9b96997a73b67d416ea5a84137f1422e0f78f78803)
[5] Swift export — Interoperability with Swift using Swift export > Mappings
https://kotlinlang.org/docs/native-swift-export.html (chunk_id: 2769e1f5f082b098830ac37352fd25674ca045fa67fc04fb3d0ffb8d7a0d8897)

Quotes
Chunk d215495afcdd769cd1fb1c4a44a0beaffc529dcfeef4a86dab2b04a83a81fdac:
> Kotlin suspending functions and suspending functional types are exported as Swift's async counterparts:
Chunk 9cc69b9e67e807b3e56af4c1484461a34bc05b4698b2c1387b80e9b4d16c5fbe:
> Kotlin suspend functions and suspend functional types are exported as Swift's idiomatic async counterparts:
Chunk 2941c6f4c0e605d779a13f9b96997a73b67d416ea5a84137f1422e0f78f78803:
> By default, when you call a Kotlin suspending function from Swift or use the asAsyncSequence function, Kotlin creates a coroutine context that uses the Dispatchers.Default dispatcher and executes the exported code there.
Chunk 2769e1f5f082b098830ac37352fd25674ca045fa67fc04fb3d0ffb8d7a0d8897:
> suspend fun | async

Review: The async mapping and Dispatchers.Default claims match the displayed quotes. However, the try await hello() example and the withContext() recommendation are not in the selected quotes. Both are present in the cited full chunks (references 1/2 and 3 respectively), so this is incomplete quote coverage rather than fabrication. The answer also omits the suite’s Alpha-status caveat; the displayed quotes do not establish it.

## q16

How do I declare an existing JavaScript API so Kotlin can call it without providing an implementation?

Answer
You declare the JavaScript API with external declarations, which describe its API to Kotlin; external JavaScript code is not visible in Kotlin by default [1]. External functions have no bodies, and you can call them like regular Kotlin functions [3]. For a JavaScript function such as `function greet (name) { console.log("Hello, " + name + "!"); }`, declare it in Kotlin as `external fun greet(name: String)` [3]. By default, external declarations correspond to the JavaScript global scope, but annotating a Kotlin file with `@JsModule` makes all external declarations within it imported from the specified module [4]. For a callable JavaScript object, you can use the `@nativeInvoke` annotation on a Kotlin member function of an external declaration (a class or an interface) to make it callable as a JavaScript function [2]. Note that `@nativeInvoke` is a temporary solution until there's a design for stable interoperability, and currently the compiler reports a warning when it is used [2].

Sources
[1] Kotlin/Wasm JavaScript interoperability — Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations
https://kotlinlang.org/docs/wasm-js-interop.html (chunk_id: 72ada8f5af0ed30cf7820dc3311092715c7f7972be65c8030b00d61fa47c060b)
[2] Kotlin/Wasm JavaScript interoperability — Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > Callable JavaScript objects with @nativeInvoke
https://kotlinlang.org/docs/wasm-js-interop.html (chunk_id: f67b3934e9e2471adc4d4cfffe63b41d79ff03d95ba5ce43ee057b84d75fff65)
[3] Kotlin/Wasm JavaScript interoperability — Interoperability with JavaScript > Use JavaScript code in Kotlin > External declarations > JavaScript functions
https://kotlinlang.org/docs/wasm-js-interop.html (chunk_id: 698ddd9815a5ad680a01cc1085579ec13daef62762c334316265cf4342c89d6a)
[4] Kotlin/Wasm JavaScript interoperability — Interoperability with JavaScript > Use JavaScript code in Kotlin > JavaScript modules
https://kotlinlang.org/docs/wasm-js-interop.html (chunk_id: e4ec9b6e192fd837189d5498463708d3d0c8752c0ee08a5dbfa81944783d1cea)

Quotes
Chunk 72ada8f5af0ed30cf7820dc3311092715c7f7972be65c8030b00d61fa47c060b:
> External JavaScript code is not visible in Kotlin by default. To use JavaScript code in Kotlin, you can describe its API with external declarations.
Chunk 698ddd9815a5ad680a01cc1085579ec13daef62762c334316265cf4342c89d6a:
> function greet (name) {
>     console.log("Hello, " + name + "!");
> }
Chunk 698ddd9815a5ad680a01cc1085579ec13daef62762c334316265cf4342c89d6a:
> external fun greet(name: String)
Chunk 698ddd9815a5ad680a01cc1085579ec13daef62762c334316265cf4342c89d6a:
> External functions don't have bodies, and you can call it as a regular Kotlin function:
Chunk e4ec9b6e192fd837189d5498463708d3d0c8752c0ee08a5dbfa81944783d1cea:
> By default, external declarations correspond to the JavaScript global scope. If you annotate a Kotlin file with the @JsModule annotation , then all external declarations within it are imported from the specified module.
Chunk f67b3934e9e2471adc4d4cfffe63b41d79ff03d95ba5ce43ee057b84d75fff65:
> You can use the @nativeInvoke annotation on a Kotlin member function of an external declaration (a class or an interface) to make it callable as a JavaScript function.
Chunk f67b3934e9e2471adc4d4cfffe63b41d79ff03d95ba5ce43ee057b84d75fff65:
> The @nativeInvoke annotation is a temporary solution until there's a design for stable interoperability. Currently, when you use @nativeInvoke , the compiler reports a warning.

Review: The external modifier, bodyless declaration, greet example, @JsModule behavior and @nativeInvoke warning are all verbatim-supported. However, every cited source is the Kotlin/Wasm interoperability guide, while the answer presents the extra module/callable-object guidance without identifying that platform scope or asking which target the user means. The core external declaration answer is supported; the broader platform framing is only partially supported. The designated Kotlin/JS source was not packed.

## q18

When did the common Kotlin UUID API become stable?

Answer
The common Kotlin standard library's UUID API became Stable in Kotlin 2.4.0 [1]. The exception is the functions for generating V4 and V7 UUIDs, which remain Experimental and require opt-in [1]. The kotlin.uuid.Uuid API is covered by this stabilization [1], and Kotlin 2.4.0 stabilizes support for UUIDs in the common Kotlin standard library [2][4].

Sources
[1] What's new in Kotlin 2.4.0 — What's new in Kotlin 2.4.0 > Standard library > Stable UUID API in the common Kotlin standard library
https://kotlinlang.org/docs/whatsnew24.html (chunk_id: e5f050b7b96b5da722d6035db7bcc47b82a4601f0bed32608a9362fd2e1a9073)
[2] What's new in Kotlin 2.4.0 — What's new in Kotlin 2.4.0 > Standard library
https://kotlinlang.org/docs/whatsnew24.html (chunk_id: 7d6db19d113c46c8c6d2473e25948d3a02cc7fa27d5a9d35ed7b7ee036812831)
[4] What's new in Kotlin 2.4.0 — What's new in Kotlin 2.4.0
https://kotlinlang.org/docs/whatsnew24.html (chunk_id: dc5ca7f6b2dd8476855e9839befc789089525079579f7f0f3e53ab2da4b2eedb)

Quotes
Chunk e5f050b7b96b5da722d6035db7bcc47b82a4601f0bed32608a9362fd2e1a9073:
> In Kotlin 2.4.0, the kotlin.uuid.Uuid API becomes Stable . The only exceptions are the functions for generating V4 and V7 UUIDs , which remain Experimental and still require opt-in.
Chunk 7d6db19d113c46c8c6d2473e25948d3a02cc7fa27d5a9d35ed7b7ee036812831:
> Kotlin 2.4.0 stabilizes support for UUIDs in the common Kotlin standard library.
Chunk dc5ca7f6b2dd8476855e9839befc789089525079579f7f0f3e53ab2da4b2eedb:
> Standard library: Stabilized support for the UUID API and support for checking sorted order

Review: The first quote explicitly establishes Kotlin 2.4.0 stabilization of kotlin.uuid.Uuid and the V4/V7 Experimental opt-in exceptions. The other quotes corroborate common-standard-library stabilization. The answer does not confuse introduction with stabilization. It omits the optional historical introduction fact, which does not undermine support for the answer actually given.

## q19

What changed in Kotlin 2.4.20 for exporting suspending lambdas to JavaScript?

Answer
Kotlin 2.4.20 adds support for exporting suspending lambda expressions as JavaScript async functions [1][2]. Previously, there was no way to export declarations containing suspending lambdas from Kotlin/JS libraries [1]. The compiler now automatically handles the bridging between Kotlin's suspend functions and JavaScript's native async / await model, which is useful for mixed Kotlin/TypeScript codebases [1]. The feature is enabled with the compiler option -Xsuspend-lambda-exporting added to freeCompilerArgs in build.gradle.kts [1], and the relevant declarations must be marked with @JsExport [3]. On the TypeScript side, the suspending lambda appears as a regular async function [3]. For example:

// Kotlin
@JsExport
class TaskRunner {
    suspend fun runTask(task: suspend () -> String): String {
        return task()
    }
}

// TypeScript
import { TaskRunner } from "..."

const runner = new TaskRunner();
const result = await runner.runTask(async () => "done");
console.log(result); // "done"
[3]

Sources
[1] What's new in Kotlin 2.4.20 — What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
https://kotlinlang.org/docs/whatsnew2420.html (chunk_id: 8e1ec2d8ace81b8363ec079ae1f0595044b41771a276870f562209986548880a)
[2] What's new in Kotlin 2.4.20 — What's new in Kotlin 2.4.20 > Kotlin/JS
https://kotlinlang.org/docs/whatsnew2420.html (chunk_id: edf32cc1084308f888efec92f4cca53fb6ca55574327ff907248980ce7cc44ce)
[3] What's new in Kotlin 2.4.20 — What's new in Kotlin 2.4.20 > Kotlin/JS > Support for exporting suspending lambdas as async functions
https://kotlinlang.org/docs/whatsnew2420.html (chunk_id: 5816283ee1a7c83c3e38fb5b56a06ded97ee2c70131eea4e9fe841a76713ec09)

Quotes
Chunk 8e1ec2d8ace81b8363ec079ae1f0595044b41771a276870f562209986548880a:
> With Kotlin 2.4.20, you can now export suspending lambda expressions as JavaScript async functions.
Chunk 8e1ec2d8ace81b8363ec079ae1f0595044b41771a276870f562209986548880a:
> Previously, there was no way to export declarations containing suspending lambdas from Kotlin/JS libraries. Now the Kotlin compiler automatically handles the bridging between Kotlin's suspend functions and JavaScript's native async / await model, which is useful for mixed Kotlin/TypeScript codebases.
Chunk 8e1ec2d8ace81b8363ec079ae1f0595044b41771a276870f562209986548880a:
> To enable this feature, add the following compiler option to your build.gradle.kts file:
Chunk edf32cc1084308f888efec92f4cca53fb6ca55574327ff907248980ce7cc44ce:
> Kotlin 2.4.20 improves exportability of data classes, introduces a new experimental DSL for browser testing, and adds support for exporting suspending lambdas as JavaScript async functions.
Chunk 5816283ee1a7c83c3e38fb5b56a06ded97ee2c70131eea4e9fe841a76713ec09:
> Then, mark the relevant declarations with @JsExport :
Chunk 5816283ee1a7c83c3e38fb5b56a06ded97ee2c70131eea4e9fe841a76713ec09:
> From the TypeScript side, the suspending lambda appears as a regular async function:
Chunk 5816283ee1a7c83c3e38fb5b56a06ded97ee2c70131eea4e9fe841a76713ec09:
> // Kotlin
> @JsExport
> class TaskRunner {
>     suspend fun runTask(task: suspend () -> String): String {
>         return task()
>     }
> }
Chunk 5816283ee1a7c83c3e38fb5b56a06ded97ee2c70131eea4e9fe841a76713ec09:
> // TypeScript
> import { TaskRunner } from "..."
>
> const runner = new TaskRunner();
> const result = await runner.runTask(async () => "done");
> console.log(result); // "done"

Review: The new export support, automatic async/await bridge, @JsExport requirement and both example programs agree with the displayed quotes. The important -Xsuspend-lambda-exporting flag and freeCompilerArgs location appear in the answer but are omitted from the selected quote: it ends immediately before the Gradle code. The full cited chunk contains the exact flag and configuration, so this is incomplete quote coverage, not an invented flag.

## q20

Which Kotlin 2.4.20 feature permits Swift implementations to extend exported Kotlin types?

Answer
Cross-language inheritance in Swift export is the Kotlin 2.4.20 feature that permits Swift implementations to extend exported Kotlin types [1]. Kotlin 2.4.20 introduces cross-language inheritance support in Swift export [1]. To use the reverse import pattern, you declare a Kotlin superclass for the Swift implementation to inherit from and a Kotlin interface, then implement this interface in Swift and pass the Swift object to Kotlin functions that accept that interface [1]. For example, on the Kotlin side, you can declare an interface, a function that accepts it, and an open base class [1].

Sources
[1] What's new in Kotlin 2.4.20 — What's new in Kotlin 2.4.20 > Kotlin/Native > New Swift export features > Cross-language inheritance in Swift export
https://kotlinlang.org/docs/whatsnew2420.html (chunk_id: c1410b74b67902dfcb3f11f836be1a711376892c554e75a1fefb0f4708a459cf)

Quotes
Chunk c1410b74b67902dfcb3f11f836be1a711376892c554e75a1fefb0f4708a459cf:
> Cross-language inheritance in Swift export
>
> Kotlin 2.4.20 introduces cross-language inheritance support in Swift export.
Chunk c1410b74b67902dfcb3f11f836be1a711376892c554e75a1fefb0f4708a459cf:
> To implement the pattern, declare a Kotlin superclass for the Swift implementation to inherit from and a Kotlin interface. Then implement this interface in Swift and pass the Swift object to Kotlin functions that accept that interface.
Chunk c1410b74b67902dfcb3f11f836be1a711376892c554e75a1fefb0f4708a459cf:
> On the Kotlin side, declare an interface, a function that accepts it, and an open base class:

Review: The quoted section names cross-language inheritance and Kotlin 2.4.20, then explicitly describes the Kotlin superclass/interface, Swift implementation and passing the object to Kotlin functions. The open base class sentence is also quoted. All factual assertions in this answer have direct support.

## q21

What is the internal production API endpoint for my company payroll service?

Answer
I don't know based on the available sources.

Could you clarify the Kotlin feature, platform, or version, or provide relevant documentation?

Sources
Unavailable: insufficient supporting evidence.

Quotes
Unavailable: insufficient supporting evidence.

Review: All candidates fell below the cutoff, leaving no packed evidence. The harness returned unknown and a clarification request, made no answer-model call, and supplied no private payroll endpoint or fabricated sources. The generic Kotlin clarification is less tailored than a payroll-specific request, but asking for relevant documentation still provides a way forward.

## q22

What breaking changes will Kotlin 3.0 ship with?

Answer
I don't know based on the available sources.

Could you clarify the Kotlin feature, platform, or version, or provide relevant documentation?

Sources
Unavailable: insufficient supporting evidence.

Quotes
Unavailable: insufficient supporting evidence.

Review: No evidence passed the cutoff for future Kotlin 3.0 breaking changes. The harness returned unknown with a version/documentation clarification request and no speculative release claims. No answer-model call, sources or quotes were fabricated.

## q23

How do I configure a PostgreSQL replication slot for Debezium?

Answer
I don't know based on the available sources.

Could you clarify the Kotlin feature, platform, or version, or provide relevant documentation?

Sources
Unavailable: insufficient supporting evidence.

Quotes
Unavailable: insufficient supporting evidence.

Review: No evidence passed the cutoff for PostgreSQL/Debezium configuration. The harness returned unknown with a documentation clarification request and no general-knowledge configuration advice. The Kotlin-oriented wording is generic for this out-of-domain question. No answer-model call, sources or quotes were fabricated.
