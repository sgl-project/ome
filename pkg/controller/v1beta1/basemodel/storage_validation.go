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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	storagepkg "github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	// HuggingFace API constants
	DefaultHFEndpoint       = "https://huggingface.co"
	HFAPIPath               = "api/models"
	ValidationTimeout       = 10 * time.Second
	DefaultSecretTokenKey   = "token"
	ValidationCheckInterval = 24 * time.Hour // Re-validate after 24 hours
)

var (
	// modelIdPattern validates the format of HuggingFace model IDs
	modelIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,96}/[a-zA-Z0-9_.-]{1,96}$`)

	// HTTP client with connection pooling for HF API calls
	hfHTTPClient *http.Client
	hfClientOnce sync.Once
)

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

// ValidateAndUpdateStorageStatus validates the HuggingFace storage URI and updates the status
// This is called during reconciliation to check if the model exists on HuggingFace
func ValidateAndUpdateStorageStatus(
	ctx context.Context,
	k8sClient client.Client,
	storage *v1beta1.StorageSpec,
	currentStatus *v1beta1.StorageValidationStatus,
	namespace string,
	log logr.Logger,
) *v1beta1.StorageValidationStatus {
	// Skip validation if storage is nil or storageUri is nil
	if storage == nil || storage.StorageUri == nil {
		return nil
	}

	storageURI := *storage.StorageUri

	// Only validate HuggingFace URIs
	if !strings.HasPrefix(storageURI, storagepkg.HuggingFaceStoragePrefix) {
		return nil
	}

	// Skip validation if we've already validated recently and status hasn't changed
	if currentStatus != nil && currentStatus.LastChecked != nil {
		timeSinceCheck := time.Since(currentStatus.LastChecked.Time)
		if timeSinceCheck < ValidationCheckInterval {
			log.V(1).Info("Skipping HuggingFace validation, recently checked",
				"lastChecked", currentStatus.LastChecked.Time,
				"timeSince", timeSinceCheck)
			return currentStatus
		}
	}

	log.Info("Validating HuggingFace storage URI", "storageUri", storageURI)

	// Get the token from the secret if specified
	token := ""
	if storage.StorageKey != nil && *storage.StorageKey != "" {
		var err error
		token, err = getTokenFromSecret(ctx, k8sClient, *storage.StorageKey, namespace, storage.Parameters)
		if err != nil {
			log.Info("Failed to retrieve token from secret, proceeding without authentication",
				"secret", *storage.StorageKey,
				"namespace", namespace,
				"error", err)
		}
	}

	// Validate the HuggingFace model
	result := validateHuggingFaceStorageURI(ctx, storageURI, token)

	now := metav1.Now()
	validationStatus := &v1beta1.StorageValidationStatus{
		Valid:       result.Valid,
		LastChecked: &now,
	}

	if result.ErrorMessage != "" {
		validationStatus.Message = result.ErrorMessage
	} else if result.WarningMessage != "" {
		validationStatus.Message = result.WarningMessage
	} else if result.Valid {
		validationStatus.Message = "HuggingFace model exists and is accessible"
	}

	return validationStatus
}

// validationResult represents the result of validating a HuggingFace model
type validationResult struct {
	Valid          bool
	Exists         bool
	RequiresAuth   bool
	ErrorMessage   string
	WarningMessage string
}

// validateHuggingFaceStorageURI validates a HuggingFace storage URI
func validateHuggingFaceStorageURI(ctx context.Context, storageURI string, token string) validationResult {
	// Parse the HuggingFace storage URI
	components, err := storagepkg.ParseHuggingFaceStorageURI(storageURI)
	if err != nil {
		return validationResult{
			Valid:        false,
			Exists:       false,
			ErrorMessage: fmt.Sprintf("invalid HuggingFace storage URI: %v", err),
		}
	}

	return validateHuggingFaceModel(ctx, components.ModelID, token)
}

// validateHuggingFaceModel validates that a HuggingFace model exists
func validateHuggingFaceModel(ctx context.Context, modelID string, token string) validationResult {
	result := validationResult{
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

	// Create the HTTP request with timeout context
	reqCtx, cancel := context.WithTimeout(ctx, ValidationTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, apiURL, nil)
	if err != nil {
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
		if reqCtx.Err() != nil {
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
		// Model does not exist
		result.Valid = false
		result.Exists = false
		result.ErrorMessage = fmt.Sprintf("HuggingFace model %q does not exist. Please verify the model ID in your storageUri", modelID)
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

// getTokenFromSecret retrieves the authentication token from a Kubernetes secret
func getTokenFromSecret(ctx context.Context, k8sClient client.Client, secretName string, namespace string, parameters *map[string]string) (string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Determine the key to use for the token
	secretKey := DefaultSecretTokenKey
	if parameters != nil {
		if customKey, exists := (*parameters)["secretKey"]; exists && customKey != "" {
			secretKey = customKey
		}
	}

	tokenBytes, exists := secret.Data[secretKey]
	if !exists {
		return "", fmt.Errorf("secret %s/%s does not contain key %q", namespace, secretName, secretKey)
	}

	return string(tokenBytes), nil
}

