package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/obs"
)

func TestMiddleware_RequireBearer(t *testing.T) {
	t.Setenv("AUTH_MODE", auth.ModeRequireBearer)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	stack := obs.RequestIDMiddleware(obs.NewLogger(), obs.TokenMiddleware(auth.Middleware(inner)))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	stack.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_BearerSession(t *testing.T) {
	t.Setenv("AUTH_MODE", auth.ModePermissive)

	var got auth.Session
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		got = session
		w.WriteHeader(http.StatusOK)
	})
	stack := obs.RequestIDMiddleware(obs.NewLogger(), obs.TokenMiddleware(auth.Middleware(inner)))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	rec := httptest.NewRecorder()
	stack.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got.Source != auth.SourceBearer || got.GitHubToken != "user-token" {
		t.Fatalf("session = %+v, want bearer user-token", got)
	}
	if got.RequestID == "" {
		t.Fatal("expected request ID in session")
	}
}
