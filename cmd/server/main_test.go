package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/obs"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	version = "test-version"
	commit = "abc123"

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(obs.NewLogger()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q", body["status"])
	}
	if body["commit"] != "abc123" {
		t.Fatalf("commit = %q", body["commit"])
	}
}

func TestHealthHandler_RequestID(t *testing.T) {
	t.Parallel()

	handler := obs.RequestIDMiddleware(obs.NewLogger(), healthHandler(obs.NewLogger()))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(obs.RequestIDHeader, "incoming-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(obs.RequestIDHeader); got != "incoming-id" {
		t.Fatalf("X-Request-ID = %q, want incoming-id", got)
	}
}
