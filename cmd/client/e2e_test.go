package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/mcptools"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEndToEnd_ListRepositories(t *testing.T) {
	t.Parallel()

	repos := []github.Repository{
		{Name: "alpha", Visibility: "public", Description: "first repo", HTMLURL: "https://github.com/u/alpha"},
		{Name: "beta", Visibility: "private", Description: "second repo", HTMLURL: "https://github.com/u/beta"},
	}

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(repos)
	}))
	t.Cleanup(ghSrv.Close)

	logger := obs.NewLogger()
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "e2e", Version: "0.0.1"}, nil)
	mcptools.RegisterListRepositories(mcpServer, github.NewClient("test-token").WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client()), nil)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	handler := obs.RequestIDMiddleware(logger, obs.TokenMiddleware(auth.Middleware(mcpHandler)))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", textContent(result))
	}

	var out bytes.Buffer
	if err := printRepositories(&out, textContent(result)); err != nil {
		t.Fatalf("print: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "alpha") || !strings.Contains(output, "beta") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestEndToEnd_BearerOverridesDefault(t *testing.T) {
	t.Parallel()

	var authHeader atomic.Value
	repos := []github.Repository{
		{Name: "client-repo", Visibility: "public", HTMLURL: "https://github.com/u/client-repo"},
	}

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader.Store(r.Header.Get("Authorization"))
		if got := r.Header.Get("Authorization"); got != "Bearer client-token" {
			http.Error(w, "wrong token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(repos)
	}))
	t.Cleanup(ghSrv.Close)

	logger := obs.NewLogger()
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "e2e", Version: "0.0.1"}, nil)
	factory := func(token string) github.Lister {
		return github.NewClient(token).WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client())
	}
	mcptools.RegisterListRepositories(mcpServer, github.NewClient("server-token").WithBaseURL(ghSrv.URL).WithHTTPClient(ghSrv.Client()), factory)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	handler := obs.RequestIDMiddleware(logger, obs.TokenMiddleware(auth.Middleware(mcpHandler)))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: srv.URL,
		HTTPClient: &http.Client{
			Transport: bearerRoundTripper{token: "client-token", base: srv.Client().Transport},
		},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", textContent(result))
	}

	if got := authHeader.Load(); got != "Bearer client-token" {
		t.Fatalf("Authorization = %v, want Bearer client-token", got)
	}
	if !strings.Contains(textContent(result), "client-repo") {
		t.Fatalf("unexpected result: %s", textContent(result))
	}
}
