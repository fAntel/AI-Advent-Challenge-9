package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDataMCPRealStdio(t *testing.T) {
	path, err := filepath.Abs("data-mcp/build/install/healbe-data-mcp/bin/healbe-data-mcp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("build data-mcp installDist first")
	}
	t.Setenv("HEALBE_LOGIN", "")
	t.Setenv("HEALBE_PASSWORD", "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx, close, err := connectDataMCP(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	d := ctx.Value(dataMCPKey{}).(*dataConnection)
	if len(d.definitions) != 25 {
		t.Fatalf("got %d tools", len(d.definitions))
	}
	raw := map[string]json.RawMessage{"data": json.RawMessage(`{"weight":-1,"measurementTime":1700000000}`)}
	_, err = callDataMCP(ctx, "healbe_add_weight", raw)
	if err == nil || !strings.Contains(err.Error(), "Invalid input") {
		t.Fatalf("expected validation error, got %v", err)
	}
	_, err = callDataMCP(ctx, "healbe_registration", raw)
	if err == nil {
		t.Fatal("unknown tool was allowed")
	}
}

func TestDataMCPCredentialsRedactedFromToolRecord(t *testing.T) {
	got := redactToolArguments(`{"login":"fixture","password":"super-secret","data":{"weight":80}}`)
	if strings.Contains(got, "super-secret") || !strings.Contains(got, "redacted") || !strings.Contains(got, "weight") {
		t.Fatal(got)
	}
}

func TestSupportNeverExposesDataMCP(t *testing.T) {
	for _, d := range supportTools() {
		name := d["function"].(map[string]any)["name"].(string)
		if strings.HasPrefix(name, "healbe_") {
			t.Fatal(name)
		}
	}
	i := &SupportInvestigator{}
	_, err := i.execute(context.Background(), SupportChannel{}, "", &Operation{}, testCall("x", "healbe_add_weight", map[string]any{}))
	if err == nil {
		t.Fatal("support accepted mutation")
	}
}

func TestRuntimeInventoryUsesConnectionNotConfiguration(t *testing.T) {
	c := Config{GitMCP: "/configured/git", DataMCP: "/configured/data"}
	plain := runtimeToolInventory(context.Background(), c)
	if !strings.Contains(plain, "not connected in this operation") || strings.Contains(plain, "healbe_add_weight") {
		t.Fatal(plain)
	}
	ctx := context.WithValue(context.Background(), dataMCPKey{}, &dataConnection{names: map[string]string{"healbe_download_weights": "download_weights", "healbe_add_weight": "add_weight"}})
	inventory := runtimeToolInventory(ctx, c)
	if !strings.Contains(inventory, "connected; discovered 2 tools") || !strings.Contains(inventory, "healbe_add_weight\nhealbe_download_weights") {
		t.Fatal(inventory)
	}
	e := &Engine{Config: c}
	op := &Operation{}
	_, err := e.executeTool(ctx, nil, &Session{}, op, testCall("inventory", "list_available_tools", map[string]any{}), func() error { return nil })
	if err != nil || len(op.Evidence) != 1 || op.Evidence[0].Text != inventory {
		t.Fatalf("inventory evidence: %v %+v", err, op.Evidence)
	}
}

func TestBundledDataDiscovery(t *testing.T) {
	root := t.TempDir()
	if got := bundledDataMCP(root); got != "" {
		t.Fatal(got)
	}
	path := filepath.Join(root, "data-mcp/build/package/healbe-data-mcp.app/Contents/MacOS/healbe-data-mcp")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := bundledDataMCP(root); got != "" {
		t.Fatal("non-executable selected")
	}
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	if got := bundledDataMCP(root); got != path {
		t.Fatal(got)
	}
}

func TestPrivateDataCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if got, err := readDataCredentials(path); got != nil || err != nil {
		t.Fatal(got, err)
	}
	if err := os.WriteFile(path, []byte(`{"login":"fixture","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readDataCredentials(path)
	if err != nil || got.Login != "fixture" || got.Password != "secret" {
		t.Fatal("credentials not loaded")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDataCredentials(path); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("unprotected credentials accepted or exposed")
	}
}
