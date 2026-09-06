# deepseek-asker

`deepseek-asker` sends prompts to DeepSeek and prints the final model answer to
standard output. It uses `deepseek-v4-flash` in non-thinking mode and has no
third-party dependencies.

## Build

```sh
go build -o deepseek-asker .
```

Set your DeepSeek API key before running the tool:

```sh
export DEEPSEEK_API_KEY='your-api-key'
```

## Usage

Pass a prompt as an option:

```sh
./deepseek-asker --prompt 'Explain goroutines briefly'
./deepseek-asker -p 'Explain goroutines briefly'
```

Or pipe a prompt through standard input:

```sh
printf 'Explain goroutines briefly' | ./deepseek-asker
```

When run without `-p`/`--prompt` and without redirected input, the tool displays
`Prompt: ` and sends the line when you press Enter. Piped input is read in full
until EOF. The prompt option always takes precedence over standard input. Run
`./deepseek-asker --help` for the complete option list.

When a prompt was entered interactively, a `/ - \\ |` spinner updates every
100 milliseconds on standard error and disappears when the response arrives.
Interactive clarification mode also displays it for every request. Ordinary
`-p`/`--prompt` calls and redirected/non-interactive runs do not display it.

## Answer controls

Use `-f`/`--format` to supply a file containing the exact format required for
the final answer. The file is read once before the first API request, and its
contents are preserved. Missing, unreadable, directory, empty, and
whitespace-only files are rejected. An explicitly supplied empty or
whitespace-only path is also rejected.

```sh
./deepseek-asker -p 'Summarize this proposal' --format summary-format.txt
```

Use `-l`/`--length` to describe the desired final-answer length. This is a
semantic instruction, so values such as `three paragraphs` or `about 500 words`
are accepted as written.

```sh
./deepseek-asker -p 'Compare Go and Rust' --length 'three paragraphs'
```

Format and length values are applied through a system message; the original
user prompt is not modified. Empty or whitespace-only length values are
rejected. The options can be combined:

```sh
./deepseek-asker -p 'Write a release note' -f release-format.txt -l '150 words'
```

## Prompt approaches

Use `-a`/`--approach` to select one prompt strategy. The option accepts exactly
one of these values:

- `none` sends the prompt without an additional approach instruction. This is
  the default and preserves the ordinary one-request behavior.
- `step-by-step` asks DeepSeek to solve the request step by step and present the
  resulting steps clearly.
- `self-prompt` first asks DeepSeek to turn the user request into a clear,
  self-contained solving prompt. It then sends that generated prompt in a
  second request and prints only the second response.
- `multi-role` asks for separate answers from multiple perspectives. By
  default, it uses a business analyst, an engineer, and a critic, and prints
  each answer below a heading naming its role.

```sh
./deepseek-asker -p 'Design an inventory service' --approach step-by-step
./deepseek-asker -p 'Review this product proposal' -a multi-role
./deepseek-asker -p 'Plan a database migration' -a self-prompt
```

Override the default multi-role perspectives with `--roles` and a
comma-separated list containing at least two roles. Surround the list with
quotes when role names contain spaces:

```sh
./deepseek-asker -p 'Create a Space Wolves colour scheme using P3 paints' \
  --approach multi-role \
  --roles 'Miniature painter, Space Wolves lore expert, Critical reviewer'
```

Without `--format`, multi-role output uses one `## Role name` Markdown section
per role, in the same order as the list, with no unlabeled introduction or
conclusion. Each role is asked for two to four short paragraphs separated by
blank lines, with bullets, numbered steps, or tables where they improve
scanability. The response may cover the role's assessment, concrete
recommendations, and risks or trade-offs when those elements are useful.

When `--format` is supplied, that format replaces the built-in Markdown
structure and DeepSeek is instructed to attribute every answer within the
custom format. Using `--roles` with another approach is rejected.

Approaches are prompt strategies only; they do not change the API's thinking
setting. The option names are case-sensitive, and an empty or unknown value is
rejected with the list of valid approaches.

All approaches can be combined with format, length, and clarification controls.
For `self-prompt`, format and length apply only to the second, final response.
When it is combined with `--stop`, DeepSeek gathers clarifications first, uses
the completed conversation to generate the self-contained prompt, and then
solves that prompt. The generated intermediate prompt is hidden unless debug
logging is enabled.

## Clarification mode

Use `-s`/`--stop` to let DeepSeek ask one concise clarifying question at a time.
Each model question is written to standard error, followed by a `Your answer: `
prompt in interactive terminals. Enter one response per line; when a response
contains the stop sequence as a case-insensitive substring, the tool sends that
response unchanged, writes the final answer to standard output, and exits. An
empty or whitespace-only stop sequence is rejected.

```sh
./deepseek-asker -p 'Plan a weekend trip' --stop READY
# DeepSeek's questions appear on stderr; answer each on stdin.
# Include READY in an answer when it has enough information.
```

If the initial `--prompt` already contains the stop sequence, the tool makes one
request and treats the response as final. Without `--prompt`, clarification mode
reads the initial prompt and every follow-up one line at a time. Redirected input
therefore supplies one conversation turn per line:

```sh
printf '%s\n' 'Plan a team offsite' 'Lisbon' 'Budget is $5000; READY' |
  ./deepseek-asker --stop READY >plan.txt 2>questions.log
```

Ending input before the stop sequence is seen is an input error. Format and
length controls may be combined with clarification mode; they govern the final
answer while intermediate questions remain concise. Each request also states
whether the latest user message contains the stop sequence, reinforcing that
non-final responses must contain exactly one concise question and nothing else.

Use `-d`/`--debug` to print the HTTP request and full response to standard
error while keeping the answer alone on standard output:

```sh
./deepseek-asker --debug -p 'Hello' >answer.txt 2>debug.log
```

The `Authorization` header is redacted from debug output. Treat debug logs as
sensitive anyway because they contain the complete conversation and API
responses. Debug logging and response validation apply to every request in a
clarification conversation.
