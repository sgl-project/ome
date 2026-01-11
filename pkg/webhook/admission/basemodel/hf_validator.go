package basemodel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	// HuggingFace API constants
	DefaultHFEndpoint = "https://huggingface.co"
	HFAPIPath         = "api/models"
	ValidationTimeout = 10 * time.Second
)

var (
	// modelIdPattern validates the format of HuggingFace model IDs
	// HuggingFace model IDs consist of organization/model where each part:
	// - Contains alphanumeric characters, hyphens, underscores, and periods
	// - Must have at least 1 character per segment
	// - Has a reasonable max length (96 chars per segment)
	modelIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,96}/[a-zA-Z0-9_.-]{1,96}$`)

	// HTTP client with connection pooling for HF API calls
	hfHTTPClient *http.Client
	hfClientOnce sync.Once
)

// HuggingFaceValidationResult represents the result of validating a HuggingFace model
type HuggingFaceValidationResult struct {
	// Valid indicates if the validation was successful and the model can be used
	Valid bool
	// Exists indicates if the model exists on HuggingFace (false for 404)
	Exists bool
	// RequiresAuth indicates if the model requires authentication (401/403)
	RequiresAuth bool
	// ErrorMessage contains the error message if validation failed
	ErrorMessage string
	// WarningMessage contains any warning messages to surface to the user
	WarningMessage string
}

// getHFHTTPClient returns a properly configured HTTP client for HuggingFace API calls
func getHFHTTPClient() *http.Client {
	hfClientOnce.Do(func() {
		transport := &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			MaxConnsPerHost:     10,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		}

		hfHTTPClient = &http.Client{
			Transport: transport,
			Timeout:   ValidationTimeout,
		}
	})

	return hfHTTPClient
}

// ValidateHuggingFaceModel validates that a HuggingFace model exists and is accessible
// It queries the HuggingFace API to check if the model exists.
// For gated/private models, it will use the provided token for authentication.
func ValidateHuggingFaceModel(ctx context.Context, modelID string, token string) HuggingFaceValidationResult {
	result := HuggingFaceValidationResult{
		Valid:  true,
		Exists: true,
	}

	// Validate model ID format
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = "model ID cannot be empty"
		return result
	}

	if !modelIdPattern.MatchString(modelID) {
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = fmt.Sprintf("invalid model ID format %q: expected format <organization>/<model>", modelID)
		return result
	}

	// Build the HuggingFace API URL
	apiURL := fmt.Sprintf("%s/%s/%s", DefaultHFEndpoint, HFAPIPath, modelID)

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, apiURL, nil)
	if err != nil {
		// Fail open - allow the resource but warn
		result.WarningMessage = fmt.Sprintf("failed to create validation request: %v, proceeding without validation", err)
		return result
	}

	// Add authorization header if token is provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute the request
	client := getHFHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		// Check if it's a timeout or network error - fail open
		if ctx.Err() != nil {
			result.WarningMessage = "HuggingFace model validation timed out, proceeding without validation"
			return result
		}
		result.WarningMessage = fmt.Sprintf("HuggingFace API unavailable (%v), proceeding without validation", err)
		return result
	}
	defer func() {
		// Drain body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Model exists and is accessible
		result.Valid = true
		result.Exists = true
		return result

	case http.StatusUnauthorized, http.StatusForbidden:
		// Model exists but requires authentication
		result.Valid = true
		result.Exists = true
		result.RequiresAuth = true
		if token == "" {
			result.WarningMessage = fmt.Sprintf("HuggingFace model %q may require authentication; consider providing a token via storageKey if access fails during download", modelID)
		}
		return result

	case http.StatusNotFound:
		// Model does not exist - reject
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = fmt.Sprintf("HuggingFace model %q does not exist. Please verify the model ID in your storageUri (hf://%s)", modelID, modelID)
		return result

	case http.StatusTooManyRequests:
		// Rate limited - fail open
		result.WarningMessage = "HuggingFace API rate limited, proceeding without validation"
		return result

	default:
		// Unknown status - fail open
		if resp.StatusCode >= 500 {
			result.WarningMessage = fmt.Sprintf("HuggingFace API returned server error (status %d), proceeding without validation", resp.StatusCode)
		} else {
			result.WarningMessage = fmt.Sprintf("HuggingFace API returned unexpected status %d, proceeding without validation", resp.StatusCode)
		}
		return result
	}
}

// ValidateHuggingFaceStorageURI validates a HuggingFace storage URI and checks if the model exists
// It parses the URI, extracts the model ID, and validates it against the HuggingFace API
func ValidateHuggingFaceStorageURI(ctx context.Context, storageURI string, token string) HuggingFaceValidationResult {
	// Parse the HuggingFace storage URI
	components, err := storage.ParseHuggingFaceStorageURI(storageURI)
	if err != nil {
		return HuggingFaceValidationResult{
			Valid:        false,
			Exists:       false,
			ErrorMessage: fmt.Sprintf("invalid HuggingFace storage URI: %v", err),
		}
	}

	// Validate the model exists
	return ValidateHuggingFaceModel(ctx, components.ModelID, token)
}

// IsHuggingFaceURI checks if the given storage URI is a HuggingFace URI
func IsHuggingFaceURI(storageURI string) bool {
	return strings.HasPrefix(storageURI, storage.HuggingFaceStoragePrefix)
}

// ValidateModelIDFormat validates only the format of a HuggingFace model ID in a storage URI
// This is used for admission webhook validation (fast, no network calls)
func ValidateModelIDFormat(storageURI string) HuggingFaceValidationResult {
	result := HuggingFaceValidationResult{
		Valid:  true,
		Exists: true, // Assume exists, will be validated later by reconciler
	}

	// Parse the HuggingFace storage URI
	components, err := storage.ParseHuggingFaceStorageURI(storageURI)
	if err != nil {
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = fmt.Sprintf("invalid HuggingFace storage URI: %v", err)
		return result
	}

	// Validate model ID format
	modelID := strings.TrimSpace(components.ModelID)
	if modelID == "" {
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = "model ID cannot be empty"
		return result
	}

	if !modelIdPattern.MatchString(modelID) {
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = fmt.Sprintf("invalid model ID format %q: expected format <organization>/<model>", modelID)
		return result
	}

	return result
}
