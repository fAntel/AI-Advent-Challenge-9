package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testCall(id, name string, args any) ToolCall {
	tc := ToolCall{ID: id, Type: "function"}
	tc.Function.Name = name
	b, _ := json.Marshal(args)
	tc.Function.Arguments = string(b)
	return tc
}
func testFinal(m []Message) string {
	var input struct {
		Evidence []Evidence `json:"evidence"`
	}
	json.Unmarshal([]byte(m[1].Content), &input)
	a := Answer{Status: "unknown", Quotes: []Quote{}, Clarification: "No supporting sources."}
	if len(input.Evidence) > 0 {
		ev := input.Evidence[len(input.Evidence)-1]
		a = Answer{Status: "answered", Answer: fmt.Sprintf("Verified [%d].", ev.Number), Quotes: []Quote{{ev.ID, ev.Text}}}
	}
	b, _ := json.Marshal(a)
	return string(b)
}
func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func newFS(t *testing.T) (*ProjectFS, string) {
	t.Helper()
	root, _ := ProjectRoot(t.TempDir())
	fs, err := OpenProjectFS(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.Close() })
	return fs, root
}
func saveFile(fs *ProjectFS, path, text, hash string, create bool) (Change, error) {
	return fs.Write(context.Background(), path, text, hash, create, func(Change) error { return nil })
}
func TestFileBoundaryAndLinks(t *testing.T) {
	fs, root := newFS(t)
	outside := t.TempDir()
	put(t, filepath.Join(outside, "sentinel"), "outside")
	put(t, filepath.Join(root, ".git/config"), "git")
	put(t, filepath.Join(root, "nested/.git/config"), "nested")
	put(t, filepath.Join(root, "src/file"), "before")
	for _, link := range []struct{ name, target string }{{"escape", outside}, {"alias", ".git"}, {"internal", "src/file"}, {"absolute", filepath.Join(root, "src/file")}, {"cycle", "cycle"}, {"dir", "src"}} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"../sentinel", filepath.Join(outside, "sentinel"), ".git/config", "nested/.git/config", "alias/config", "escape/sentinel", "cycle", "src/../../sentinel"} {
		if _, _, err := fs.Read(path); err == nil {
			t.Errorf("read accepted %s", path)
		}
		if _, err := saveFile(fs, path, "bad", hashText("before"), false); err == nil {
			t.Errorf("write accepted %s", path)
		}
	}
	for _, path := range []string{"internal", "absolute", "dir/file"} {
		text, _, err := fs.Read(path)
		if err != nil || text != "before" {
			t.Fatalf("internal link %s: %s %v", path, text, err)
		}
	}
	if _, err := saveFile(fs, "internal", "after", hashText("before"), false); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(filepath.Join(root, "internal")); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link replaced")
	}
	got, _ := os.ReadFile(filepath.Join(root, "src/file"))
	if string(got) != "after" {
		t.Fatal("target not updated")
	}
	if _, err := saveFile(fs, "src/file", "stale", hashText("before"), false); err == nil {
		t.Fatal("stale overwrite")
	}
	if _, err := saveFile(fs, "src/file", "clobber", "", true); err == nil {
		t.Fatal("create clobbered")
	}
	c, err := saveFile(fs, "new.md", "new\n", "", true)
	if err != nil || c.Status != "saved" || !strings.Contains(c.Diff, "/dev/null") {
		t.Fatalf("create: %+v %v", c, err)
	}
	if _, err := saveFile(fs, "empty.md", "", "", true); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{filepath.Join(outside, "sentinel"): "outside", filepath.Join(root, ".git/config"): "git"} {
		b, _ := os.ReadFile(path)
		if string(b) != want {
			t.Fatal("sentinel modified")
		}
	}
}
func TestStateSpecialFilesAndHardlinks(t *testing.T) {
	root, _ := ProjectRoot(t.TempDir())
	state := filepath.Join(root, "runtime")
	put(t, filepath.Join(state, "session.json"), "state")
	fs, err := OpenProjectFS(root, state)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	os.Symlink("runtime", filepath.Join(root, "state-alias"))
	for _, p := range []string{"runtime/session.json", "state-alias/session.json"} {
		if _, _, err := fs.Read(p); err == nil {
			t.Fatal("read state")
		}
		if _, err := saveFile(fs, p, "bad", hashText("state"), false); err == nil {
			t.Fatal("write state")
		}
	}
	outside := filepath.Join(t.TempDir(), "hardlink")
	put(t, outside, "original")
	os.Link(outside, filepath.Join(root, "linked"))
	if _, err := saveFile(fs, "linked", "new", hashText("original"), false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "original" {
		t.Fatal("modified external inode")
	}
	put(t, filepath.Join(root, "binary"), "\x00binary")
	put(t, filepath.Join(root, "huge"), strings.Repeat("x", maxFileBytes+1))
	for _, p := range []string{"binary", "huge", "runtime"} {
		if _, _, err := fs.Read(p); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
}
func TestSymlinkSwap(t *testing.T) {
	fs, root := newFS(t)
	outside := t.TempDir()
	put(t, filepath.Join(root, "safe/file"), "safe")
	put(t, filepath.Join(root, ".git/file"), "git")
	put(t, filepath.Join(outside, "file"), "outside")
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, target := range []string{"safe", ".git", outside} {
				temp := filepath.Join(root, "swap-next")
				os.Remove(temp)
				os.Symlink(target, temp)
				os.Rename(temp, filepath.Join(root, "swap"))
			}
		}
	}()
	for i := 0; i < 200; i++ {
		text, _, err := fs.Read("swap/file")
		if err == nil && text != "safe" {
			t.Errorf("escaped read: %s", text)
		}
		saveFile(fs, "swap/file", "safe", hashText("safe"), false)
	}
	close(stop)
	wg.Wait()
	for p, want := range map[string]string{filepath.Join(root, ".git/file"): "git", filepath.Join(outside, "file"): "outside"} {
		b, _ := os.ReadFile(p)
		if string(b) != want {
			t.Fatal("escaped write")
		}
	}
}
func TestProjectRootAndLegacy(t *testing.T) {
	parent := repo(t)
	child := filepath.Join(parent, "child")
	os.Mkdir(child, 0755)
	got, err := ProjectRoot(child)
	if err != nil || got != child {
		t.Fatalf("widened: %s %v", got, err)
	}
	e := fakeEngine(t)
	s, err := e.Create(child)
	if err != nil {
		t.Fatal(err)
	}
	s.Version = 0
	e.save(s)
	if _, err = e.Ask(context.Background(), s.ID, "edit", false); err == nil {
		t.Fatal("legacy upgraded")
	}
	if _, err = e.Get(s.ID); err != nil {
		t.Fatal("legacy unreadable")
	}
}
func TestToolProviderWire(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if len(body["tools"].([]any)) != 6 {
			t.Error("tool definitions missing")
		}
		messages := body["messages"].([]any)
		last := messages[len(messages)-1].(map[string]any)
		if last["tool_call_id"] != "previous" {
			t.Error("lost tool id")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"next","type":"function","function":{"name":"list_files","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"total_tokens":9}}`)
	}))
	defer server.Close()
	out, call, err := ToolProvider(Config{Endpoint: server.URL, TimeoutSeconds: 5})(context.Background(), []ToolMessage{{Role: "tool", ToolCallID: "previous", Content: "{}"}}, true)
	if err != nil || len(out.ToolCalls) != 1 || call.Usage.Total != 9 {
		t.Fatalf("wire: %+v %+v %v", out, call, err)
	}
}

func TestGoalDrivenScenariosAndRecovery(t *testing.T) {
	e := fakeEngine(t)
	root, _ := ProjectRoot(t.TempDir())
	for _, name := range []string{"catalog.go", "service.go", "README.md"} {
		b, err := os.ReadFile(filepath.Join("testdata/task3", name))
		if err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(root, name), string(b))
	}
	s, err := e.Create(root)
	if err != nil {
		t.Fatal(err)
	}
	e.ToolComplete = func(_ context.Context, m []ToolMessage, _ bool) (ToolMessage, Call, error) {
		if len(m) == 2 {
			return ToolMessage{Role: "assistant", ToolCalls: []ToolCall{testCall("search", "search_files", map[string]any{"query": "Lookup", "glob": "*.go"})}}, Call{}, nil
		}
		if !strings.Contains(m[len(m)-1].Content, "service.go:3") || !strings.Contains(m[len(m)-1].Content, "catalog.go:4") {
			t.Fatal("usage locations missing")
		}
		return ToolMessage{Role: "assistant", Content: "Found definition and two calls."}, Call{}, nil
	}
	s, err = e.Ask(context.Background(), s.ID, "Find all uses of the catalog API.", false)
	if err != nil {
		t.Fatal(err)
	}
	newDocs := "# Catalog\n\nLookup(42) returns Answer. Unknown IDs return unknown.\nFeatured calls Lookup(42); Missing calls Lookup(-1).\n"
	wrote := false
	e.ToolComplete = func(_ context.Context, m []ToolMessage, _ bool) (ToolMessage, Call, error) {
		if len(m) == 2 {
			return ToolMessage{Role: "assistant", ToolCalls: []ToolCall{testCall("src", "read_file", map[string]any{"path": "catalog.go"}), testCall("calls", "read_file", map[string]any{"path": "service.go"}), testCall("doc", "read_file", map[string]any{"path": "README.md"})}}, Call{}, nil
		}
		if !wrote {
			var out struct {
				Hash string `json:"hash"`
			}
			json.Unmarshal([]byte(m[len(m)-1].Content), &out)
			if out.Hash == "" {
				t.Fatal("missing read hash")
			}
			wrote = true
			return ToolMessage{Role: "assistant", ToolCalls: []ToolCall{testCall("write", "write_file", map[string]any{"path": "README.md", "content": newDocs, "expected_hash": out.Hash})}}, Call{}, nil
		}
		return ToolMessage{}, Call{}, fmt.Errorf("simulated provider failure after write")
	}
	s, err = e.Ask(context.Background(), s.ID, "Update documentation to match the implementation.", false)
	if err == nil || len(s.Operations[1].Changes) != 1 || s.Operations[1].Changes[0].Status != "saved" {
		t.Fatalf("partial result lost: %+v %v", s, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if string(b) != newDocs {
		t.Fatal("edit not saved")
	}
	loaded, _ := e.Get(s.ID)
	if loaded.Operations[1].Changes[0].Diff == "" {
		t.Fatal("diff not persisted")
	}
	e.ToolComplete = func(_ context.Context, m []ToolMessage, _ bool) (ToolMessage, Call, error) {
		if len(m) == 2 {
			if !strings.Contains(m[1].Content, "previous_attempt") || !strings.Contains(m[1].Content, "saved") {
				t.Fatal("retry lacks outcomes")
			}
			return ToolMessage{Role: "assistant", ToolCalls: []ToolCall{testCall("verify", "read_file", map[string]any{"path": "README.md"})}}, Call{}, nil
		}
		return ToolMessage{Role: "assistant", Content: "Documentation matches implementation."}, Call{}, nil
	}
	s, err = e.Ask(context.Background(), s.ID, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Operations[2].Changes) != 0 {
		t.Fatal("replayed write")
	}
	// A separate session cannot read a sibling project's files even through an alias.
	other, _ := ProjectRoot(t.TempDir())
	os.Symlink(root, filepath.Join(other, "sibling"))
	otherFS, err := OpenProjectFS(other, e.Config.State)
	if err != nil {
		t.Fatal(err)
	}
	defer otherFS.Close()
	if _, _, err = otherFS.Read("sibling/README.md"); err == nil {
		t.Fatal("cross-session escape")
	}
}

func TestToolValidationLimitsAndCancellation(t *testing.T) {
	e := fakeEngine(t)
	fs, root := newFS(t)
	s := Session{Root: root}
	op := Operation{}
	for _, tc := range []ToolCall{testCall("a", "shell", map[string]any{}), testCall("b", "read_file", map[string]any{"path": "a", "extra": true}), testCall("c", "search_files", map[string]any{"query": ""}), testCall("d", "write_file", map[string]any{"path": "a"}), testCall("e", "read_file", map[string]any{"path": "a", "count": 999})} {
		if _, err := e.executeTool(context.Background(), fs, &s, &op, tc, func() error { return nil }); err == nil {
			t.Fatalf("accepted %s", tc.Function.Name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := fs.Write(ctx, "cancelled", "new", "", true, func(Change) error { cancel(); return nil })
	if err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err = os.Stat(filepath.Join(root, "cancelled")); !os.IsNotExist(err) {
		t.Fatal("wrote after cancellation")
	}
	_, err = fs.Write(context.Background(), "failure", "new", "", true, func(Change) error { return fmt.Errorf("disk full") })
	if err == nil {
		t.Fatal("ignored journal failure")
	}
	if _, err = os.Stat(filepath.Join(root, "failure")); !os.IsNotExist(err) {
		t.Fatal("wrote without durable intent")
	}
	for _, mode := range []string{"turns", "calls", "duplicates"} {
		t.Run(mode, func(t *testing.T) {
			engine := fakeEngine(t)
			session, _ := engine.Create(root)
			n := 0
			engine.ToolComplete = func(context.Context, []ToolMessage, bool) (ToolMessage, Call, error) {
				n++
				calls := []ToolCall{testCall(fmt.Sprint(n), "list_files", map[string]any{})}
				if mode == "calls" {
					for j := 0; j < 60; j++ {
						calls = append(calls, testCall(fmt.Sprintf("extra%d", j), "list_files", map[string]any{}))
					}
				}
				if mode == "duplicates" {
					calls = append(calls, calls[0])
				}
				return ToolMessage{Role: "assistant", ToolCalls: calls}, Call{}, nil
			}
			result, err := engine.Ask(context.Background(), session.ID, "discover", false)
			if err == nil {
				t.Fatal("limit ignored")
			}
			if mode != "turns" && len(result.Operations[0].Tools) != 0 {
				t.Fatal("invalid batch partially executed")
			}
		})
	}
}

func TestPaginationAndScopedIndex(t *testing.T) {
	e := fakeEngine(t)
	fs, root := newFS(t)
	op := Operation{}
	s := Session{Root: root}
	for i := 0; i < 105; i++ {
		put(t, filepath.Join(root, fmt.Sprintf("file%03d.txt", i)), "needle\n")
	}
	first, err := e.executeTool(context.Background(), fs, &s, &op, testCall("1", "search_files", map[string]any{"query": "needle"}), func() error { return nil })
	if err != nil || !strings.Contains(first, `"has_more":true`) {
		t.Fatalf("page: %s %v", first, err)
	}
	second, err := e.executeTool(context.Background(), fs, &s, &op, testCall("2", "search_files", map[string]any{"query": "needle", "offset": 100}), func() error { return nil })
	if err != nil || !strings.Contains(second, "file104.txt") || strings.Contains(second, "file099.txt") {
		t.Fatalf("page2: %s %v", second, err)
	}
	parent := repo(t)
	child := filepath.Join(parent, "child")
	put(t, filepath.Join(child, "README.md"), "inside")
	put(t, filepath.Join(child, ".gitignore"), "ignored.md\n")
	put(t, filepath.Join(child, "ignored.md"), "ignored")
	scoped, err := OpenProjectFS(child, e.Config.State)
	if err != nil {
		t.Fatal(err)
	}
	defer scoped.Close()
	snapshot, err := indexCorpus(context.Background(), scoped)
	if err != nil || strings.Contains(string(snapshot), "A project") || strings.Contains(string(snapshot), "ignored.md") || !strings.Contains(string(snapshot), "inside") {
		t.Fatalf("scoped index: %s %v", snapshot, err)
	}
}

func TestRuntimeHelperAndFIFO(t *testing.T) {
	root, _ := ProjectRoot(t.TempDir())
	helper := filepath.Join(root, "index.py")
	put(t, helper, "trusted")
	fs, err := OpenProjectFS(root, t.TempDir(), helper)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	os.Symlink("index.py", filepath.Join(root, "alias.py"))
	for _, path := range []string{"index.py", "alias.py"} {
		if _, err = saveFile(fs, path, "malicious", hashText("trusted"), false); err == nil {
			t.Fatal("runtime helper overwritten")
		}
	}
	if err = unix.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = fs.Read("fifo"); err == nil {
		t.Fatal("FIFO accepted")
	}
	if _, err = saveFile(fs, "fifo", "x", hashText(""), false); err == nil {
		t.Fatal("FIFO replaced")
	}
}
