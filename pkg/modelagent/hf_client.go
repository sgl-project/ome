package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// building endpoint related
const (
	// Default hf endpoint
	DefaultEndpoint = "https://huggingface.co"
	// API path
	HfAPI = "api"
)

// send request and receive response
const (
	// Request timeouts
	DefaultRequestTimeout = 10 * time.Second
)

// HF model metadata attributes name
const (
	Sha = "sha"
)

// build endpoint related
var (
	modelIdPattern = regexp.MustCompile(`^[^/]+/[^/]+$`)
)

// send request and receive response
var (
	// defaultHTTPClient is the shared HTTP client with connection pooling
	defaultHTTPClient *http.Client
	clientOnce        sync.Once
	jitterRandOnce    sync.Once
	jitterRand        *rand.Rand
)

// FetchAttributeFromHfModelMetaData retrieves a single top-level attribute from the Hugging Face model metadata endpoint for the provided modelId.
func FetchAttributeFromHfModelMetaData(ctx context.Context, modelId string, attribute string) (interface{}, error) {
	modelMetaDataUrl, err := hfModelMetaDataUrl(modelId)
	if err != nil {
		return nil, fmt.Errorf("failed to build model metadata URL: %s", err)
	}
	// Get retry configuration from context (HubConfig)
	maxRetries := 3                   // default
	retryInterval := 10 * time.Second // default

	// Use the shared HTTP client for connection pooling
	client := GetHTTPClient()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create a context with timeout for this specific request
		reqCtx, cancel := context.WithTimeout(ctx, DefaultRequestTimeout)

		// build request
		req, err := http.NewRequestWithContext(reqCtx, "GET", modelMetaDataUrl, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create request: %s", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel() // Clean up request context
			// Network errors are retryable
			if attempt < maxRetries {
				delay := exponentialBackoffWithJitter(attempt+1, retryInterval, 60*time.Second)
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("failed to perform request: %s", err)
		}

		// Handle successful response
		var data map[string]interface{}
		if resp.StatusCode == http.StatusOK {
			err := json.NewDecoder(resp.Body).Decode(&data)
			resp.Body.Close()
			cancel()
			if err != nil {
				return nil, fmt.Errorf("failed to decode response: %s", err)
			}
			val, exists := data[attribute]
			if !exists {
				return nil, fmt.Errorf("attribute %s not found in JSON of the response", attribute)
			}

			return val, nil
		}

		// Handle rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp)
			resp.Body.Close()
			cancel()
			if retryAfter == 0 {
				// Use exponential backoff with jitter if no Retry-After header
				retryAfter = exponentialBackoffWithJitter(attempt+1, retryInterval, 300*time.Second) // Max 5 minutes
			}

			// Only retry if we haven't exhausted attempts
			if attempt < maxRetries {
				select {
				case <-time.After(retryAfter):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}

		// Handle other HTTP errors with retry for server errors
		if resp.StatusCode >= 500 && attempt < maxRetries {
			resp.Body.Close()
			cancel()
			delay := exponentialBackoffWithJitter(attempt+1, retryInterval, 60*time.Second)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Handle non-retryable error responses
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("failed to invoke HuggingFace endpoint %s: response status code %d", modelMetaDataUrl, resp.StatusCode)
	}

	// Should not reach here
	return nil, fmt.Errorf("failed to retrieve attribute %s value from HuggingFace after %d attempts", attribute, maxRetries+1)
}

// exponentialBackoffWithJitter calculates the delay with jitter to prevent thundering herd
func exponentialBackoffWithJitter(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Ensure random source is initialized
	initJitterRand()

	// Calculate exponential delay
	delay := time.Duration(math.Min(
		float64(baseDelay)*math.Pow(2, float64(attempt-1)),
		float64(maxDelay),
	))

	// Add jitter (±25% of calculated delay)
	jitter := time.Duration(jitterRand.Float64() * 0.5 * float64(delay))
	if jitterRand.Intn(2) == 0 {
		delay -= jitter
	} else {
		delay += jitter
	}

	return delay
}

// initJitterRand initializes the random source for jitter calculation
func initJitterRand() {
	jitterRandOnce.Do(func() {
		// Use current time nanoseconds as seed for non-deterministic randomness
		source := rand.NewSource(time.Now().UnixNano())
		jitterRand = rand.New(source)
	})
}

// parseRetryAfter parses the Retry-After header from HTTP 429 responses
func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try to parse as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as HTTP date
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// NewHTTPClientWithTimeout creates a new HTTP client with a specific timeout
// This uses the same transport (connection pooling) but with a custom timeout
func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: GetHTTPClient().Transport,
		Timeout:   timeout,
	}
}

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

// hfModelMetaDataUrl builds the Hugging Face model metadata API URL for the
// given modelId.
// Behavior:
// - Trims surrounding whitespace from modelId.
// - Validates modelId against modelIdPattern, requiring format "<orgnization>/<modelName>".
// - Returns an error if modelId is empty or invalid.
// Resulting URL format:
// {https://huggingface.co/api/models/{modelId}
func hfModelMetaDataUrl(modelId string) (string, error) {
	if modelId == "" {
		return "", fmt.Errorf("no model name has been specified")
	}
	modelId = strings.TrimSpace(modelId)

	if !modelIdPattern.MatchString(modelId) {
		return "", fmt.Errorf("invalid model name %q: expected format <namespace>/<model>", modelId)
	}

	baseUrl := fmt.Sprintf("%s/%s", DefaultEndpoint, HfAPI)
	return fmt.Sprintf("%s/models/%s", baseUrl, modelId), nil
}
