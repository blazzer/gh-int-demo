package obs

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}
type githubTokenKey struct{}

// RequestIDMiddleware adds or propagates a request ID and structured logger.
func RequestIDMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, requestID)

		reqLogger := logger.With("request_id", requestID)
		ctx := WithLogger(r.Context(), reqLogger)
		ctx = context.WithValue(ctx, requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TokenMiddleware extracts a Bearer token and stores it in the request context.
func TokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			ctx = context.WithValue(ctx, githubTokenKey{}, strings.TrimPrefix(auth, "Bearer "))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithGitHubToken stores a GitHub token in context.
func WithGitHubToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, githubTokenKey{}, token)
}

// GitHubTokenFromContext returns the GitHub token from context.
func GitHubTokenFromContext(ctx context.Context) string {
	if token, ok := ctx.Value(githubTokenKey{}).(string); ok {
		return token
	}
	return ""
}
