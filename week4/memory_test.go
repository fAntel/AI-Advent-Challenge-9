package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestINIMemoryRoundTripOrderingPunctuationAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.ini")
	storage := INIMemoryStorage{}
	values := map[string]map[string]string{"solutions": {"z-key": "SQLite: WAL; temp DB (per test) #1", "a.key": "x = y, then z"}, "preferences": {"answer-style": "concise"}, "knowledge": {}}
	if err := storage.Save(path, values); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "[knowledge]\n\n[preferences]\nanswer-style = concise\n\n[solutions]\na.key = x = y, then z\nz-key = SQLite: WAL; temp DB (per test) #1\n"
	if string(data) != want {
		t.Fatalf("serialized:\n%s\nwant:\n%s", data, want)
	}
	got, err := storage.Load(path, "preferences", "solutions", "knowledge")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("round trip=%#v", got)
	}
	for _, key := range []string{"", "bad key", "[section]", "a=b", "UPPER SPACE"} {
		if _, err := normalizeMemoryKey(key); err == nil {
			t.Fatalf("key %q accepted", key)
		}
	}
	if key, err := normalizeMemoryKey("Mixed-KEY_1.x"); err != nil || key != "mixed-key_1.x" {
		t.Fatalf("normalized=%q err=%v", key, err)
	}
	for _, value := range []string{"", "  ", "two\nlines", "bad\x00value"} {
		if validateMemoryValue(value) == nil {
			t.Fatalf("value %q accepted", value)
		}
	}
}

func TestMalformedAndDuplicateINIIsNeverReplaced(t *testing.T) {
	root := t.TempDir()
	manager := MemoryManager{Root: root}
	path := manager.LongTermPath("coding")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := "[preferences]\na = one\na = two\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create("coding", "p", MemoryPreference, "b", "three"); err == nil {
		t.Fatal("duplicate file should be rejected")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatalf("malformed file replaced: %q", data)
	}
}

func TestMemoryCRUDMoveAndScopeIsolation(t *testing.T) {
	m := MemoryManager{Root: t.TempDir()}
	if err := m.Create("coding", "project-a", MemoryWorking, "Language", "Go"); err != nil {
		t.Fatal(err)
	}
	if err := m.Create("coding", "project-a", MemoryPreference, "answer-style", "concise"); err != nil {
		t.Fatal(err)
	}
	if err := m.Create("coding", "project-a", MemoryWorking, "language", "Rust"); err == nil {
		t.Fatal("duplicate create accepted")
	}
	if err := m.Edit("coding", "project-a", MemoryWorking, "language", "Go 1.27"); err != nil {
		t.Fatal(err)
	}
	if err := m.Move("coding", "project-a", MemoryWorking, "language", MemoryKnowledge); err != nil {
		t.Fatal(err)
	}
	view, err := m.View("coding", "project-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Working) != 0 || view.Knowledge["language"] != "Go 1.27" || view.Preferences["answer-style"] != "concise" {
		t.Fatalf("view=%+v", view)
	}
	otherProject, _ := m.View("coding", "project-b")
	if len(otherProject.Working) != 0 || otherProject.Knowledge["language"] != "Go 1.27" {
		t.Fatalf("other project=%+v", otherProject)
	}
	otherProfile, _ := m.View("writer", "project-a")
	if len(otherProfile.Working)+len(otherProfile.Knowledge)+len(otherProfile.Preferences) != 0 {
		t.Fatalf("other profile=%+v", otherProfile)
	}
	if err := m.Delete("coding", "project-a", MemoryKnowledge, "language"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryPromptOrderAndImmediateCRUD(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"one", "two"}}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := a.Create(CreateSessionRequest{Profile: "coding", ProjectID: "project", Instructions: []InstructionSource{{Path: "user", Content: "shared rules"}, {Path: "profile", Content: "coding rules"}}, Settings: ptrSettings(DefaultSettings(cfg))})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "create", Scope: MemoryPreference, Key: "language", Value: "Python"}); err != nil {
		t.Fatal(err)
	}
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "create", Scope: MemoryWorking, Key: "language", Value: "Go"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Use Rust for this answer"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "completed")
	prompt := fake.requests[0][0].Content
	indices := []int{strings.Index(prompt, "shared rules"), strings.Index(prompt, "coding rules"), strings.Index(prompt, "<harness-task>"), strings.Index(prompt, "[preferences]"), strings.Index(prompt, "[working]")}
	for i := 1; i < len(indices); i++ {
		if indices[i] <= indices[i-1] {
			t.Fatalf("prompt order %v:\n%s", indices, prompt)
		}
	}
	if fake.requests[0][1].Content != "Use Rust for this answer" {
		t.Fatalf("dialog not last: %+v", fake.requests[0])
	}
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "edit", Scope: MemoryWorking, Key: "language", Value: "Kotlin"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "again"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "completed")
	if !strings.Contains(fake.requests[1][0].Content, "language = Kotlin") || strings.Contains(fake.requests[1][0].Content, "language = Go\n") {
		t.Fatalf("updated prompt=%s", fake.requests[1][0].Content)
	}
}

func TestClearOnlyShortTermAndMutationRejectedWhileActive(t *testing.T) {
	cfg := testConfig(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	fake := &fakeCompleter{entered: entered, release: release}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Profile: "coding", ProjectID: "p", Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(s.ID)
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "create", Scope: MemoryWorking, Key: "language", Value: "Go"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "edit", Scope: MemoryWorking, Key: "language", Value: "Rust"}); err != ErrConflict {
		t.Fatalf("active mutation err=%v", err)
	}
	close(release)
	waitState(t, a, s.ID, "completed")
	if err := a.Clear(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Get(s.ID)
	if len(got.Messages) != 0 || got.Summary != "" || got.Task != nil || got.Metrics.Requests != 1 {
		t.Fatalf("cleared session=%+v", got)
	}
	view, _ := a.Memory(s.ID)
	if view.Working["language"] != "Go" {
		t.Fatalf("memory=%+v", view)
	}
}

func TestProfileSessionsIsolateAndBranchesShareExplicitMemory(t *testing.T) {
	cfg := testConfig(t)
	a, _ := NewAgent(cfg, &fakeCompleter{}, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.ContextStrategy = "branching"
	settings.Compression = false
	coding, _ := a.Create(CreateSessionRequest{Profile: "coding", ProjectID: "same-project", Settings: &settings})
	writer, _ := a.Create(CreateSessionRequest{Profile: "writer", ProjectID: "same-project", Settings: &settings})
	codingList, _ := a.ListProfile("coding")
	writerList, _ := a.ListProfile("writer")
	if len(codingList) != 1 || codingList[0].ID != coding.ID || len(writerList) != 1 || writerList[0].ID != writer.ID {
		t.Fatalf("coding=%+v writer=%+v", codingList, writerList)
	}
	lease, _ := a.Attach(coding.ID)
	if err := a.MutateMemory(coding.ID, lease, MemoryMutationRequest{Action: "create", Scope: MemoryWorking, Key: "database", Value: "SQLite"}); err != nil {
		t.Fatal(err)
	}
	cp, err := a.CreateCheckpoint(coding.ID, lease, "fork")
	if err != nil {
		t.Fatal(err)
	}
	branch, err := a.CreateBranch(coding.ID, lease, BranchRequest{Name: "alternative", Checkpoint: cp.ID})
	if err != nil {
		t.Fatal(err)
	}
	branchMemory, _ := a.Memory(branch.ID)
	if branch.Profile != "coding" || branch.ProjectID != "same-project" || branchMemory.Working["database"] != "SQLite" {
		t.Fatalf("branch=%+v memory=%+v", branch, branchMemory)
	}
	writerMemory, _ := a.Memory(writer.ID)
	if len(writerMemory.Working) != 0 {
		t.Fatalf("writer memory leaked: %+v", writerMemory)
	}
}
