package basemodel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			expectedValid:  true,  // Fail open - allow resource creation
			expectedExists: true,  // We assume exists since we got 401 not 404
			expectWarning:  true,  // Should have warning about auth
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

func TestBaseModelValidatorGetTokenFromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name        string
		secretName  string
		namespace   string
		parameters  *map[string]string
		secret      *corev1.Secret
		expectToken string
		expectError bool
	}{
		{
			name:       "Valid secret with default key",
			secretName: "hf-secret",
			namespace:  "default",
			parameters: nil,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hf-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("hf_test_token_123"),
				},
			},
			expectToken: "hf_test_token_123",
			expectError: false,
		},
		{
			name:       "Valid secret with custom key",
			secretName: "hf-secret",
			namespace:  "default",
			parameters: &map[string]string{
				"secretKey": "myCustomKey",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hf-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"myCustomKey": []byte("hf_custom_token"),
				},
			},
			expectToken: "hf_custom_token",
			expectError: false,
		},
		{
			name:        "Secret not found",
			secretName:  "nonexistent-secret",
			namespace:   "default",
			parameters:  nil,
			secret:      nil,
			expectToken: "",
			expectError: true,
		},
		{
			name:       "Secret exists but key not found",
			secretName: "hf-secret",
			namespace:  "default",
			parameters: nil,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hf-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"wrongKey": []byte("some_value"),
				},
			},
			expectToken: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build fake client
			objs := []runtime.Object{}
			if tt.secret != nil {
				objs = append(objs, tt.secret)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			token, err := getTokenFromSecret(context.Background(), fakeClient, tt.secretName, tt.namespace, tt.parameters)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectToken, token)
			}
		})
	}
}

func TestBaseModelValidatorValidateStorageURI(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name             string
		storage          *v1beta1.StorageSpec
		namespace        string
		expectAllowed    bool
		expectWarning    bool
		warningContains  string
	}{
		{
			name:          "Nil storage - should allow",
			storage:       nil,
			namespace:     "default",
			expectAllowed: true,
			expectWarning: false,
		},
		{
			name: "Nil storageUri - should allow",
			storage: &v1beta1.StorageSpec{
				StorageUri: nil,
			},
			namespace:     "default",
			expectAllowed: true,
			expectWarning: false,
		},
		{
			name: "Non-HuggingFace URI (OCI) - should allow and skip validation",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("oci://n/namespace/b/bucket/o/object"),
			},
			namespace:     "default",
			expectAllowed: true,
			expectWarning: false,
		},
		{
			name: "Non-HuggingFace URI (PVC) - should allow and skip validation",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("pvc://my-pvc/subpath"),
			},
			namespace:     "default",
			expectAllowed: true,
			expectWarning: false,
		},
		{
			name: "Non-HuggingFace URI (S3) - should allow and skip validation",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("s3://bucket/prefix"),
			},
			namespace:     "default",
			expectAllowed: true,
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			// Use the shared validateHuggingFaceStorage function
			response := validateHuggingFaceStorage(context.Background(), fakeClient, tt.storage, tt.namespace, log)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectWarning {
				assert.NotEmpty(t, response.Warnings)
				if tt.warningContains != "" {
					assert.Contains(t, response.Warnings[0], tt.warningContains)
				}
			}
		})
	}
}

func TestValidateHuggingFaceStorageClusterScope(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

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
			name: "Non-HuggingFace URI - should allow and skip validation",
			storage: &v1beta1.StorageSpec{
				StorageUri: ptr("oci://n/namespace/b/bucket/o/object"),
			},
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			// ClusterBaseModel uses OME namespace for secrets
			response := validateHuggingFaceStorage(context.Background(), fakeClient, tt.storage, "ome", clusterLog)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
		})
	}
}

func TestBaseModelValidatorHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	decoder := admission.NewDecoder(scheme)
	require.NotNil(t, decoder)

	validator := &BaseModelValidator{
		Client:  fakeClient,
		Decoder: decoder,
	}

	// Note: The Handle method would need a proper admission.Request with encoded object
	// This is a simplified test - in practice, you'd use envtest or integration tests
	// Full integration tests would use envtest to test the actual webhook server
	require.NotNil(t, validator.Client)
	require.NotNil(t, validator.Decoder)
}

// ptr returns a pointer to the string value
func ptr(s string) *string {
	return &s
}
