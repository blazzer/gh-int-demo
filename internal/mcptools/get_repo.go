package mcptools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type repositoryGetter interface {
	GetRepository(ctx context.Context, owner, name string) (github.Repository, error)
}

type getRepositoryParams struct {
	Owner string `json:"owner" jsonschema:"GitHub repository owner"`
	Name  string `json:"name" jsonschema:"GitHub repository name"`
}

// RegisterGetRepository adds the get_repository tool to the MCP server.
func RegisterGetRepository(server *mcp.Server, defaultClient github.Lister, factory github.ClientFactory) {
	if factory == nil {
		factory = github.DefaultClientFactory()
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_repository",
		Description: "Get a single GitHub repository by owner and name",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params getRepositoryParams) (*mcp.CallToolResult, any, error) {
		logger := obs.LoggerFromContext(ctx)
		requestID := obs.RequestIDFromContext(ctx)

		lister, authSource := resolveLister(ctx, defaultClient, factory)
		logger.Info("get_repository invoked",
			"request_id", requestID,
			"auth_source", authSource,
			"owner", params.Owner,
			"name", params.Name,
		)

		getter, ok := lister.(repositoryGetter)
		if !ok || lister == nil {
			return toolErrorResult(ToolError{
				Code:      CodeInternal,
				Message:   "missing GitHub client; set GITHUB_TOKEN or pass Authorization: Bearer",
				Retryable: false,
			}), nil, nil
		}

		repo, err := getter.GetRepository(ctx, params.Owner, params.Name)
		if err != nil {
			toolErr := toolErrorFrom(err)
			logger.Error("github get repository failed",
				"error", err,
				"error_code", toolErr.Code,
				"retryable", toolErr.Retryable,
				"request_id", requestID,
				"auth_source", authSource,
			)
			return toolErrorResult(toolErr), nil, nil
		}

		summary := RepoSummary{
			Name:        repo.Name,
			Visibility:  repo.Visibility,
			Description: repo.Description,
			URL:         repo.HTMLURL,
		}
		payload, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return toolErrorResult(ToolError{
				Code:      CodeInternal,
				Message:   fmt.Sprintf("encode result: %v", err),
				Retryable: false,
			}), nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(payload)},
			},
		}, summary, nil
	})
}
