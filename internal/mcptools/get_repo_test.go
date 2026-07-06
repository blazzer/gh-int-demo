package mcptools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/mcptools"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockRepoGetter struct {
	repo github.Repository
	err  error
}

func (m *mockRepoGetter) ListRepositories(context.Context) ([]github.Repository, error) {
	return nil, nil
}

func (m *mockRepoGetter) GetRepository(ctx context.Context, owner, name string) (github.Repository, error) {
	if m.err != nil {
		return github.Repository{}, m.err
	}
	return m.repo, nil
}

func TestGetRepositoryTool(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcptools.RegisterGetRepository(server, &mockRepoGetter{
		repo: github.Repository{
			Name:        "demo",
			Visibility:  "public",
			Description: "hello",
			HTMLURL:     "https://github.com/acme/demo",
		},
	}, nil)

	ctx := obs.WithLogger(context.Background(), obs.NewLogger())
	result, err := callGetRepository(ctx, server, "acme", "demo")
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}

	var summary mcptools.RepoSummary
	if err := json.Unmarshal([]byte(textFromResult(t, result)), &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if summary.Name != "demo" || summary.URL != "https://github.com/acme/demo" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func callGetRepository(ctx context.Context, server *mcp.Server, owner, name string) (*mcp.CallToolResult, error) {
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = serverSession.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = clientSession.Close() }()

	return clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_repository",
		Arguments: map[string]any{
			"owner": owner,
			"name":  name,
		},
	})
}
