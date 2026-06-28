package mcptools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RepoSummary is the MCP-facing repository shape.
type RepoSummary struct {
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// RegisterListRepositories adds the list_repositories tool to the MCP server.
func RegisterListRepositories(server *mcp.Server, defaultClient github.Lister) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_repositories",
		Description: "List repositories for the authenticated GitHub user",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		logger := obs.LoggerFromContext(ctx)
		logger.Info("list_repositories invoked", "request_id", obs.RequestIDFromContext(ctx))

		lister := defaultClient
		if token := obs.GitHubTokenFromContext(ctx); token != "" {
			lister = github.NewClient(token)
		}
		if lister == nil {
			return toolError("missing GitHub token; set GITHUB_TOKEN or pass Authorization: Bearer")
		}

		repos, err := lister.ListRepositories(ctx)
		if err != nil {
			logger.Error("github list repositories failed", "error", err)
			return toolError(fmt.Sprintf("github API error: %v", err))
		}

		summaries := make([]RepoSummary, 0, len(repos))
		for _, repo := range repos {
			summaries = append(summaries, RepoSummary{
				Name:        repo.Name,
				Visibility:  repo.Visibility,
				Description: repo.Description,
				URL:         repo.HTMLURL,
			})
		}

		payload, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			return toolError(fmt.Sprintf("encode result: %v", err))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(payload)},
			},
		}, summaries, nil
	})
}

func toolError(message string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: true,
	}, nil, nil
}
