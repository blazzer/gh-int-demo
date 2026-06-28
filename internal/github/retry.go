package github

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 200 * time.Millisecond
	defaultMaxDelay    = 5 * time.Second
)

type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxAttempts: defaultMaxAttempts,
		baseDelay:   defaultBaseDelay,
		maxDelay:    defaultMaxDelay,
	}
}

func withRetry(ctx context.Context, cfg retryConfig, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := fn()
		if err != nil {
			lastErr = err
			if attempt+1 >= cfg.maxAttempts {
				break
			}
			if waitErr := sleepWithContext(ctx, backoffDelay(cfg, attempt)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		if resp.StatusCode < 500 {
			return resp, nil
		}

		lastErr = &APIError{Status: resp.StatusCode, Message: "server error"}
		resp.Body.Close()
		if attempt+1 >= cfg.maxAttempts {
			break
		}
		if waitErr := sleepWithContext(ctx, backoffDelay(cfg, attempt)); waitErr != nil {
			return nil, waitErr
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("github: retry exhausted: %w", lastErr)
	}
	return nil, fmt.Errorf("github: retry exhausted")
}

func backoffDelay(cfg retryConfig, attempt int) time.Duration {
	delay := cfg.baseDelay * time.Duration(1<<attempt)
	if delay > cfg.maxDelay {
		delay = cfg.maxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(delay / 4)))
	return delay + jitter
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
