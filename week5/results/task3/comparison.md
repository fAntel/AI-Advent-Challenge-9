# Task3 — Bounded local scorer optimization

The long repeated experiment was stopped at the user’s request. This report reuses the controlled four-question baseline and runs the ordered-score finalist once, with a separate deadline of 60% of each baseline duration. Warmup is capped at 60 seconds and excluded. Timeouts are terminal failures, receive zero delivered recall, and cannot qualify by being fast.

Spark Q4_K_M weights and quantization are unchanged. Both arms use the same frozen passages and score sorting. Labels were authored by the implementing assistant before tuning; these four examples were also used for selection, so this is a development check, not a held-out evaluation.

## Settings

```json
{
  "temperature": 0,
  "prompt_version": "compact-v2",
  "batch_size": 4,
  "context_window_tokens": 8192,
  "output_tokens": 64
}
```

## Bounded validation

| Question | Baseline | Deadline | Finalist | Result |
| --- | ---: | ---: | ---: | --- |
| q01 | 49.20 s | 29.52 s | 6.60 s | Completed |
| q13 | 75.31 s | 45.18 s | 30.86 s | Completed |
| q19 | 93.75 s | 56.25 s | 38.71 s | Completed |
| q23 | 117.41 s | 70.45 s | 50.80 s | Completed |

| Measurement | Baseline | Finalist |
| --- | ---: | ---: |
| Successful-trial median seconds | 84.53 | 34.79 |
| Filter recall | 90.9% | 100.0% |
| Filter precision | 52.6% | 45.8% |
| Packed evidence recall | 81.8% | 81.8% |
| nDCG@5 | 90.7% | 85.2% |
| Retained chunks on unsupported control | 0 | 0 |
| Failed trials | 0 | 0 |
| Recorded input tokens | 39954 | 23690 |
| Recorded output tokens | 1209 | 283 |
| Recorded calls | 40 | 20 |

Median scoring speedup: **2.43×**.

Scoring gate: **passed**. The target remains 2× median speedup and no more than a five-percentage-point drop in filter or packed recall, with no failures. The 0.6 deadline is a permissive cutoff (1.67×), not the success target. Successful-only medians must not be interpreted as an overall speedup when other trials fail.

Answer generation settings are unchanged, but evidence can change. Answer-quality equivalence has not been established; the existing default profile remains in place. The cancelled three-pass run and older uncontrolled-cache pilot are excluded. Token counts on failed trials cover only recorded calls, not necessarily all interrupted server work.

## Screening

| Variant | Median seconds | Filter recall | Packed recall | Errors |
| --- | ---: | ---: | ---: | ---: |
| baseline | 84.53 | 90.9% | 81.8% | 0 |
| temperature0 | 94.56 | 90.9% | 81.8% | 0 |
| compact | Failed | — | — | 1 |
| batch4 | 108.73 | 100.0% | 90.9% | 0 |
| batch5 | 98.05 | 100.0% | 72.7% | 0 |
| context4096 | Failed | — | — | 1 |
| context8192 | 123.95 | 90.9% | 81.8% | 0 |
| output | 97.94 | 100.0% | 81.8% | 0 |
| ordered | 42.23 | 100.0% | 81.8% | 0 |

The ordered variant’s screening median was approximately 2× faster, with equal packed recall and higher filtering recall, but lower filtering precision. Screening selected the candidate; bounded validation determines whether it meets the runtime constraint.

## Resources

| Run | Peak sampled model allocation | Peak sampled Ollama RSS | System swap range |
| --- | ---: | ---: | ---: |
| screen-baseline-1 | 2676.0 MiB | 3074.8 MiB | 5863.2–11816.6 MiB |
| bounded-finalist-1 | 2736.3 MiB | 3540.2 MiB | 1638.7–3256.9 MiB |

Five-second samples include warmup and can miss peaks. Allocation and RSS are distinct and must not be added. Swap is system-wide. Both runs unload the model before warmup; separate executions still vary with system load and cache state.

## Reproduction

After `make test` and `make build`, use `python3 scripts/run-score-bench.py bounded`, then `python3 scripts/render-score-bench.py`. Existing completed captures are immutable; use a fresh output directory to run a rebuilt binary. The runner invokes `/Users/keldzh/Develop/AiAdventChallenge/week5/advent-agent` with isolated workspace profiles. The CLI accepts `--baseline`, `--timeout-factor 0.6`, and `--warmup-timeout 60s`; the saved baseline hash binds the per-question budgets.

## Answer spot checks

Two full-pipeline trials, each capped at 90 seconds. Baseline answers below are historical Task2 first-pass outputs, not newly generated under controlled timing. These checks cannot establish full-suite equivalence.

| Question | Arm | Result | Expected facts fully covered | Review |
| --- | --- | --- | ---: | --- |
| q01 | historical baseline | Failed: invalid RAG quote for chunk "2dbf892f46e37b486442c3499d22535ad4217ce70468e4594e9f09707c102a31": not an exact evidence fragment; retry the operation | 0/3 | Rejected by exact-quote validation; no answer delivered. |
| q23 | historical baseline | unknown | N/A | Correct abstention: supplied Kotlin documentation does not answer the Debezium/PostgreSQL question. |
| q01 | finalist | Failed: Ollama unavailable or request interrupted; check ollama serve: Post "http://127.0.0.1:11434/api/chat": context deadline exceeded | 0/3 | The complete pipeline hit its 90-second deadline during answer generation; no answer delivered. Scoring had completed. |
| q23 | finalist | unknown | N/A | Correct abstention: no retrieved Kotlin passage supports an answer about Debezium/PostgreSQL. |

Failed or unreviewed outputs receive no delivered-answer quality credit. Semantic reviews are hash-bound to the saved trials. The default profile is unchanged.
