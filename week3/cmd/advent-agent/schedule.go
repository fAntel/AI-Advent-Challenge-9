package main

import (
	agent "advent-agent"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
)

func runSchedule(api *agent.APIClient, cfg agent.Config, args []string) {
	if len(args) == 0 {
		fatal(errors.New("usage: advent-agent schedule add|list|remove|run|reload"))
	}
	fatalIf(ensureDaemon(api, cfg))
	switch args[0] {
	case "reload":
		if len(args) != 1 {
			fatal(errors.New("usage: schedule reload"))
		}
		jobs, err := api.ScheduleReload()
		fatalIf(err)
		for _, j := range jobs {
			fmt.Printf("%s  every=%s  next=%s\n", j.Name, j.Every, j.NextRun)
		}
	case "list":
		jobs, err := api.ScheduleList()
		fatalIf(err)
		for _, j := range jobs {
			fmt.Printf("%s  %s/%s  next=%s  last=%s  error=%s\n", j.Name, j.Server, j.Tool, j.NextRun, j.LastRun, j.LastError)
		}
	case "remove":
		if len(args) != 2 {
			fatal(errors.New("usage: schedule remove NAME"))
		}
		fatalIf(api.ScheduleRemove(args[1]))
	case "run":
		if len(args) != 2 {
			fatal(errors.New("usage: schedule run NAME"))
		}
		job, err := api.ScheduleRun(args[1])
		fatalIf(err)
		fmt.Println(job.LastResult)
	case "add":
		fs := flag.NewFlagSet("schedule add", flag.ExitOnError)
		server := fs.String("server", "", "registered MCP server")
		tool := fs.String("tool", "", "MCP tool name")
		arguments := fs.String("args", "{}", "JSON object of tool arguments")
		every := fs.String("every", "", "recurring interval (for example 1h)")
		at := fs.String("at", "", "one-time RFC3339 timestamp")
		notify := fs.Bool("notify", false, "push result to connected chats")
		fatalIf(fs.Parse(args[1:]))
		if fs.NArg() != 1 {
			fatal(errors.New("usage: schedule add [--server NAME] [--tool NAME] [--args JSON] [--every DURATION|--at RFC3339] [--notify] NAME"))
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(*arguments), &parsed); err != nil || parsed == nil {
			fatal(errors.New("--args must be a JSON object"))
		}
		if *every == "" && *at == "" {
			*every = "default"
		}
		fatalIf(api.ScheduleAdd(agent.ScheduleJob{Name: fs.Arg(0), Server: *server, Tool: *tool, ArgumentsJSON: *arguments, Every: *every, At: *at, Notify: *notify}))
	default:
		fatal(errors.New("usage: advent-agent schedule add|list|remove|run|reload"))
	}
}
