package mcptools

import (
	"context"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/obs"
)

type stubLister struct{}

func (stubLister) ListRepositories(context.Context) ([]github.Repository, error) {
	return nil, nil
}

func TestResolveLister_BearerFromSession(t *testing.T) {
	t.Parallel()

	factoryCalled := false
	factory := func(token string) github.Lister {
		factoryCalled = true
		if token != "session-token" {
			t.Fatalf("token = %q, want session-token", token)
		}
		return stubLister{}
	}

	ctx := auth.WithSession(context.Background(), auth.Session{
		GitHubToken: "session-token",
		Source:      auth.SourceBearer,
	})

	lister, source := resolveLister(ctx, stubLister{}, factory)
	if lister == nil {
		t.Fatal("expected lister")
	}
	if !factoryCalled {
		t.Fatal("expected factory to be called")
	}
	if source != auth.SourceBearer {
		t.Fatalf("source = %q, want bearer", source)
	}
}

func TestResolveLister_ServerDefault(t *testing.T) {
	t.Parallel()

	factoryCalled := false
	factory := func(token string) github.Lister {
		factoryCalled = true
		return stubLister{}
	}

	defaultLister := stubLister{}
	lister, source := resolveLister(context.Background(), defaultLister, factory)
	if lister == nil {
		t.Fatal("expected lister")
	}
	if factoryCalled {
		t.Fatal("factory should not be called for server default")
	}
	if source != auth.SourceServerDefault {
		t.Fatalf("source = %q, want server_default", source)
	}
}

func TestResolveLister_BearerFromObsContext(t *testing.T) {
	t.Parallel()

	factoryCalled := false
	factory := func(token string) github.Lister {
		factoryCalled = true
		if token != "obs-token" {
			t.Fatalf("token = %q, want obs-token", token)
		}
		return stubLister{}
	}

	ctx := obs.WithGitHubToken(context.Background(), "obs-token")
	lister, source := resolveLister(ctx, stubLister{}, factory)
	if lister == nil {
		t.Fatal("expected lister")
	}
	if !factoryCalled {
		t.Fatal("expected factory to be called")
	}
	if source != auth.SourceBearer {
		t.Fatalf("source = %q, want bearer", source)
	}
}

func TestResolveLister_Missing(t *testing.T) {
	t.Parallel()

	lister, source := resolveLister(context.Background(), nil, github.DefaultClientFactory())
	if lister != nil || source != "" {
		t.Fatalf("resolveLister() = (%v, %q), want (nil, \"\")", lister, source)
	}
}
