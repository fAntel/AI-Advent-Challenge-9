# DeepSeek Pro versus Flash: the same ten hard Kotlin questions

**Flash was competitive with Pro in this run, and cheaper and faster on the recorded provider calls. RAG helped both much more consistently than choosing Pro over Flash.** Flash RAG satisfied 26 of the 30 pre-recorded expected facts; Pro RAG satisfied 25. This is a small, deliberately difficult, source-reviewed example—not a general model ranking.

I generated and personally reviewed twenty additional Flash answers: ten plain and ten with RAG. DeepSeek generated answers only; it did not grade either model. The original Pro answers and reviews were preserved.

- [Pro: all answers and individual reviews](comparison.md)
- [Flash: all answers and individual reviews](flash/comparison.md)
- Exact prompts, evidence, settings, usage and latency: [Pro](answers.json), [Flash](flash/answers.json)
- Separate manually authored assessments: [Pro](review.json), [Flash](flash/review.json)
- [Questions, expected facts and selection rationale](../../rag-suite.json)

## Conditions

Both models received **exactly identical messages for each corresponding question/mode**, verified from the saved JSON—not merely similar prompts. All ten retrieved evidence snapshots were also identical. The same suite and index SHA-256 fingerprints were verified. Model was the only provider-setting difference: `deepseek-v4-pro` versus `deepseek-v4-flash`.

Both used low reasoning, no temperature override, independent questions with no history/memory/project instructions/tools, and alternating plain/RAG order. RAG used semantic retrieval, five candidates and a 6,000-character whole-chunk budget through `week4/.venv/bin/kotlin-index`. Generation used the already tested and rebuilt `week4/advent-agent`.

I used the same expected facts recorded before the Pro run and read each Flash answer against the saved source sections before assigning paired/model outcomes. This was not a blinded review: I knew the model and had seen the Pro results. The saved corpus is the reference standard; examples were not compiled. Missing includes partial coverage. Some expected facts ask for useful background beyond the literal question, so these counts are completeness checks, not percentages of answers that are “correct.” Extra unsupported claims and citation problems are assessed separately.

## Expected-fact checks and recorded generation costs

| Model / mode | Facts satisfied | Missing / partial | Incorrect | Provider seconds | Total tokens | Estimated USD |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Pro / plain | 12/30 | 8 | 10 | 188.91 | 18164 | $0.015547 |
| Pro / rag | 25/30 | 4 | 1 | 52.19 | 16282 | $0.009055 |
| Flash / plain | 17/30 | 9 | 4 | 115.31 | 24740 | $0.006808 |
| Flash / rag | 26/30 | 4 | 0 | 36.67 | 19351 | $0.003737 |

The cost figures use the existing adapter’s estimated rates and provider-reported usage, not a separately reconciled bill. Provider latency excludes retrieval. These were sequential single runs; differences in answer length, reasoning, cache usage, and service conditions prevent treating the timing/cost ratios as controlled performance benchmarks. More tokens can still cost less on a differently priced model.

## My question-by-question model comparison

“Similar” means no clear material content winner in this sample; it does not mean the answers are identical or flawless.

| Question | Without RAG: Pro versus Flash | With RAG: Pro versus Flash |
| --- | --- | --- |
| q01: implicit services | **Flash advantage:** names the newer context-parameter syntax and acknowledges uncertainty. Pro primarily recommends the old context receivers. Flash still centers its example on the older form. | **Similar:** both explain the current syntax, type matching and same-scope ambiguity. Flash gives a clearer availability caveat but is longer. |
| q11: expect/actual placement | **Mixed:** both cover signature matching, but both add rules not verified by the reviewed source. Flash adds more questionable details; Pro also overstates modifier matching. | **Flash advantage in transparency:** both omit explicit signature rules, but Flash explicitly says the supplied excerpt does not establish them. Core expected-fact coverage is the same. |
| q12: actual typealias | **Flash advantage in completeness:** includes the package requirement and class Beta caveat; Pro omits the former and calls the feature stable. Flash’s extra only-classes restriction remains unverified. | **Flash advantage:** omits the compatibility requirement but does not repeat Pro’s false claim that aliases cannot satisfy expected members. Neither gives a fully satisfactory answer. |
| q13: Swift exceptions | **Flash modest advantage:** explains that an unlisted exception still terminates; Pro omits that qualification. Both miss the unannotated suspend CancellationException exception and include unverified extras. | **Similar:** both cover the required propagation rules, including subclasses and cancellation. Both use an IOException example needing an appropriate Native definition/import. |
| q14: Swift suspend export | **Flash advantage on the main answer:** correctly identifies direct async export; Pro confuses it with callbacks and suggests unsupported flags. Flash still adds outdated Flow and Objective-C claims. | **Similar:** both correctly map async/try-await and give dispatcher guidance. Flash explains the Objective-C distinction more fully. Both cite a chunk that does not contain their Alpha-status claim, although the complete source confirms it. |
| q16: existing JS API | **Pro advantage on avoiding extra errors:** both know external, but Flash adds an unsupported JsGlobal annotation and a false blanket no-body rule. Pro’s plugin-version advice is also unverified. | **Mixed:** both answer the core question and miss detailed rules. Pro labels the Wasm material but overgeneralizes its experimental status; Flash limits the warning to nativeInvoke but does not clearly label the example’s Wasm target. |
| q17: Wasm handles | **Flash advantage on the mechanism, neither example reliable:** Flash names JsReference and get() but proposes unsupported construction/release APIs; Pro wrongly exports and uses a Kotlin class directly from JS. | **Similar:** both use the documented JsReference/toJsReference/get helper pattern and appropriate JS import. |
| q18: UUID stability | **Neither answers the requested date correctly:** Pro says 2.0.20, Flash says 2.1.0; source says 2.4.0. Flash correctly remembers experimental introduction in 2.0.20; Pro does not. | **Similar:** both give 2.4.0 and V4/V7 exceptions. Both omit introduction history, which is a secondary completeness check. |
| q19: suspending JS lambdas | **Mixed:** Pro identifies the change but falsely says it is enabled by default. Flash admits it cannot confirm the release, yet adds an incompatible wrapper example. Neither provides the needed enabling flag. | **Pro slight citation advantage:** both give the right flag and export pattern. Flash additionally cites source chunks for a compatibility caveat that actually came from the harness instructions. Both omit the older 2.3.0 feature’s own enabling flag in their historical aside. |
| q20: Swift inheritance | **Flash advantage:** mentions interfaces and avoids Pro’s unsupported enabling flag/default-status speculation. Both omit the return path into Kotlin. | **Flash advantage in completeness:** explains passing the Swift implementation back to Kotlin through the interface and supplies source-backed status context. Pro stops after naming inheritance and interfaces. |

## What RAG changed within each model

- **Pro:** 7 improvements, 1 regression, 1 mixed result, 1 largely unchanged result.
- **Flash:** 8 improvements, 1 regression in completeness, 1 mixed result.

These are manual holistic judgments, not deductions from fact-count totals. For example, Flash q16 improves by removing unsupported extras even though its three expected-fact statuses do not change. On q12, Flash RAG loses an important compatibility condition; Pro RAG makes a stronger, false restriction. The same retrieval evidence can lead different models to different mistakes.

For this exact workload, paying for Pro did not produce a clear quality advantage. Flash with RAG answered most of the version-sensitive questions well at a lower observed cost, but neither model eliminated source-coverage gaps, unsupported extras, or citation overreach. Repeated trials and a larger independent question set would be needed before making a broad model-selection claim.

## Reproduce generation

From `week4/`, using the previously tested and rebuilt artifacts:

```sh
./advent-agent rag compare --suite rag-suite.json --output reports/task1/flash/answers.json --model deepseek-v4-flash --reasoning low --rag-strategy semantic
.venv/bin/python scripts/render_rag_review.py --answers reports/task1/flash/answers.json --review reports/task1/flash/review.json --output reports/task1/flash/comparison.md
```

The existing answer path resumes/skips completed answers. Use a new output path to generate another sample; new answers need new manual reviews. The renderer never recreates these judgments automatically.
