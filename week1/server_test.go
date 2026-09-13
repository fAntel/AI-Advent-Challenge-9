package agent

import (
	"net/http"
	"net/http/httptest"
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
