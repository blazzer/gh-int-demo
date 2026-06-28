package httpx

import (
	"net"
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// DefaultTransport returns an HTTP transport with tuned connection timeouts.
func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	}
}

// DefaultClient returns an HTTP client with DefaultTransport and a 30s overall timeout.
func DefaultClient() *http.Client {
	return &http.Client{
		Timeout:   defaultTimeout,
		Transport: DefaultTransport(),
	}
}
