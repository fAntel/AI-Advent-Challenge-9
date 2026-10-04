package main

import (
	"advent-agent/minichat"
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"--help"}, strings.NewReader(""), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"resume SESSION_ID", "/retry", "/constraint KEY=VALUE"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing help", want)
		}
	}
}
func TestChatLocalCommands(t *testing.T) {
	store, err := minichat.Open(filepath.Join(t.TempDir(), "chat.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, err := store.Create(minichat.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	e := &minichat.Engine{Store: store}
	input := "/goal Build a browser library\n/constraint platform=JS\n/definition client=browser\n/memory\n/forget constraint platform\n/history\n/no-such-command\n/exit\n"
	var out, errOut bytes.Buffer
	if err := chatLoop(context.Background(), e, s.ID, strings.NewReader(input), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Task memory updated:", "Build a browser library", "browser", "questions"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing display", want)
		}
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatal("missing command error")
	}
	got, _ := store.Load(s.ID)
	if len(got.Turns) != 0 || len(got.Memory.Constraints) != 0 {
		t.Fatal("local commands created a turn or failed to forget")
	}
}

func TestRetryWithoutPendingDoesNotRepublish(t *testing.T) {
	store, err := minichat.Open(filepath.Join(t.TempDir(), "chat.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, err := store.Create(minichat.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	s.Turns = []*minichat.Turn{{State: "completed", Question: "q", Answer: "OLD ANSWER"}}
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := chatLoop(context.Background(), &minichat.Engine{Store: store}, s.ID, strings.NewReader("/retry\n/exit\n"), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "OLD ANSWER") || !strings.Contains(errOut.String(), "no pending turn") {
		t.Fatal(out.String(), errOut.String())
	}
}
