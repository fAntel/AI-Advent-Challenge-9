package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShutdownEndpointAcknowledgesAndSignals(t *testing.T) {
	called := false
	handler := Server{Shutdown: func() { called = true }}.Handler()
	request := httptest.NewRequest(http.MethodPost, "/v1/shutdown", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d", response.Code)
	}
	if !called {
		t.Fatal("shutdown callback was not called")
	}
}

func TestTaskTransitionEndpointsRunDaemonValidatedPhases(t *testing.T) {
	cfg := testConfig(t)
	a, _ := NewAgent(cfg, &fakeCompleter{answers: []string{"plan", "execution", "revised plan"}}, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	session, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(session.ID)
	_ = a.Submit(session.ID, lease, MessageRequest{Content: "objective"})
	waitState(t, a, session.ID, "completed")
	handler := Server{Agent: a}.Handler()

	call := func(path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("X-Agent-Lease", lease)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	continued := call("/v1/sessions/"+session.ID+"/continue", "")
	if continued.Code != http.StatusAccepted {
		t.Fatalf("continue status=%d body=%s", continued.Code, continued.Body.String())
	}
	if got := waitState(t, a, session.ID, "completed"); got.Task.Phase != TaskPhaseExecution {
		t.Fatalf("continued task=%+v", got.Task)
	}
	back := call("/v1/sessions/"+session.ID+"/back", `{"phase":"planning"}`)
	if back.Code != http.StatusAccepted {
		t.Fatalf("back status=%d body=%s", back.Code, back.Body.String())
	}
	if got := waitState(t, a, session.ID, "completed"); got.Task.Phase != TaskPhasePlanning {
		t.Fatalf("back task=%+v", got.Task)
	}
	invalid := call("/v1/sessions/"+session.ID+"/back", `{"phase":"validation"}`)
	if invalid.Code != http.StatusBadRequest {
		var failure ErrorResponse
		_ = json.Unmarshal(invalid.Body.Bytes(), &failure)
		t.Fatalf("invalid status=%d error=%q", invalid.Code, failure.Error)
	}
}

func TestShutdownEndpointRejectsGet(t *testing.T) {
	response := httptest.NewRecorder()
	Server{}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/shutdown", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestDecodeJSONAllowsMultiMegabytePrompt(t *testing.T) {
	content := strings.Repeat("x", 3<<20)
	request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"system_prompt":"`+content+`"}`))
	response := httptest.NewRecorder()
	var decoded CreateSessionRequest
	if err := decodeJSON(response, request, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SystemPrompt != content {
		t.Fatalf("decoded prompt length=%d want=%d", len(decoded.SystemPrompt), len(content))
	}
}

func TestDecodeJSONRejectsBodyOverEightMiB(t *testing.T) {
	content := strings.Repeat("x", maxRequestBodyBytes)
	request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"system_prompt":"`+content+`"}`))
	response := httptest.NewRecorder()
	var decoded CreateSessionRequest
	if err := decodeJSON(response, request, &decoded); err == nil {
		t.Fatal("expected oversized request to be rejected")
	}
}
