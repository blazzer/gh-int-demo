package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/blazzer/gh-int-demo/internal/httpx"
)

const (
	deviceCodeURL  = "https://github.com/login/device/code"
	accessTokenURL = "https://github.com/login/oauth/access_token"
)

// Config holds GitHub OAuth device flow settings.
type Config struct {
	ClientID     string
	Scope        string
	HTTPClient   *http.Client
	DeviceURL    string
	TokenURL     string
	Output       io.Writer
	PollInterval time.Duration
}

// DeviceFlow runs the GitHub OAuth2 device authorization grant.
func DeviceFlow(ctx context.Context, cfg Config) (string, error) {
	if cfg.ClientID == "" {
		cfg.ClientID = os.Getenv("GITHUB_CLIENT_ID")
	}
	if cfg.ClientID == "" {
		return "", fmt.Errorf("oauth: GITHUB_CLIENT_ID is required")
	}
	if cfg.Scope == "" {
		cfg.Scope = "repo"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = httpx.DefaultClient()
	}
	if cfg.DeviceURL == "" {
		cfg.DeviceURL = deviceCodeURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = accessTokenURL
	}
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}

	device, err := requestDeviceCode(ctx, cfg)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(cfg.Output, "Visit %s and enter code: %s\n", device.VerificationURI, device.UserCode)

	interval := cfg.PollInterval
	if interval == 0 {
		interval = time.Duration(device.Interval) * time.Second
	}
	if interval == 0 {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		token, pending, slowDown, pollErr := pollAccessToken(ctx, cfg, device.DeviceCode)
		if pollErr != nil {
			return "", pollErr
		}
		if token != "" {
			return token, nil
		}
		if slowDown {
			interval += 5 * time.Second
		}
		if pending {
			if err := sleep(ctx, interval); err != nil {
				return "", err
			}
			continue
		}
	}

	return "", fmt.Errorf("oauth: device code expired")
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

func requestDeviceCode(ctx context.Context, cfg Config) (*deviceCodeResponse, error) {
	form := url.Values{
		"client_id": {cfg.ClientID},
		"scope":     {cfg.Scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: device code request failed: %s", strings.TrimSpace(string(body)))
	}

	var device deviceCodeResponse
	if err := json.Unmarshal(body, &device); err != nil {
		return nil, fmt.Errorf("oauth: decode device code: %w", err)
	}
	return &device, nil
}

func pollAccessToken(ctx context.Context, cfg Config, deviceCode string) (token string, pending bool, slowDown bool, err error) {
	form := url.Values{
		"client_id":   {cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", false, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", false, false, fmt.Errorf("oauth: poll token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", false, false, err
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", false, false, fmt.Errorf("oauth: decode token: %w", err)
	}

	switch tr.Error {
	case "":
		if tr.AccessToken == "" {
			return "", false, false, fmt.Errorf("oauth: empty access token")
		}
		return tr.AccessToken, false, false, nil
	case "authorization_pending":
		return "", true, false, nil
	case "slow_down":
		return "", true, true, nil
	case "expired_token":
		return "", false, false, fmt.Errorf("oauth: device code expired")
	case "access_denied":
		return "", false, false, fmt.Errorf("oauth: access denied")
	default:
		return "", false, false, fmt.Errorf("oauth: token error: %s", tr.Error)
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
