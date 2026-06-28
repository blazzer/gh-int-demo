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
	"github.com/blazzer/gh-int-demo/internal/obs"
)

func TestClient_ListRepositories(t *testing.T) {
	t.Parallel()

	repos := []github.Repository{
		{Name: "alpha", Visibility: "public", Description: "first", HTMLURL: "https://github.com/u/alpha"},
		{Name: "beta", Visibility: "private", Description: "second", HTMLURL: "https://github.com/u/beta"},
	}

	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantErr   error
		wantErrIs error
		wantCount int
		wantNames []string
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
			name: "propagates request id",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(obs.RequestIDHeader); got != "req-123" {
					t.Errorf("X-Request-ID = %q, want req-123", got)
				}
				_ = json.NewEncoder(w).Encode(repos[:1])
			},
			wantCount: 1,
			wantNames: []string{"alpha"},
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
			name: "retry exhausted",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			wantErr: errors.New("retry exhausted"),
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
			name: "rate limit exhausted",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+3600, 10))
				http.Error(w, "rate limited", http.StatusForbidden)
			},
			wantErrIs: github.ErrRateLimited,
		},
		{
			name: "rate limit reset parse error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, "rate limited", http.StatusForbidden)
			},
			wantErrIs: github.ErrRateLimited,
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

			ctx := obs.WithLogger(context.Background(), obs.NewLogger())
			if tt.name == "propagates request id" {
				ctx = obs.WithRequestID(ctx, "req-123")
			}
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
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

func TestClient_ListRepositories_MultiPage(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	page1 := []github.Repository{
		{Name: "alpha", Visibility: "public", HTMLURL: "https://github.com/u/alpha"},
		{Name: "beta", Visibility: "public", HTMLURL: "https://github.com/u/beta"},
	}
	page2 := []github.Repository{
		{Name: "gamma", Visibility: "public", HTMLURL: "https://github.com/u/gamma"},
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.Header().Set("Link", `<`+srv.URL+`/page2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		default:
			_ = json.NewEncoder(w).Encode(page2)
		}
	}))
	t.Cleanup(srv.Close)

	client := github.NewClient("test-token").
		WithBaseURL(srv.URL).
		WithHTTPClient(srv.Client())

	got, err := client.ListRepositories(context.Background())
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d repos, want 3", len(got))
	}
	for i, name := range []string{"alpha", "beta", "gamma"} {
		if got[i].Name != name {
			t.Fatalf("repo[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestClient_ListRepositories_ContextCanceledDuringRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := github.NewClient("test-token").
		WithBaseURL(srv.URL).
		WithHTTPClient(srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.ListRepositories(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRepositories() error = %v, want context.Canceled", err)
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

func TestClient_GetRepository(t *testing.T) {
	t.Parallel()

	repo := github.Repository{
		Name:        "demo",
		Visibility:  "public",
		Description: "hello",
		HTMLURL:     "https://github.com/acme/demo",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		_ = json.NewEncoder(w).Encode(repo)
	}))
	t.Cleanup(srv.Close)

	client := github.NewClient("test-token").
		WithBaseURL(srv.URL).
		WithHTTPClient(srv.Client())

	got, err := client.GetRepository(context.Background(), "acme", "demo")
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}
	if got.Name != "demo" {
		t.Fatalf("Name = %q, want demo", got.Name)
	}
}

func TestClient_GetRepository_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client := github.NewClient("test-token").
		WithBaseURL(srv.URL).
		WithHTTPClient(srv.Client())

	_, err := client.GetRepository(context.Background(), "acme", "missing")
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("GetRepository() error = %v, want 404 APIError", err)
	}
}
