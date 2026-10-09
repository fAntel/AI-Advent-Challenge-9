package main

import (
	agent "advent-agent"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

func runPipeline(api *agent.APIClient, cfg agent.Config, args []string) {
	if len(args) == 0 {
		fatal(errors.New("usage: pipeline add|list|show|step|run|remove"))
	}
	fatalIf(ensureDaemon(api, cfg))
	switch args[0] {
	case "add":
		if len(args) != 2 {
			fatal(errors.New("usage: pipeline add NAME"))
		}
		fatalIf(api.PipelineAdd(args[1]))
	case "list":
		if len(args) != 1 {
			fatal(errors.New("usage: pipeline list"))
		}
		items, err := api.PipelineList()
		fatalIf(err)
		for _, item := range items {
			fmt.Printf("%s  steps=%d\n", item.Name, len(item.Steps))
		}
	case "show":
		if len(args) != 2 {
			fatal(errors.New("usage: pipeline show NAME"))
		}
		item, err := api.PipelineGet(args[1])
		fatalIf(err)
		data, err := json.MarshalIndent(item, "", "  ")
		fatalIf(err)
		fmt.Println(string(data))
	case "remove":
		if len(args) != 2 {
			fatal(errors.New("usage: pipeline remove NAME"))
		}
		fatalIf(api.PipelineRemove(args[1]))
	case "run":
		if len(args) != 2 {
			fatal(errors.New("usage: pipeline run NAME"))
		}
		// The daemon owns the deadline; allow its error response to reach the CLI.
		api.HTTP.Timeout += 10 * time.Second
		run, err := api.PipelineRun(args[1])
		fatalIf(err)
		fmt.Println(run.Result)
	case "step":
		if len(args) < 2 {
			fatal(errors.New("usage: pipeline step add|remove ..."))
		}
		switch args[1] {
		case "add":
			fs := flag.NewFlagSet("pipeline step add", flag.ExitOnError)
			server := fs.String("server", "", "registered MCP server")
			tool := fs.String("tool", "", "MCP tool")
			arguments := fs.String("args", "{}", "literal JSON arguments")
			bindings := map[string]string{}
			fs.Func("bind", "ARG=EARLIER_STEP:/json/pointer or ARG=EARLIER_STEP:$text; repeatable", func(value string) error {
				target, source, ok := strings.Cut(value, "=")
				if !ok || target == "" || source == "" {
					return errors.New("--bind requires ARG=STEP:POINTER")
				}
				if _, exists := bindings[target]; exists {
					return fmt.Errorf("duplicate binding %q", target)
				}
				bindings[target] = source
				return nil
			})
			fatalIf(fs.Parse(args[2:]))
			if fs.NArg() != 2 {
				fatal(errors.New("usage: pipeline step add --server SERVER --tool TOOL [--args JSON] [--bind ARG=STEP:/pointer] PIPELINE STEP"))
			}
			var parsed map[string]any
			if err := json.Unmarshal([]byte(*arguments), &parsed); err != nil || parsed == nil {
				fatal(errors.New("--args must be a JSON object"))
			}
			encoded, err := json.Marshal(bindings)
			fatalIf(err)
			fatalIf(api.PipelineStepAdd(fs.Arg(0), agent.PipelineStep{Name: fs.Arg(1), Server: *server, Tool: *tool, ArgumentsJSON: *arguments, BindingsJSON: string(encoded)}))
		case "remove":
			if len(args) != 4 {
				fatal(errors.New("usage: pipeline step remove PIPELINE STEP"))
			}
			fatalIf(api.PipelineStepRemove(args[2], args[3]))
		default:
			fatal(errors.New("usage: pipeline step add|remove ..."))
		}
	default:
		fatal(errors.New("usage: pipeline add|list|show|step|run|remove"))
	}
}
