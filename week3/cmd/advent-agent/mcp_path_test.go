package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAbsoluteMCPCommandKeepsDaemonIndependentOfCLICwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := absoluteMCPCommand([]string{"./supplier-mcp", "--config", "config.json"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(cwd, "supplier-mcp"), "--config", "config.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command=%v want=%v", got, want)
	}
	bare, err := absoluteMCPCommand([]string{"npx", "-y", "package"})
	if err != nil || bare[0] != "npx" {
		t.Fatalf("bare command=%v err=%v", bare, err)
	}
}
