package oci

import (
	"testing"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func TestOCIStorage_Provider(t *testing.T) {
	// Note: This is a unit test that doesn't require actual OCI credentials
	s := &OCIStorage{
		logger: logging.NewNopLogger(),
	}

	if provider := s.Provider(); provider != storage.ProviderOCI {
		t.Errorf("Expected provider %s, got %s", storage.ProviderOCI, provider)
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Not found error",
			err:      &mockError{message: "NotFound"},
			expected: true,
		},
		{
			name:     "not found lowercase",
			err:      &mockError{message: "not found"},
			expected: true,
		},
		{
			name:     "404 error",
			err:      &mockError{message: "404"},
			expected: true,
		},
		{
			name:     "Other error",
			err:      &mockError{message: "Internal Server Error"},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}
