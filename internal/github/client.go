package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/blazzer/gh-int-demo/internal/httpx"
	"github.com/blazzer/gh-int-demo/internal/obs"
)

const defaultBaseURL = "https://api.github.com"

// Client is a thin GitHub REST client for listing repositories.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	retry      retryConfig
}

// NewClient creates a GitHub API client with the given token.
func NewClient(token string) *Client {
	return &Client{
		httpClient: httpx.DefaultClient(),
		baseURL:    defaultBaseURL,
		token:      token,
		retry:      defaultRetryConfig(),
	}
}

// DefaultClientFactory returns a factory that reuses a shared tuned HTTP client.
func DefaultClientFactory() ClientFactory {
	shared := httpx.DefaultClient()
	return func(token string) Lister {
		return NewClient(token).WithHTTPClient(shared)
	}
}

// WithHTTPClient sets a custom HTTP client (used in tests).
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	cp := *c
	cp.httpClient = httpClient
	return &cp
}

// WithBaseURL sets a custom API base URL (used in tests).
func (c *Client) WithBaseURL(baseURL string) *Client {
	cp := *c
	cp.baseURL = strings.TrimRight(baseURL, "/")
	return &cp
}

// ListRepositories returns repositories owned by the authenticated user.
func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	if c.token == "" {
		return nil, fmt.Errorf("github: missing token")
	}

	nextURL := c.baseURL + "/user/repos?per_page=100&affiliation=owner"
	all := make([]Repository, 0)

	for nextURL != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		pageRepos, linkHeader, statusCode, err := c.fetchPage(ctx, nextURL)
		obs.IncGitHubRequest(statusCode)
		if err != nil {
			return nil, err
		}
		all = append(all, pageRepos...)

		if next, ok := parseLinkNext(linkHeader); ok {
			nextURL = next
		} else {
			nextURL = ""
		}
	}

	return all, nil
}

func (c *Client) fetchPage(ctx context.Context, url string) ([]Repository, string, int, error) {
	const maxRateLimitRetries = 2
	start := time.Now()

	for rateAttempt := 0; rateAttempt <= maxRateLimitRetries; rateAttempt++ {
		resp, err := withRetry(ctx, c.retry, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+c.token)
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			if requestID := obs.RequestIDFromContext(ctx); requestID != "" {
				req.Header.Set(obs.RequestIDHeader, requestID)
			}
			return c.httpClient.Do(req)
		})
		if err != nil {
			return nil, "", 0, err
		}

		statusCode := resp.StatusCode
		obs.RecordGitHubDuration(time.Since(start).Milliseconds())

		if statusCode == http.StatusUnauthorized {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			closeResponseBody(resp)
			return nil, "", statusCode, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(body)))
		}

		if statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests {
			if rateErr := handleRateLimit(ctx, resp); rateErr == nil {
				closeResponseBody(resp)
				continue
			}
			closeResponseBody(resp)
			return nil, "", statusCode, ErrRateLimited
		}

		if statusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			closeResponseBody(resp)
			return nil, "", statusCode, &APIError{
				Status:  statusCode,
				Message: strings.TrimSpace(string(body)),
			}
		}

		var repos []Repository
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			closeResponseBody(resp)
			return nil, "", statusCode, fmt.Errorf("github: decode repos: %w", err)
		}
		linkHeader := resp.Header.Get("Link")
		if ghReqID := resp.Header.Get("X-GitHub-Request-Id"); ghReqID != "" {
			if logger := obs.LoggerFromContext(ctx); logger != nil {
				logger.Debug("github response", "github_request_id", ghReqID, "request_id", obs.RequestIDFromContext(ctx))
			}
		}
		closeResponseBody(resp)
		return repos, linkHeader, statusCode, nil
	}

	return nil, "", http.StatusForbidden, ErrRateLimited
}
