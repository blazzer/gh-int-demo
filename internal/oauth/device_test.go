package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blazzer/gh-int-demo/internal/oauth"
)

func testDeviceConfig(srv *httptest.Server) oauth.Config {
	return oauth.Config{
		ClientID:     "client-id",
		DeviceURL:    srv.URL + "/device/code",
		TokenURL:     srv.URL + "/access_token",
		HTTPClient:   srv.Client(),
		PollInterval: 10 * time.Millisecond,
	}
}

func encodeDeviceCode(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"device_code":      "device-123",
		"user_code":        "ABCD-1234",
		"verification_uri": "https://github.com/login/device",
		"expires_in":       900,
		"interval":         1,
	})
}

func TestDeviceFlow_Success(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/device/code":
			encodeDeviceCode(w)
		case r.URL.Path == "/access_token":
			if polls.Add(1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{
					"access_token": "gho_test_token",
					"token_type":   "bearer",
				})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	token, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err != nil {
		t.Fatalf("DeviceFlow() error = %v", err)
	}
	if token != "gho_test_token" {
		t.Fatalf("token = %q, want gho_test_token", token)
	}
}

func TestDeviceFlow_SlowDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow_down backoff test in short mode")
	}
	t.Parallel()

	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/device/code":
			encodeDeviceCode(w)
		case r.URL.Path == "/access_token":
			switch polls.Add(1) {
			case 1:
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
			default:
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("DeviceFlow() error = %v, want access denied", err)
	}
	if polls.Load() < 2 {
		t.Fatalf("polls = %d, want at least 2", polls.Load())
	}
}

func TestDeviceFlow_AccessDenied(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			encodeDeviceCode(w)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("DeviceFlow() error = %v, want access denied", err)
	}
}

func TestDeviceFlow_ExpiredToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			encodeDeviceCode(w)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "expired_token"})
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("DeviceFlow() error = %v, want expired", err)
	}
}

func TestDeviceFlow_MissingClientID(t *testing.T) {
	t.Parallel()

	_, err := oauth.DeviceFlow(t.Context(), oauth.Config{})
	if err == nil || !strings.Contains(err.Error(), "GITHUB_CLIENT_ID") {
		t.Fatalf("DeviceFlow() error = %v, want missing client id", err)
	}
}

func TestDeviceFlow_NonJSONDeviceCode(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "decode device code") {
		t.Fatalf("DeviceFlow() error = %v, want decode error", err)
	}
}

func TestDeviceFlow_UnknownTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			encodeDeviceCode(w)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown"})
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(t.Context(), testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("DeviceFlow() error = %v, want unknown token error", err)
	}
}

func TestDeviceFlow_ContextCanceledDuringPoll(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			encodeDeviceCode(w)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := oauth.DeviceFlow(ctx, testDeviceConfig(srv))
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("DeviceFlow() error = %v, want context canceled", err)
	}
}
