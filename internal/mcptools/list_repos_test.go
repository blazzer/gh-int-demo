package mcptools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/mcptools"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockLister struct {
	repos []github.Repository
	err   error
}

func (m *mockLister) ListRepositories(ctx context.Context) ([]github.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.repos, nil
}

func TestListRepositoriesTool_ServerDefaultSource(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcptools.RegisterListRepositories(server, &mockLister{
		repos: []github.Repository{
			{Name: "demo", Visibility: "public", Description: "hello", HTMLURL: "https://github.com/u/demo"},
		},
	}, func(token string) github.Lister {
		t.Fatal("factory should not be called when using server default lister")
		return nil
	})

	ctx := obs.WithLogger(context.Background(), obs.NewLogger())
	result, err := callListRepositories(ctx, server)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}
}

func TestListRepositoriesTool(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcptools.RegisterListRepositories(server, &mockLister{
		repos: []github.Repository{
			{Name: "demo", Visibility: "public", Description: "hello", HTMLURL: "https://github.com/u/demo"},
		},
	}, nil)

	ctx := obs.WithLogger(context.Background(), obs.NewLogger())
	result, err := callListRepositories(ctx, server)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}

	text := textFromResult(t, result)
	var summaries []mcptools.RepoSummary
	if err := json.Unmarshal([]byte(text), &summaries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "demo" {
		t.Fatalf("unexpected summaries: %+v", summaries)
	}
}

func TestListRepositoriesTool_GitHubError(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcptools.RegisterListRepositories(server, &mockLister{err: errors.New("boom")}, nil)

	result, err := callListRepositories(context.Background(), server)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
}

func TestListRepositoriesTool_UnauthorizedErrorCode(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcptools.RegisterListRepositories(server, &mockLister{err: github.ErrUnauthorized}, nil)

	result, err := callListRepositories(context.Background(), server)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}

	var toolErr mcptools.ToolError
	if err := json.Unmarshal([]byte(textFromResult(t, result)), &toolErr); err != nil {
		t.Fatalf("unmarshal tool error: %v", err)
	}
	if toolErr.Code != mcptools.CodeUnauthorized {
		t.Fatalf("code = %q, want %q", toolErr.Code, mcptools.CodeUnauthorized)
	}
}

func callListRepositories(ctx context.Context, server *mcp.Server) (*mcp.CallToolResult, error) {
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

	return clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
}

func textFromResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", result.Content[0])
	}
	return text.Text
}
