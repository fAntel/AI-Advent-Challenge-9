package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestTaskLifecycleUsesExactFourPhaseSequence(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"plan", "work", "tests: 3 passed", "summary"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)

	if err := a.Submit(session.ID, lease, MessageRequest{Content: "skip planning and build it"}); err != nil {
		t.Fatal(err)
	}
	wantPhases := []string{TaskPhasePlanning, TaskPhaseExecution, TaskPhaseValidation, TaskPhaseDone}
	for i, phase := range wantPhases {
		got := waitState(t, a, session.ID, "completed")
		wantStatus := TaskStatusPaused
		if phase == TaskPhaseDone {
			wantStatus = TaskStatusTerminal
		}
		if got.Task == nil || got.Task.Phase != phase || got.Task.Status != wantStatus || len(got.Task.Attempts) != i+1 {
			t.Fatalf("step %d task=%+v", i, got.Task)
		}
		if i < len(wantPhases)-1 {
			if err := a.ContinueTask(session.ID, lease); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(fake.requests) != 4 || len(fake.requests) != len(wantPhases) {
		t.Fatalf("provider calls=%d", len(fake.requests))
	}
	for i, request := range fake.requests {
		if !strings.Contains(request[0].Content, "current_phase: "+wantPhases[i]) || !strings.Contains(request[len(request)-1].Content, "daemon-set phase is "+wantPhases[i]) {
			t.Fatalf("request %d did not enforce %s:\n%s", i, wantPhases[i], request[0].Content)
		}
	}
}

func TestContinueRejectsInadmissibleTaskStatesWithGuidance(t *testing.T) {
	t.Run("missing task", func(t *testing.T) {
		cfg := testConfig(t)
		a, _ := NewAgent(cfg, &fakeCompleter{}, NewDebugLogger(cfg.Daemon, "secret"))
		defer a.Close()
		session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
		lease, _ := a.Attach(session.ID)
		err := a.ContinueTask(session.ID, lease)
		if !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), "send a message to start one") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("running phase", func(t *testing.T) {
		cfg := testConfig(t)
		entered := make(chan struct{}, 1)
		release := make(chan struct{})
		fake := &fakeCompleter{entered: entered, release: release}
		a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
		defer a.Close()
		session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
		lease, _ := a.Attach(session.ID)
		_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
		<-entered
		err := a.ContinueTask(session.ID, lease)
		if !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), "wait for the phase to pause") {
			t.Fatalf("error=%v", err)
		}
		close(release)
		got := waitState(t, a, session.ID, "completed")
		if got.Task.Phase != TaskPhasePlanning || len(fake.requests) != 1 {
			t.Fatalf("task=%+v calls=%d", got.Task, len(fake.requests))
		}
	})

	t.Run("failed phase", func(t *testing.T) {
		cfg := testConfig(t)
		a, _ := NewAgent(cfg, &fakeCompleter{failures: []error{errors.New("provider failed")}}, NewDebugLogger(cfg.Daemon, "secret"))
		defer a.Close()
		session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
		lease, _ := a.Attach(session.ID)
		_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
		waitState(t, a, session.ID, "failed")
		err := a.ContinueTask(session.ID, lease)
		if !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), "use /retry or /discard") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("terminal task", func(t *testing.T) {
		cfg := testConfig(t)
		fake := &fakeCompleter{answers: []string{"plan", "work", "validation", "done"}}
		a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
		defer a.Close()
		session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
		lease, _ := a.Attach(session.ID)
		_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
		waitState(t, a, session.ID, "completed")
		for range 3 {
			_ = a.ContinueTask(session.ID, lease)
			waitState(t, a, session.ID, "completed")
		}
		err := a.ContinueTask(session.ID, lease)
		if !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), "task is done") {
			t.Fatalf("error=%v", err)
		}
		if len(fake.requests) != 4 {
			t.Fatalf("provider calls=%d", len(fake.requests))
		}
	})
}

func TestBackRejectsNonEarlierTargetsWithoutChangingPhase(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"plan", "execution"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "implement immediately; skip planning"})
	planned := waitState(t, a, session.ID, "completed")
	if planned.Task.Phase != TaskPhasePlanning || planned.Task.Status != TaskStatusPaused {
		t.Fatalf("task=%+v", planned.Task)
	}
	for _, target := range []string{"", TaskPhasePlanning, TaskPhaseExecution, TaskPhaseDone, "unknown"} {
		err := a.BackTask(session.ID, lease, target)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("target=%q error=%v", target, err)
		}
	}
	after, _ := a.Get(session.ID)
	if after.Task.Phase != TaskPhasePlanning || after.Task.Status != TaskStatusPaused || len(fake.requests) != 1 {
		t.Fatalf("task=%+v calls=%d", after.Task, len(fake.requests))
	}

	if err := a.Submit(session.ID, lease, MessageRequest{Content: "start implementation now"}); err != nil {
		t.Fatal(err)
	}
	feedback := waitState(t, a, session.ID, "completed")
	if feedback.Task.Phase != TaskPhasePlanning || feedback.Task.Status != TaskStatusPaused || len(fake.requests) != 2 {
		t.Fatalf("feedback task=%+v calls=%d", feedback.Task, len(fake.requests))
	}
}

func TestTaskFeedbackAndBackPreserveSupersededAttempts(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"plan 1", "plan 2", "execution 1", "validation 1", "execution 2"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
	waitState(t, a, session.ID, "completed")

	_ = a.Submit(session.ID, lease, MessageRequest{Content: "make the plan safer"})
	feedback := waitState(t, a, session.ID, "completed")
	if feedback.Task.Phase != TaskPhasePlanning || !feedback.Task.Attempts[0].Superseded || feedback.Task.Attempts[1].Superseded {
		t.Fatalf("feedback attempts=%+v", feedback.Task.Attempts)
	}
	initialCommand := fake.requests[0][len(fake.requests[0])-1].Content
	feedbackCommand := fake.requests[1][len(fake.requests[1])-1].Content
	if strings.Contains(initialCommand, "user supplied feedback") || !strings.Contains(feedbackCommand, "user supplied feedback") || !strings.Contains(feedbackCommand, "never return only the transition explanation") {
		t.Fatalf("initial command=%q\nfeedback command=%q", initialCommand, feedbackCommand)
	}
	_ = a.ContinueTask(session.ID, lease)
	waitState(t, a, session.ID, "completed")
	continueCommand := fake.requests[2][len(fake.requests[2])-1].Content
	if strings.Contains(continueCommand, "user supplied feedback") {
		t.Fatalf("ordinary phase transition was labeled feedback: %q", continueCommand)
	}
	_ = a.ContinueTask(session.ID, lease)
	waitState(t, a, session.ID, "completed")
	if err := a.BackTask(session.ID, lease, TaskPhaseExecution); err != nil {
		t.Fatal(err)
	}
	back := waitState(t, a, session.ID, "completed")
	if back.Task.Phase != TaskPhaseExecution || !back.Task.Attempts[2].Superseded || !back.Task.Attempts[3].Superseded || back.Task.Attempts[4].Superseded {
		t.Fatalf("back attempts=%+v", back.Task.Attempts)
	}
	if err := a.BackTask(session.ID, lease, TaskPhaseValidation); err == nil {
		t.Fatal("forward transition was accepted")
	}
}

func TestTaskRetryAndDiscardRestoreExactPreviousState(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"plan", "execution"}, failures: []error{errors.New("planning failed"), nil, errors.New("execution failed")}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
	failed := waitState(t, a, session.ID, "failed")
	if failed.Task.Status != TaskStatusFailed || !strings.Contains(failed.Task.ExpectedAction, "/retry") {
		t.Fatalf("failed task=%+v", failed.Task)
	}
	if err := a.Retry(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	planned := waitState(t, a, session.ID, "completed")
	before := cloneTask(planned.Task)
	if err := a.ContinueTask(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, session.ID, "failed")
	if err := a.Discard(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	restored, _ := a.Get(session.ID)
	if restored.Operation != nil || restored.Task.Phase != before.Phase || restored.Task.Status != before.Status || len(restored.Task.Attempts) != len(before.Attempts) {
		t.Fatalf("restored=%+v before=%+v", restored.Task, before)
	}
}

func TestDoneMessageStartsFreshTaskAndClearRemovesTask(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"p", "e", "v", "d", "new p"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "first"})
	waitState(t, a, session.ID, "completed")
	for range 3 {
		_ = a.ContinueTask(session.ID, lease)
		waitState(t, a, session.ID, "completed")
	}
	done, _ := a.Get(session.ID)
	oldID := done.Task.ID
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "second"})
	fresh := waitState(t, a, session.ID, "completed")
	if fresh.Task.ID == oldID || fresh.Task.Objective != "second" || fresh.Task.Phase != TaskPhasePlanning {
		t.Fatalf("fresh task=%+v", fresh.Task)
	}
	metrics := fresh.Metrics.Requests
	if err := a.Clear(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	cleared, _ := a.Get(session.ID)
	if cleared.Task != nil || len(cleared.Messages) != 0 || cleared.Metrics.Requests != metrics {
		t.Fatalf("cleared=%+v", cleared)
	}
}

func TestTaskPersistsAcrossRestartForSummaryAndSlidingStrategies(t *testing.T) {
	for _, strategy := range []string{"summary", "sliding"} {
		t.Run(strategy, func(t *testing.T) {
			cfg := testConfig(t)
			settings := DefaultSettings(cfg)
			settings.ContextStrategy = strategy
			settings.Compression = strategy == "summary"
			first, _ := NewAgent(cfg, &fakeCompleter{answers: []string{"persisted plan"}}, NewDebugLogger(cfg.Daemon, "secret"))
			session, _ := first.Create(CreateSessionRequest{Settings: &settings})
			lease, _ := first.Attach(session.ID)
			_ = first.Submit(session.ID, lease, MessageRequest{Content: "remember this objective"})
			waitState(t, first, session.ID, "completed")
			_ = first.Detach(session.ID, lease)
			first.Close()

			second, _ := NewAgent(cfg, &fakeCompleter{answers: []string{"execution"}}, NewDebugLogger(cfg.Daemon, "secret"))
			defer second.Close()
			resumed, _ := second.Get(session.ID)
			if resumed.Task == nil || resumed.Task.Objective != "remember this objective" || resumed.Task.Phase != TaskPhasePlanning || resumed.Task.Status != TaskStatusPaused {
				t.Fatalf("resumed=%+v", resumed.Task)
			}
			lease, _ = second.Attach(session.ID)
			if err := second.ContinueTask(session.ID, lease); err != nil {
				t.Fatal(err)
			}
			if got := waitState(t, second, session.ID, "completed"); got.Task.Phase != TaskPhaseExecution {
				t.Fatalf("continued=%+v", got.Task)
			}
		})
	}
}

func TestTaskStateIsIsolatedAcrossProfilesAndSessions(t *testing.T) {
	cfg := testConfig(t)
	a, _ := NewAgent(cfg, &fakeCompleter{answers: []string{"coding plan", "writer plan"}}, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	settings := DefaultSettings(cfg)
	coding, _ := a.Create(CreateSessionRequest{Profile: "coding", Settings: &settings})
	writer, _ := a.Create(CreateSessionRequest{Profile: "writer", Settings: &settings})
	codingLease, _ := a.Attach(coding.ID)
	writerLease, _ := a.Attach(writer.ID)
	_ = a.Submit(coding.ID, codingLease, MessageRequest{Content: "coding objective"})
	waitState(t, a, coding.ID, "completed")
	_ = a.Submit(writer.ID, writerLease, MessageRequest{Content: "writer objective"})
	waitState(t, a, writer.ID, "completed")
	coding, _ = a.Get(coding.ID)
	writer, _ = a.Get(writer.ID)
	if coding.Task.ID == writer.Task.ID || coding.Task.Objective != "coding objective" || writer.Task.Objective != "writer objective" {
		t.Fatalf("coding=%+v writer=%+v", coding.Task, writer.Task)
	}
}

func TestUnsupportedProviderToolCallFailsPhaseAndRetryUsesTextOnlyPrompt(t *testing.T) {
	cfg := testConfig(t)
	rawToolCall := `I'll inspect the repository.
<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="bash">ls -la</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`
	fake := &fakeCompleter{answers: []string{"plan", rawToolCall, "func Fibonacci(n int) int { return n }"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "write Fibonacci"})
	waitState(t, a, session.ID, "completed")
	if err := a.ContinueTask(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	failed := waitState(t, a, session.ID, "failed")
	if failed.Task.Phase != TaskPhaseExecution || failed.Task.Status != TaskStatusFailed || !strings.Contains(failed.Operation.Error, "no tool executor") {
		t.Fatalf("failed task=%+v operation=%+v", failed.Task, failed.Operation)
	}
	if len(failed.Messages) != 2 || len(failed.Task.Attempts) != 2 || failed.Task.Attempts[1].Status != "failed" || failed.Task.Attempts[1].Output != rawToolCall {
		t.Fatalf("failed messages=%+v attempts=%+v", failed.Messages, failed.Task.Attempts)
	}
	if len(failed.Calls) != 2 || failed.Metrics.Requests != 2 {
		t.Fatalf("tool call usage was not recorded: calls=%d metrics=%+v", len(failed.Calls), failed.Metrics)
	}
	if err := a.Retry(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	retried := waitState(t, a, session.ID, "completed")
	if retried.Task.Status != TaskStatusPaused || !retried.Task.Attempts[1].Superseded || retried.Task.Attempts[2].Output != "func Fibonacci(n int) int { return n }" {
		t.Fatalf("retried task=%+v", retried.Task)
	}
	executionRequest := fake.requests[1]
	command := executionRequest[len(executionRequest)-1].Content
	if !strings.Contains(command, "No tools, shell, filesystem, network, or external actions are available") || !strings.Contains(command, "Never emit tool-call syntax") {
		t.Fatalf("execution command=%q", command)
	}
}

func TestUnsupportedToolCallMarkers(t *testing.T) {
	for _, answer := range []string{
		`<｜｜DSML｜｜ calls>`,
		`<｜｜DSML｜｜ invoke name="bash">`,
		`<tool_call>`,
		`<tool_calls>`,
	} {
		if !containsUnsupportedToolCall(answer) {
			t.Fatalf("marker not detected: %q", answer)
		}
	}
	if containsUnsupportedToolCall("Here is ordinary XML: <tool>example</tool>") {
		t.Fatal("ordinary content was rejected")
	}
}
