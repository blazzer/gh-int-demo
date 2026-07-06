package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpStackOpts struct {
	requireBearer bool
}

func newTestMCPStack(t *testing.T, opts mcpStackOpts) http.Handler {
	logger, _ := obs.NewBufferLogger()
	return newTestMCPStackWithLogger(t, logger, opts)
}

func newTestMCPStackWithLogger(t *testing.T, logger *slog.Logger, opts mcpStackOpts) http.Handler {
	t.Helper()

	if opts.requireBearer {
		t.Setenv("AUTH_MODE", auth.ModeRequireBearer)
	} else {
		t.Setenv("AUTH_MODE", auth.ModePermissive)
	}

	mcpServer := newMCPServer(logger)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	return obs.RequestIDMiddleware(logger, obs.TokenMiddleware(auth.Middleware(mcpHandler)))
}

func newTestMCPStackWithServer(t *testing.T, logger *slog.Logger, mcpServer *mcp.Server, opts mcpStackOpts) http.Handler {
	t.Helper()

	if opts.requireBearer {
		t.Setenv("AUTH_MODE", auth.ModeRequireBearer)
	} else {
		t.Setenv("AUTH_MODE", auth.ModePermissive)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	return obs.RequestIDMiddleware(logger, obs.TokenMiddleware(auth.Middleware(mcpHandler)))
}

func postMCPInitialize(t *testing.T, srv *httptest.Server, bearerToken string) *http.Response {
	t.Helper()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func connectMCPClient(t *testing.T, srvURL, bearerToken string) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)

	var httpClient *http.Client
	if bearerToken != "" {
		httpClient = &http.Client{
			Transport: testBearerRoundTripper{token: bearerToken},
		}
	}

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   srvURL,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type testBearerRoundTripper struct {
	token string
}

func (rt testBearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+rt.token)
	return http.DefaultTransport.RoundTrip(cloned)
}

func TestMCPInitializeHTTP(t *testing.T) {
	handler := newTestMCPStack(t, mcpStackOpts{requireBearer: false})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	resp := postMCPInitialize(t, srv, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMCP_RequireBearer(t *testing.T) {
	handler := newTestMCPStack(t, mcpStackOpts{requireBearer: true})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	resp := postMCPInitialize(t, srv, "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 401; body = %s", resp.StatusCode, body)
	}

	resp2 := postMCPInitialize(t, srv, "test-token")
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("status = %d, want 200; body = %s", resp2.StatusCode, body)
	}
}

func TestMCP_CallTool_AuthSourceBearer(t *testing.T) {
	t.Setenv("AUTH_MODE", auth.ModePermissive)

	repos := []github.Repository{
		{Name: "client-repo", Visibility: "public", HTMLURL: "https://github.com/u/client-repo"},
	}
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer client-token" {
			http.Error(w, "wrong token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(repos)
	}))
	t.Cleanup(ghSrv.Close)

	logger, logBuf := obs.NewBufferLogger()
	factory := func(token string) github.Lister {
		return github.NewClient(token).WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client())
	}
	defaultClient := github.NewClient("server-token").WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client())
	mcpServer := newMCPServerWithClients(logger, defaultClient, factory)

	handler := newTestMCPStackWithServer(t, logger, mcpServer, mcpStackOpts{requireBearer: false})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	session := connectMCPClient(t, srv.URL, "client-token")
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", toolTextContent(result))
	}
	if !strings.Contains(logBuf.String(), `"auth_source":"bearer"`) {
		t.Fatalf("logs missing auth_source bearer:\n%s", logBuf.String())
	}
}

func TestMCP_CallTool_AuthSourceServerDefault(t *testing.T) {
	t.Setenv("AUTH_MODE", auth.ModePermissive)

	repos := []github.Repository{
		{Name: "server-repo", Visibility: "public", HTMLURL: "https://github.com/u/server-repo"},
	}
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(repos)
	}))
	t.Cleanup(ghSrv.Close)

	logger, logBuf := obs.NewBufferLogger()
	defaultClient := github.NewClient("server-token").WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client())
	factory := func(token string) github.Lister {
		t.Fatal("factory should not be called when using server default lister")
		return nil
	}
	mcpServer := newMCPServerWithClients(logger, defaultClient, factory)

	handler := newTestMCPStackWithServer(t, logger, mcpServer, mcpStackOpts{requireBearer: false})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	session := connectMCPClient(t, srv.URL, "")
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", toolTextContent(result))
	}
	if !strings.Contains(logBuf.String(), `"auth_source":"server_default"`) {
		t.Fatalf("logs missing auth_source server_default:\n%s", logBuf.String())
	}
}

func toolTextContent(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}
