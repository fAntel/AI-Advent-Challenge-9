# Task3 — Local relevance scorer optimization

Spark Q4_K_M is fixed. No quantization experiment or cloud calls. The first Task2 pass supplies the frozen candidates; labels were authored before tuning by the implementing assistant, not an independent human reviewer.

## Screening

| Variant | Median scoring | Filter recall | Packed recall | Errors |
| --- | ---: | ---: | ---: | ---: |
| baseline | 81.83 s | 90.9% | 72.7% | 0 |
| temperature0 | 54.58 s | 90.9% | 81.8% | 0 |

Screening uses q01, q13, q19 and q23 once per variant, excluding warmup. It is noisy and selection-biased; the repeated comparison is the adoption test.

Selected scorer:

```json
{
  "temperature": 0
}
```

Repeated scoring comparison is incomplete; no adoption claim is available.

## Full-pipeline comparison

| Arm | Valid first attempts | Median trial | Correct unknown controls | Fully quote-supported factual answers | Expected facts fully covered |
| --- | ---: | ---: | ---: | ---: | ---: |
| baseline | Incomplete | — | — | — | — |
| finalist | Incomplete | — | — | — | — |

Rejected outputs receive no delivered-answer credit. Semantic reviews are hash-bound to saved trials; quote validation alone is insufficient. Finalist adoption: **not established**.

## Resources

| Arm | Peak sampled model allocation | Peak sampled Ollama RSS | Swap range |
| --- | ---: | ---: | ---: |

Allocation and RSS are different measurements and must not be added. Five-second samples can miss peaks. Swap and VM statistics are system-wide, not attributable solely to this task. Runs share the same host and are not thermally controlled. Full-pipeline timings include context changes, model loading, retrieval, and validation.

## Reproduction

Run `make test` and `make build`, then `python3 scripts/prepare-score-bench.py` and `python3 scripts/run-score-bench.py all`. The runner uses the absolute Week5 `advent-agent` binary, workspace-only profiles, and removes the cloud key from child environments. It refuses to resume measurements with a changed binary. Review saved answers before running this renderer.
