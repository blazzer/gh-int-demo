package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blazzer/gh-int-demo/internal/github"
)

func TestClient_ListRepositories(t *testing.T) {
	t.Parallel()

	repos := []github.Repository{
		{Name: "alpha", Visibility: "public", Description: "first", HTMLURL: "https://github.com/u/alpha"},
		{Name: "beta", Visibility: "private", Description: "second", HTMLURL: "https://github.com/u/beta"},
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    error
		wantErrIs  error
		wantCount  int
		wantNames  []string
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("Authorization = %q, want Bearer test-token", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(repos)
			},
			wantCount: 2,
			wantNames: []string{"alpha", "beta"},
		},
		{
			name: "retry on 500 then success",
			handler: func() http.HandlerFunc {
				var calls atomic.Int32
				return func(w http.ResponseWriter, r *http.Request) {
					if calls.Add(1) == 1 {
						http.Error(w, "boom", http.StatusInternalServerError)
						return
					}
				_ = json.NewEncoder(w).Encode(repos[:1])
				}
			}(),
			wantCount: 1,
			wantNames: []string{"alpha"},
		},
		{
			name: "rate limit then success",
			handler: func() http.HandlerFunc {
				var calls atomic.Int32
				return func(w http.ResponseWriter, r *http.Request) {
					if calls.Add(1) == 1 {
						w.Header().Set("X-RateLimit-Remaining", "0")
						w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+1, 10))
						http.Error(w, "rate limited", http.StatusForbidden)
						return
					}
					_ = json.NewEncoder(w).Encode(repos[:1])
				}
			}(),
			wantCount: 1,
			wantNames: []string{"alpha"},
		},
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad credentials", http.StatusUnauthorized)
			},
			wantErrIs: github.ErrUnauthorized,
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("not-json"))
			},
			wantErr: errors.New("decode"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(tt.handler))
			t.Cleanup(srv.Close)

			client := github.NewClient("test-token").
				WithBaseURL(srv.URL).
				WithHTTPClient(srv.Client())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)

			got, err := client.ListRepositories(ctx)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("ListRepositories() error = %v, want %v", err, tt.wantErrIs)
				}
				return
			}
			if tt.wantErr != nil {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Fatalf("ListRepositories() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListRepositories() unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d repos, want %d", len(got), tt.wantCount)
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Fatalf("repo[%d].Name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestClient_ListRepositories_MissingToken(t *testing.T) {
	t.Parallel()

	client := github.NewClient("")
	_, err := client.ListRepositories(context.Background())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}
