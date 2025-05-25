package modelagent

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelAgentHealthCheck_Name(t *testing.T) {
	healthCheck := NewModelAgentHealthCheck("/some/path")
	assert.Equal(t, "model-agent-health", healthCheck.Name())
}

func TestModelAgentHealthCheck_Check(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "model-agent-health-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name          string
		setupFunc     func() string
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid directory",
			setupFunc: func() string {
				return tempDir
			},
			expectedError: false,
		},
		{
			name: "Directory doesn't exist",
			setupFunc: func() string {
				nonExistentDir := filepath.Join(tempDir, "does-not-exist")
				return nonExistentDir
			},
			expectedError: true,
			errorContains: "no such file or directory",
		},
		{
			name: "Not a directory but a file",
			setupFunc: func() string {
				filePath := filepath.Join(tempDir, "test-file")
				err := os.WriteFile(filePath, []byte("test content"), 0644)
				require.NoError(t, err)
				return filePath
			},
			expectedError: true,
			errorContains: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc()

			healthCheck := NewModelAgentHealthCheck(path)
			err := healthCheck.Check(nil)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestModelAgentHealthCheck_CheckWithRequest(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "model-agent-health-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock HTTP request
	req, err := http.NewRequest("GET", "/healthz", nil)
	require.NoError(t, err)

	// Test that the request is properly ignored (the parameter is unused in the implementation)
	healthCheck := NewModelAgentHealthCheck(tempDir)
	err = healthCheck.Check(req)
	assert.NoError(t, err)
}
