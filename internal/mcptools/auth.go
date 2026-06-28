package mcptools

import (
	"context"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/obs"
)

func resolveLister(ctx context.Context, defaultClient github.Lister, factory github.ClientFactory) (github.Lister, auth.Source) {
	if session, ok := auth.SessionFromContext(ctx); ok && session.GitHubToken != "" && session.Source == auth.SourceBearer {
		return factory(session.GitHubToken), auth.SourceBearer
	}
	if token := obs.GitHubTokenFromContext(ctx); token != "" {
		return factory(token), auth.SourceBearer
	}
	if defaultClient != nil {
		return defaultClient, auth.SourceServerDefault
	}
	return nil, ""
}
