# HEALBE data MCP

Standalone Kotlin/JVM stdio MCP server using **server-api-jvm 1.6.0** (latest
published release verified on October 19, 2026). No LLM runs inside the server.
The harness creates synthetic values and selects tools. No Android runtime,
device connection, RAG index, registration, firmware or weight-goal APIs are needed.

## Build

Build requirements: JDK 21 and private Maven access. Gradle reads
`nexusReleaseUrl`, `nexusPrivateUrl`, `nexusPrivateUsername`, and
`nexusPrivatePassword` from the existing global `~/.gradle/gradle.properties`.
Dependencies are pinned; the SDK checkout is not used to build the application.

From Week6:

```sh
make build-data
make test
make build
```

`build-data` runs JVM tests and creates a host-platform application image using
`jpackage`. The macOS executable is:

```text
week6/data-mcp/build/package/healbe-data-mcp.app/Contents/MacOS/healbe-data-mcp
```

Distribute the **whole application bundle**, not just its executable. It includes
Java and dependencies; users do not install Java or Gradle. Builds are specific
to the host OS and architecture. The bundle is unsigned. Other-platform packaging
is not verified. Generated distributions and reference sources are Git-ignored.

For JVM development, `build/install/healbe-data-mcp/bin/healbe-data-mcp` uses an
installed Java runtime. `make test-data` runs JVM tests and the real stdio/SDK
integration against a local HTTP fixture. The ordinary Python suite skips these
integration tests if `installDist` has not been built.

## Connect

Any stdio MCP client can launch the bundled executable directly; it implements
MCP protocol 2024-11-05 initialization, ping, tool listing and tool calls. No HTTP
MCP listener is exposed. Stdout contains protocol messages only.

The normal Week6 CLI automatically discovers a bundled executable beside the
harness in the build output locations above. Set `data_mcp` to override the
absolute executable path, or `data_mcp = ""` to disable automatic selection.
Restart the daemon after building/updating or changing configuration.

`./advent-agent mcp list` performs direct discovery with the CLI's current
configuration, without a model, credentials, or account API calls. It labels
this as direct inspection, not inspection of a previously running daemon.
Natural-language questions about connected tools use `list_available_tools`,
which reports the actual connections in that operation. Git is identified as
configured on demand; Mattermost is not connected to the developer harness. The harness discovers
schemas dynamically and prefixes tool names with `healbe_`. Each operation owns
one MCP subprocess. Tool results become evidence for the existing final-answer
citation checks. The read-only support bot rejects these tools.

Tool arguments optionally include `login` and `password` together. Otherwise,
the server uses `HEALBE_LOGIN` and `HEALBE_PASSWORD` from its process environment.
The Week6 harness also accepts a private JSON credentials file with `login` and
`password` keys, defaulting to `healbe-credentials.json` inside its state directory.
Override its absolute path using `data_mcp_credentials`. Missing files are allowed;
existing files must be regular files with no group/other permissions (0600).
The harness injects these values into the MCP process only when neither account
environment variable is set. Explicit tool credentials take precedence. The file
is excluded from agent file access; do not commit it. This workstation's authorized
SDK test account has been configured there for normal CLI use.

Each call authenticates internally through `UserService` and a fresh
`InMemoryAccessProvider`; access IDs are never tool outputs. Library logging is
disabled. Supplying credentials through chat sends them to the model and stores
the user message in session history. Tool-operation records redact the password,
but this does not erase the original user message. Prefer environment defaults
for the video. Do not record credentials.

`HEALBE_ENDPOINT` optionally overrides the SDK endpoint for local integration
tests or a configured test server. It is a launch setting, not a model-controlled
tool argument. The demonstration helper uses the real SDK default endpoint.

## Tools and behavior

- Ten `download_*` and ten `upload_*` tools wrap DataService: short/full summaries,
  pressures, generic/sleep/system events, sleep information, meals, anxieties and
  resync ranges. Record schemas come from the published Kotlin serializers.
- `download_weights_by_measurement_time`, `download_weights_by_last_update`,
  `add_weight`, `add_batch_weight`, and `update_weight` wrap WeightService.
  Weights are kilograms, height is centimeters, timestamps are Unix **seconds**.
  Optional `additionalData.muscleMass` is kilograms.
- Upload batches contain 1–100 records. DataService always uses `resync=false`;
  matching record identities can still be overwritten. No automatic retries.
- Weight batches validate all inputs first and stop at the first failed/unknown
  item. Results identify accepted indexes and the unattempted remainder through
  requested/attempted counts. A failed transport does not prove the write failed.
- Weight updates require an ID and cannot delete. Updates replace optional
  fields in the server: read and preserve existing values before editing.
- SDK 1.6.0 weight downloads both filter **measurement time** and ignore the SDK
  limit. The MCP wrapper slices the returned list with `offset`/`limit`, `total`,
  `next_offset`, and `has_more`. This is not a server snapshot or server pagination.
- SDK weight additions may return an ID even when body-composition enrichment
  fails. Always read back and compare the requested fields before claiming success.
- Download pages default to 50 records, maximum 100. Results over 28 KB return an
  explicit size error; reduce the page size. Domain plausibility is the model's
  responsibility; input validation does not constitute physiological validation.

## Switching accounts safely

The daemon keeps the environment it started with. Stop it before changing
accounts; exporting variables in another terminal cannot update a running daemon.
In zsh, use hidden password input to keep the password out of shell history:

```sh
./advent-agentd stop
read -r 'HEALBE_LOGIN?Test account email: '
read -rs 'HEALBE_PASSWORD?Password: '
echo
export HEALBE_LOGIN HEALBE_PASSWORD
./advent-agent
```

Both variables must be set together. Explicit environment values override the
private credentials file. Unsetting both and restarting restores the file's
account. Use a fresh test account for a clean video; rerunning generation can add
another series. There is no automatic cleanup.

The harness provides `plan_daily_timestamps` for correct UTC dates and Unix
seconds, and `render_weight_chart` for ASCII output derived from selected records
in MCP download evidence. These are local harness helpers, not extra MCP tools.

## Video / live capture

After successful tests and builds, from Week6:

```sh
python3 scripts/demo_data.py --probe
python3 scripts/demo_data.py --chat
```

These commands use the exact bundled executable above and the freshly rebuilt
`week6/advent-agent` / `week6/advent-agentd`. The helper reads the SDK test account
from `~/AndroidStudioProjects/healbe-sdk-android/` into process environment only;
`--sdk PATH` changes that location. Explicit `HEALBE_LOGIN` / `HEALBE_PASSWORD`
override account discovery. Nothing copies credentials into repository files.

Suggested 2–3 minute recording:

1. Explain: “The model generates test data; MCP uses our actual server library.”
2. Run `--chat` and ask for 30 daily synthetic weigh-ins with a modest weight and
   muscle-mass increase, daily variation, and read-back verification. The helper
   prints a ready-to-paste prompt with concrete dates. It configures credentials
   privately and uses isolated daemon state.
3. Show the resulting confirmation and evidence. Ask which fields were verified.
4. Open/sync the mobile app with the same account and show the corresponding
   date range. This UI check is manual and separate from server verification.

For a reproducible model-driven capture with independent comparison:

```sh
python3 scripts/demo_data.py --output results/task4-another-demo.json
```

This **uploads real synthetic records** to the test account. It reads existing
records first through the model's tools, asks the model to preserve them, and
checks all generated timestamps, weights and muscle masses after execution.
Captures refuse overwrite, redact the test credentials, include tool records
and artifact hashes, and stop the isolated daemon. A rerun can create another
dataset; it is not a cleanup or rollback command. Video recording is user-operated.
