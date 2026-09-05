# deepseek-asker

`deepseek-asker` sends one prompt to DeepSeek and prints only the model's answer
to standard output. It uses `deepseek-v4-flash` in non-thinking mode and has no
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

Use `-d`/`--debug` to print the HTTP request and full response to standard
error while keeping the answer alone on standard output:

```sh
./deepseek-asker --debug -p 'Hello' >answer.txt 2>debug.log
```

The `Authorization` header is redacted from debug output. Treat debug logs as
sensitive anyway because they contain the prompt and the complete API response.
