package mcptools

import (
	"context"
	"encoding/json"
	"errors"
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
func RegisterListRepositories(server *mcp.Server, defaultClient github.Lister, factory github.ClientFactory) {
	if factory == nil {
		factory = github.DefaultClientFactory()
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_repositories",
		Description: "List repositories for the authenticated GitHub user",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		logger := obs.LoggerFromContext(ctx)
		requestID := obs.RequestIDFromContext(ctx)

		lister, authSource := resolveLister(ctx, defaultClient, factory)
		logger.Info("list_repositories invoked", "request_id", requestID, "auth_source", authSource)

		if lister == nil {
			return toolErrorResult(ToolError{
				Code:      CodeInternal,
				Message:   "missing GitHub token; set GITHUB_TOKEN or pass Authorization: Bearer",
				Retryable: false,
			}), nil, nil
		}

		repos, err := lister.ListRepositories(ctx)
		if err != nil {
			toolErr := toolErrorFrom(err)
			logger.Error("github list repositories failed",
				"error", err,
				"error_code", toolErr.Code,
				"retryable", toolErr.Retryable,
				"request_id", requestID,
				"auth_source", authSource,
			)
			return toolErrorResult(toolErr), nil, nil
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
		}, summaries, nil
	})
}

func toolErrorFrom(err error) ToolError {
	if errors.Is(err, github.ErrUnauthorized) {
		return ToolError{Code: CodeUnauthorized, Message: err.Error(), Retryable: false}
	}
	if errors.Is(err, github.ErrRateLimited) {
		return ToolError{Code: CodeRateLimited, Message: err.Error(), Retryable: true}
	}
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		return ToolError{
			Code:      CodeUpstream,
			Message:   apiErr.Error(),
			Retryable: apiErr.Status >= 500,
		}
	}
	return ToolError{Code: CodeInternal, Message: err.Error(), Retryable: false}
}

func toolErrorResult(toolErr ToolError) *mcp.CallToolResult {
	payload, _ := json.Marshal(toolErr)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		},
		IsError: true,
	}
}
