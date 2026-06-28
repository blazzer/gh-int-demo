package github

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const maxRateLimitWait = 60 * time.Second

func handleRateLimit(ctx context.Context, resp *http.Response) error {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return nil
	}

	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining != "0" && resp.StatusCode != http.StatusTooManyRequests {
		return nil
	}

	resetUnix, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
	if err != nil {
		return ErrRateLimited
	}

	wait := time.Until(time.Unix(resetUnix, 0))
	if wait <= 0 {
		return ErrRateLimited
	}
	if wait > maxRateLimitWait {
		wait = maxRateLimitWait
	}

	if err := sleepWithContext(ctx, wait); err != nil {
		return err
	}
	return nil
}
