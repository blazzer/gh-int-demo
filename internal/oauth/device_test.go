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

func TestDeviceFlow_Success(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/device/code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1,
			})
		case r.URL.Path == "/access_token":
			switch polls.Add(1) {
			case 1:
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			case 2:
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
			default:
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

	base := srv.URL
	token, err := oauth.DeviceFlow(context.Background(), oauth.Config{
		ClientID:     "client-id",
		DeviceURL:    base + "/device/code",
		TokenURL:     base + "/access_token",
		PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("DeviceFlow() error = %v", err)
	}
	if token != "gho_test_token" {
		t.Fatalf("token = %q, want gho_test_token", token)
	}
}

func TestDeviceFlow_AccessDenied(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(context.Background(), oauth.Config{
		ClientID:     "client-id",
		DeviceURL:    srv.URL + "/device/code",
		TokenURL:     srv.URL + "/access_token",
		PollInterval: 10 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("DeviceFlow() error = %v, want access denied", err)
	}
}

func TestDeviceFlow_ExpiredToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device/code" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "expired_token"})
	}))
	t.Cleanup(srv.Close)

	_, err := oauth.DeviceFlow(context.Background(), oauth.Config{
		ClientID:     "client-id",
		DeviceURL:    srv.URL + "/device/code",
		TokenURL:     srv.URL + "/access_token",
		PollInterval: 10 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("DeviceFlow() error = %v, want expired", err)
	}
}

func TestDeviceFlow_MissingClientID(t *testing.T) {
	t.Parallel()

	_, err := oauth.DeviceFlow(context.Background(), oauth.Config{})
	if err == nil || !strings.Contains(err.Error(), "GITHUB_CLIENT_ID") {
		t.Fatalf("DeviceFlow() error = %v, want missing client id", err)
	}
}
