package basemodel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

func TestIsHuggingFaceURI(t *testing.T) {
	tests := []struct {
		name       string
		storageURI string
		expected   bool
	}{
		{
			name:       "Valid HuggingFace URI",
			storageURI: "hf://meta-llama/Llama-2-7b",
			expected:   true,
		},
		{
			name:       "Valid HuggingFace URI with branch",
			storageURI: "hf://meta-llama/Llama-2-7b@main",
			expected:   true,
		},
		{
			name:       "OCI storage URI",
			storageURI: "oci://n/namespace/b/bucket/o/object",
			expected:   false,
		},
		{
			name:       "PVC storage URI",
			storageURI: "pvc://my-pvc/subpath",
			expected:   false,
		},
		{
			name:       "S3 storage URI",
			storageURI: "s3://bucket/prefix",
			expected:   false,
		},
		{
			name:       "Empty URI",
			storageURI: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHuggingFaceURI(tt.storageURI)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateModelIDFormat(t *testing.T) {
	tests := []struct {
		name          string
		storageURI    string
		expectedValid bool
		hasError      bool
	}{
		{
			name:          "Valid model ID format",
			storageURI:    "hf://meta-llama/Llama-2-7b",
			expectedValid: true,
			hasError:      false,
		},
		{
			name:          "Valid model ID with branch",
			storageURI:    "hf://meta-llama/Llama-2-7b@main",
			expectedValid: true,
			hasError:      false,
		},
		{
			name:          "Valid model ID with underscores and dots",
			storageURI:    "hf://org_name/model.name-v1",
			expectedValid: true,
			hasError:      false,
		},
		{
			name:          "Invalid model ID format - missing org",
			storageURI:    "hf://just-model-name",
			expectedValid: false,
			hasError:      true,
		},
		{
			name:          "Invalid HuggingFace URI format",
			storageURI:    "hf://",
			expectedValid: false,
			hasError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateModelIDFormat(tt.storageURI)
			assert.Equal(t, tt.expectedValid, result.Valid)
			if tt.hasError {
				assert.NotEmpty(t, result.ErrorMessage)
			} else {
				assert.Empty(t, result.ErrorMessage)
			}
		})
	}
}

func TestValidateHuggingFaceModel_LocalValidation(t *testing.T) {
	// These tests validate the local input validation logic (model ID format)
	// without hitting the actual HuggingFace API
	tests := []struct {
		name           string
		modelID        string
		expectedValid  bool
		expectedExists bool
		hasError       bool
	}{
		{
			name:           "Empty model ID",
			modelID:        "",
			expectedValid:  false,
			expectedExists: false,
			hasError:       true,
		},
		{
			name:           "Invalid model ID format - missing org",
			modelID:        "just-model-name",
			expectedValid:  false,
			expectedExists: false,
			hasError:       true,
		},
		{
			name:           "Invalid model ID format - too many slashes",
			modelID:        "org/model/extra",
			expectedValid:  false,
			expectedExists: false,
			hasError:       true,
		},
		{
			name:           "Invalid model ID format - whitespace only",
			modelID:        "   ",
			expectedValid:  false,
			expectedExists: false,
			hasError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHuggingFaceModel(context.Background(), tt.modelID, "")
			assert.Equal(t, tt.expectedValid, result.Valid)
			assert.Equal(t, tt.expectedExists, result.Exists)
			if tt.hasError {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

func TestValidateHuggingFaceModel_Integration(t *testing.T) {
	// These tests hit the actual HuggingFace API
	// Skip in short mode or CI environments without network access
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tests := []struct {
		name           string
		modelID        string
		token          string
		expectedValid  bool
		expectedExists bool
		expectWarning  bool
	}{
		{
			name:           "Valid public model - bert-base-uncased",
			modelID:        "google-bert/bert-base-uncased",
			token:          "",
			expectedValid:  true,
			expectedExists: true,
			expectWarning:  false,
		},
		{
			// Note: HuggingFace API returns 401 for non-existent models (not 404)
			// Our webhook treats 401 as "requires auth" and allows with warning
			// This is the "fail open" behavior we want for authentication scenarios
			name:           "Non-existent model (returns 401 from HF API)",
			modelID:        "nonexistent-org-12345/nonexistent-model-67890",
			token:          "",
			expectedValid:  true, // Fail open - allow resource creation
			expectedExists: true, // We assume exists since we got 401 not 404
			expectWarning:  true, // Should have warning about auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result := ValidateHuggingFaceModel(ctx, tt.modelID, tt.token)
			assert.Equal(t, tt.expectedValid, result.Valid)
			assert.Equal(t, tt.expectedExists, result.Exists)
			if tt.expectWarning {
				assert.NotEmpty(t, result.WarningMessage, "Expected warning message")
			}
		})
	}
}

func TestValidateHuggingFaceStorageURI(t *testing.T) {
	tests := []struct {
		name          string
		storageURI    string
		token         string
		expectedValid bool
		hasError      bool
	}{
		{
			name:          "Invalid HuggingFace URI format",
			storageURI:    "hf://",
			token:         "",
			expectedValid: false,
			hasError:      true,
		},
		{
			name:          "Invalid URI - not HuggingFace prefix",
			storageURI:    "oci://bucket/object",
			token:         "",
			expectedValid: false,
			hasError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHuggingFaceStorageURI(context.Background(), tt.storageURI, tt.token)
			assert.Equal(t, tt.expectedValid, result.Valid)
			if tt.hasError {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

func TestValidateStorageURIFormat(t *testing.T) {
	tests := []struct {
		name          string
		storage       *v1beta1.StorageSpec
		expectAllowed bool
	}{
		{
			name:          "Nil storage - should allow",
			storage:       nil,
			expectAllowed: true,
		},
		{
			name: "Nil storageUri - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: nil,
			},
			expectAllowed: true,
		},
		{
			name: "Non-HuggingFace URI (OCI) - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("oci://n/namespace/b/bucket/o/object"),
			},
			expectAllowed: true,
		},
		{
			name: "Non-HuggingFace URI (PVC) - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("pvc://my-pvc/subpath"),
			},
			expectAllowed: true,
		},
		{
			name: "Non-HuggingFace URI (S3) - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("s3://bucket/prefix"),
			},
			expectAllowed: true,
		},
		{
			name: "Valid HuggingFace URI format - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("hf://meta-llama/Llama-2-7b"),
			},
			expectAllowed: true,
		},
		{
			name: "Invalid HuggingFace URI format - should deny",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("hf://just-model-name"),
			},
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := validateStorageURIFormat(tt.storage, log)
			assert.Equal(t, tt.expectAllowed, response.Allowed)
		})
	}
}

func TestBaseModelValidatorHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	decoder := admission.NewDecoder(scheme)
	require.NotNil(t, decoder)

	validator := &BaseModelValidator{
		Decoder: decoder,
	}

	require.NotNil(t, validator.Decoder)
}

func TestClusterBaseModelValidatorHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	decoder := admission.NewDecoder(scheme)
	require.NotNil(t, decoder)

	validator := &ClusterBaseModelValidator{
		Decoder: decoder,
	}

	require.NotNil(t, validator.Decoder)
}

// ptr returns a pointer to the string value
func ptr(s string) *string {
	return &s
}
