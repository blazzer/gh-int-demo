package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/blazzer/gh-int-demo/internal/mcptools"
	"github.com/blazzer/gh-int-demo/internal/oauth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	serverURL = flag.String("server", "http://localhost:8080/mcp", "MCP server URL")
	scope     = flag.String("scope", "repo", "GitHub OAuth scope")
	clientID  = flag.String("client-id", "", "GitHub OAuth client ID (or set GITHUB_CLIENT_ID)")
)

func main() {
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	token, err := oauth.DeviceFlow(context.Background(), oauth.Config{
		ClientID: *clientID,
		Scope:    *scope,
	})
	if err != nil {
		logger.Error("oauth failed", "error", err)
		os.Exit(1)
	}

	if err := writeTokenCache(token); err != nil {
		logger.Warn("failed to cache token locally", "error", err)
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "gh-int-demo-client",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: *serverURL,
		HTTPClient: &http.Client{
			Transport: bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
	}, nil)
	if err != nil {
		logger.Error("connect failed", "error", err)
		os.Exit(1)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_repositories"})
	if err != nil {
		logger.Error("call tool failed", "error", err)
		os.Exit(1)
	}
	if result.IsError {
		logger.Error("tool returned error", "content", textContent(result))
		os.Exit(1)
	}

	if err := printRepositories(os.Stdout, textContent(result)); err != nil {
		logger.Error("print failed", "error", err)
		os.Exit(1)
	}
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (rt bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+rt.token)
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func textContent(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return fmt.Sprintf("%v", result.Content[0])
}

func printRepositories(out *os.File, payload string) error {
	var repos []mcptools.RepoSummary
	if err := json.Unmarshal([]byte(payload), &repos); err != nil {
		return fmt.Errorf("decode repositories: %w", err)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVISIBILITY\tDESCRIPTION\tURL")
	for _, repo := range repos {
		desc := strings.ReplaceAll(repo.Description, "\n", " ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", repo.Name, repo.Visibility, desc, repo.URL)
	}
	return w.Flush()
}

func writeTokenCache(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := home + string(os.PathSeparator) + ".gh-int-demo-token"
	content := "# demo only; do not commit\n" + token + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}
