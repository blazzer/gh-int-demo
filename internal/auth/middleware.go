package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/blazzer/gh-int-demo/internal/obs"
)

const (
	ModePermissive    = "permissive"
	ModeRequireBearer = "require_bearer"
)

// Mode returns the configured auth mode from AUTH_MODE (default permissive).
func Mode() string {
	if m := strings.TrimSpace(os.Getenv("AUTH_MODE")); m != "" {
		return m
	}
	return ModePermissive
}

// Middleware enforces AUTH_MODE and attaches an auth.Session to the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := obs.GitHubTokenFromContext(r.Context())
		session := Session{RequestID: obs.RequestIDFromContext(r.Context())}

		if token != "" {
			session.GitHubToken = token
			session.Source = SourceBearer
		} else if Mode() == ModeRequireBearer {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), session)))
	})
}
