//go:build integration

package main

import (
	"context"
	"os"
	"testing"

	"github.com/blazzer/gh-int-demo/internal/github"
)

func TestIntegration_ListRepositories_Live(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set")
	}

	client := github.NewClient(token)
	repos, err := client.ListRepositories(context.Background())
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	t.Logf("listed %d repositories", len(repos))
}
