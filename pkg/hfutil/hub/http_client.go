package hub

import (
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	// defaultHTTPClient is the shared HTTP client with connection pooling
	defaultHTTPClient *http.Client
	clientOnce        sync.Once
)

// GetHTTPClient returns a properly configured HTTP client with connection pooling
// This client is shared across all Hub operations for efficient connection reuse
func GetHTTPClient() *http.Client {
	clientOnce.Do(func() {
		// Configure transport with connection pooling and HTTP/2 support
		transport := &http.Transport{
			// Connection pooling settings
			MaxIdleConns:        100,              // Maximum idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
			MaxConnsPerHost:     20,               // Maximum total connections per host
			IdleConnTimeout:     90 * time.Second, // How long idle connections are kept alive

			// HTTP/2 support
			ForceAttemptHTTP2: true, // Try HTTP/2 when available

			// Timeouts for establishing connections
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second, // Connection timeout
				KeepAlive: 30 * time.Second, // Keep-alive probe interval
			}).DialContext,

			// TLS handshake timeout
			TLSHandshakeTimeout: 10 * time.Second,

			// Other timeouts
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,

			// Compression
			DisableCompression: false, // Enable gzip compression
		}

		defaultHTTPClient = &http.Client{
			Transport: transport,
			Timeout:   0, // No overall timeout, we handle timeouts per-request
		}
	})

	return defaultHTTPClient
}

// NewHTTPClientWithTimeout creates a new HTTP client with a specific timeout
// This uses the same transport (connection pooling) but with a custom timeout
func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: GetHTTPClient().Transport,
		Timeout:   timeout,
	}
}
