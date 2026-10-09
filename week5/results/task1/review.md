# Task1 live integration review

Reviewed the retained [demo.json](demo.json) capture on October 10, 2026
(Moscow). This is a single illustrative conversation, not a model benchmark.
The report records the exact artifact path and SHA-256, CLI argv/stdin/stdout/
stderr, three session snapshots, local settings, working-memory content, six
provider calls and Ollama status. UTC timestamps are on October 9; Moscow time
is already October 10.

The real CLI produced four completed answers and two completed local summaries.
All calls finished with `stop` without truncation. A newly started daemon loaded
the existing session after the capture stopped its isolated local predecessor.
The resumed answer identified Cedar as a Go CLI for offline notes. Explicit
working memory retained `language = Go`; summaries and session settings persisted.
The adapter used the installed quantized Spark model on loopback Ollama. No
cloud key was passed to the child processes; local startup skips Keychain lookup
and cloud balance/scheduler initialization. Deterministic tests separately verify
credential isolation, local-only transport and provider mismatch rejection.

| Response | Assessment |
| --- | --- |
| Project/language identification | Correctly retains Cedar, Go and offline notes. |
| Standard-library storage package | Incorrectly identifies `sqlite` as standard and associates it with `database/sql`. The latter is a generic SQL API and requires a separate database driver. The answer also ignores the requested one-sentence format. |
| Notes file format | SQLite is a plausible storage choice, but inherits the misleading standard-library assumption from the previous answer. |
| First summary | Correctly preserves the initial project facts. |
| Recall after restart | Correctly preserves the project name, language and purpose. |
| Second summary | Keeps the project facts but repeats the generated SQLite error; this demonstrates that summarization preserves claims without validating them. |

The integration, local generation, summary persistence and restart/resume checks
pass. Technical correctness is mixed. Mock tests verify mechanics, not semantic
quality. No code from these answers was compiled or executed. Model inspection
and CLI polling are excluded from the per-call durations; hardware operating
costs are excluded from the zero API-charge totals.

The temporary demo project was removed after capture; its stored session and
project memory remain in the isolated local state directory. For a new
conversation use `advent-agent --profile local --chat` in the desired project.
For a recording rerun the capture script with a fresh output path after tests
and rebuild, preserving this original report.
