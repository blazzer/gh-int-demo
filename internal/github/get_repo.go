package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/blazzer/gh-int-demo/internal/obs"
)

// GetRepository returns a single repository by owner and name.
func (c *Client) GetRepository(ctx context.Context, owner, name string) (Repository, error) {
	if c.token == "" {
		return Repository{}, fmt.Errorf("github: missing token")
	}
	if owner == "" || name == "" {
		return Repository{}, fmt.Errorf("github: owner and name are required")
	}

	repoURL := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, url.PathEscape(owner), url.PathEscape(name))
	resp, statusCode, err := c.doGET(ctx, repoURL)
	obs.IncGitHubRequest(statusCode)
	if err != nil {
		return Repository{}, err
	}
	defer closeResponseBody(resp)

	if statusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if statusCode == http.StatusUnauthorized {
			return Repository{}, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(body)))
		}
		return Repository{}, &APIError{
			Status:  statusCode,
			Message: strings.TrimSpace(string(body)),
		}
	}

	var repo Repository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return Repository{}, fmt.Errorf("github: decode repo: %w", err)
	}
	return repo, nil
}

func (c *Client) doGET(ctx context.Context, reqURL string) (*http.Response, int, error) {
	const maxRateLimitRetries = 2
	start := time.Now()

	for rateAttempt := 0; rateAttempt <= maxRateLimitRetries; rateAttempt++ {
		resp, err := withRetry(ctx, c.retry, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
			return nil, 0, err
		}

		statusCode := resp.StatusCode
		obs.RecordGitHubDuration(time.Since(start).Milliseconds())

		if statusCode == http.StatusUnauthorized {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			closeResponseBody(resp)
			return nil, statusCode, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(body)))
		}

		if statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests {
			if rateErr := handleRateLimit(ctx, resp); rateErr == nil {
				closeResponseBody(resp)
				continue
			}
			closeResponseBody(resp)
			return nil, statusCode, ErrRateLimited
		}

		if ghReqID := resp.Header.Get("X-GitHub-Request-Id"); ghReqID != "" {
			if logger := obs.LoggerFromContext(ctx); logger != nil {
				logger.Debug("github response", "github_request_id", ghReqID, "request_id", obs.RequestIDFromContext(ctx))
			}
		}

		return resp, statusCode, nil
	}

	return nil, http.StatusForbidden, ErrRateLimited
}
