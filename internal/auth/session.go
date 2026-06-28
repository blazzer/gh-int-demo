package auth

import "context"

type sessionKey struct{}

// Source identifies how the GitHub token was obtained for a request.
type Source string

const (
	SourceBearer        Source = "bearer"
	SourceServerDefault Source = "server_default"
)

// Session binds request identity for MCP tool calls.
type Session struct {
	RequestID   string
	GitHubToken string
	Source      Source
}

// WithSession stores a session in context.
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// SessionFromContext returns the session from context.
func SessionFromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	return s, ok
}
