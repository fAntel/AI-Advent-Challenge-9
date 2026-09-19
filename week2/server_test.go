package agent

import (
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
