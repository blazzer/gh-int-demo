package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
		token:      token,
		retry:      defaultRetryConfig(),
	}
}

// WithHTTPClient sets a custom HTTP client (used in tests).
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	c.httpClient = httpClient
	return c
}

// WithBaseURL sets a custom API base URL (used in tests).
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = strings.TrimRight(baseURL, "/")
	return c
}

// ListRepositories returns repositories owned by the authenticated user.
func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	if c.token == "" {
		return nil, fmt.Errorf("github: missing token")
	}

	url := c.baseURL + "/user/repos?per_page=100&affiliation=owner"
	const maxRateLimitRetries = 2

	for rateAttempt := 0; rateAttempt <= maxRateLimitRetries; rateAttempt++ {
		resp, err := withRetry(ctx, c.retry, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+c.token)
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			return c.httpClient.Do(req)
		})
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(body)))
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			if rateErr := handleRateLimit(ctx, resp); rateErr == nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()
			return nil, ErrRateLimited
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return nil, &APIError{
				Status:  resp.StatusCode,
				Message: strings.TrimSpace(string(body)),
			}
		}

		var repos []Repository
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("github: decode repos: %w", err)
		}
		resp.Body.Close()
		return repos, nil
	}

	return nil, ErrRateLimited
}
