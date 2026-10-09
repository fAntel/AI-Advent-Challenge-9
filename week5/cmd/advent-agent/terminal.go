package main

import (
	"encoding/json"
	"os"
)

func speaker(label, color string) string {
	info, err := os.Stdout.Stat()
	tty := err == nil && info.Mode()&os.ModeCharDevice != 0
	_, noColor := os.LookupEnv("NO_COLOR")
	return styleLabel(label, color, tty, os.Getenv("TERM"), noColor)
}
func styleLabel(label, color string, tty bool, term string, noColor bool) string {
	if !tty || term == "dumb" || noColor {
		return label
	}
	return "\x1b[1;" + color + "m" + label + "\x1b[0m"
}
func formatJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
