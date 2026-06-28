package github

import (
	"context"
	"errors"
	"fmt"
)

// Repository represents a GitHub repository returned by the API.
type Repository struct {
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
}

// Lister fetches repositories for the authenticated user.
type Lister interface {
	ListRepositories(ctx context.Context) ([]Repository, error)
}

// ClientFactory creates a Lister for the given GitHub token.
type ClientFactory func(token string) Lister

// ErrUnauthorized indicates the GitHub token is invalid or expired.
var ErrUnauthorized = errors.New("github: unauthorized")

// ErrRateLimited indicates the GitHub API rate limit was exceeded.
var ErrRateLimited = errors.New("github: rate limited")

// APIError wraps a non-success GitHub API response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github: API error %d: %s", e.Status, e.Message)
}
