package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHubError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		cause    error
		expected string
	}{
		{
			name:     "error without cause",
			message:  "test error message",
			cause:    nil,
			expected: "test error message",
		},
		{
			name:     "error with cause",
			message:  "wrapper error",
			cause:    assert.AnError,
			expected: "wrapper error: assert.AnError general error for testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HubError{
				Message: tt.message,
				Cause:   tt.cause,
			}

			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.cause, err.Unwrap())
		})
	}
}

func TestNewHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	httpErr := NewHTTPError("custom message", 404, resp)

	assert.NotNil(t, httpErr)
	assert.Equal(t, "HTTP 404: custom message", httpErr.Error())
	assert.Equal(t, 404, httpErr.StatusCode)
	assert.Equal(t, resp, httpErr.Response)
}

func TestNewRepositoryNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		expected string
	}{
		{
			name:     "model repository",
			repoID:   "microsoft/DialoGPT-medium",
			repoType: RepoTypeModel,
			expected: "Repository 'microsoft/DialoGPT-medium' not found",
		},
		{
			name:     "dataset repository",
			repoID:   "squad",
			repoType: RepoTypeDataset,
			expected: "dataset repository 'squad' not found",
		},
		{
			name:     "space repository",
			repoID:   "gradio/hello_world",
			repoType: RepoTypeSpace,
			expected: "space repository 'gradio/hello_world' not found",
		},
		{
			name:     "empty repo type defaults to model",
			repoID:   "test/repo",
			repoType: "",
			expected: "Repository 'test/repo' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRepositoryNotFoundError(tt.repoID, tt.repoType, nil)

			assert.Contains(t, err.Error(), tt.expected)
			assert.Equal(t, tt.repoID, err.RepoID)
			assert.Equal(t, tt.repoType, err.RepoType)
			assert.Equal(t, 404, err.StatusCode)
		})
	}
}

func TestNewRepositoryNotFoundErrorWithResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	repoErr := NewRepositoryNotFoundError("test/repo", RepoTypeModel, resp)
	assert.Equal(t, 401, repoErr.StatusCode) // Should use response status code
	assert.Equal(t, resp, repoErr.Response)
}

func TestNewGatedRepoError(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		expected string
	}{
		{
			name:     "gated model",
			repoID:   "meta-llama/Llama-2-7b-hf",
			repoType: RepoTypeModel,
			expected: "Repository 'meta-llama/Llama-2-7b-hf' is gated and requires authentication",
		},
		{
			name:     "gated dataset",
			repoID:   "private/dataset",
			repoType: RepoTypeDataset,
			expected: "dataset repository 'private/dataset' is gated and requires authentication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewGatedRepoError(tt.repoID, tt.repoType, nil)

			assert.Contains(t, err.Error(), tt.expected)
			assert.Equal(t, tt.repoID, err.RepoID)
			assert.Equal(t, tt.repoType, err.RepoType)
		})
	}
}

func TestNewDisabledRepoError(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		expected string
	}{
		{
			name:     "disabled model",
			repoID:   "disabled/model",
			repoType: RepoTypeModel,
			expected: "Repository 'disabled/model' is disabled",
		},
		{
			name:     "disabled space",
			repoID:   "disabled/space",
			repoType: RepoTypeSpace,
			expected: "space repository 'disabled/space' is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewDisabledRepoError(tt.repoID, tt.repoType, nil)

			assert.Contains(t, err.Error(), tt.expected)
			assert.Equal(t, tt.repoID, err.RepoID)
			assert.Equal(t, tt.repoType, err.RepoType)
			assert.Equal(t, 403, err.StatusCode)
		})
	}
}

func TestNewRevisionNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		revision string
		expected string
	}{
		{
			name:     "model with revision",
			repoID:   "microsoft/DialoGPT-medium",
			repoType: RepoTypeModel,
			revision: "v1.0",
			expected: "Revision 'v1.0' not found for repository 'microsoft/DialoGPT-medium'",
		},
		{
			name:     "dataset with revision",
			repoID:   "squad",
			repoType: RepoTypeDataset,
			revision: "v2.0",
			expected: "Revision 'v2.0' not found for dataset repository 'squad'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRevisionNotFoundError(tt.repoID, tt.repoType, tt.revision, nil)

			assert.Contains(t, err.Error(), tt.expected)
			assert.Equal(t, tt.repoID, err.RepoID)
			assert.Equal(t, tt.repoType, err.RepoType)
			assert.Equal(t, tt.revision, err.Revision)
			assert.Equal(t, 404, err.StatusCode)
		})
	}
}

func TestNewEntryNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		revision string
		path     string
		expected string
	}{
		{
			name:     "file not found in model",
			repoID:   "microsoft/DialoGPT-medium",
			repoType: RepoTypeModel,
			revision: DefaultRevision,
			path:     "missing.json",
			expected: "Entry 'missing.json' not found in repository 'microsoft/DialoGPT-medium'",
		},
		{
			name:     "file not found with custom revision",
			repoID:   "test/repo",
			repoType: RepoTypeModel,
			revision: "v1.0",
			path:     "config.json",
			expected: "Entry 'config.json' not found in repository 'test/repo' at revision 'v1.0'",
		},
		{
			name:     "file not found in dataset",
			repoID:   "squad",
			repoType: RepoTypeDataset,
			revision: DefaultRevision,
			path:     "train.json",
			expected: "Entry 'train.json' not found in dataset repository 'squad'",
		},
		{
			name:     "file not found in space with revision",
			repoID:   "gradio/hello",
			repoType: RepoTypeSpace,
			revision: "v2.0",
			path:     "app.py",
			expected: "Entry 'app.py' not found in space repository 'gradio/hello' at revision 'v2.0'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewEntryNotFoundError(tt.repoID, tt.repoType, tt.revision, tt.path, nil)

			assert.Contains(t, err.Error(), tt.expected)
			assert.Equal(t, tt.repoID, err.RepoID)
			assert.Equal(t, tt.repoType, err.RepoType)
			assert.Equal(t, tt.revision, err.Revision)
			assert.Equal(t, tt.path, err.Path)
			assert.Equal(t, 404, err.StatusCode)
		})
	}
}

func TestNewLocalEntryNotFoundError(t *testing.T) {
	path := "/path/to/missing/file.json"
	err := NewLocalEntryNotFoundError(path)

	expectedMessage := "Entry '/path/to/missing/file.json' not found locally"
	assert.Equal(t, expectedMessage, err.Error())
	assert.Equal(t, path, err.Path)
}

func TestNewBadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	badReqErr := NewBadRequestError("invalid request", resp)

	assert.Equal(t, "HTTP 400: invalid request", badReqErr.Error())
	assert.Equal(t, 400, badReqErr.StatusCode)
	assert.Equal(t, resp, badReqErr.Response)
}

func TestNewFileMetadataError(t *testing.T) {
	path := "model.bin"
	message := "file metadata is corrupted"

	err := NewFileMetadataError(path, message)

	assert.Equal(t, message, err.Error())
	assert.Equal(t, path, err.Path)
}

func TestNewOfflineModeIsEnabledError(t *testing.T) {
	message := "Cannot access remote files in offline mode"
	err := NewOfflineModeIsEnabledError(message)

	assert.Equal(t, message, err.Error())
}

func TestNewValidationError(t *testing.T) {
	field := "token"
	value := ""
	message := "token cannot be empty"

	err := NewValidationError(field, value, message)

	assert.Equal(t, message, err.Error())
	assert.Equal(t, field, err.Field)
	assert.Equal(t, value, err.Value)
}

// Test error type assertions and unwrapping
func TestErrorTypeAssertions(t *testing.T) {
	// Test that we can assert to correct types
	var err error

	// Repository not found error
	err = NewRepositoryNotFoundError("test/repo", RepoTypeModel, nil)
	_, ok := err.(*RepositoryNotFoundError)
	assert.True(t, ok)

	// Gated repo error should be its own type
	err = NewGatedRepoError("test/repo", RepoTypeModel, nil)
	gatedErr, ok := err.(*GatedRepoError)
	assert.True(t, ok)
	// But should have access to RepositoryNotFoundError fields through embedding
	assert.NotNil(t, gatedErr.RepositoryNotFoundError)
	assert.Equal(t, "test/repo", gatedErr.RepoID)

	// HTTP error
	err = NewHTTPError("test", 500, nil)
	_, ok = err.(*HTTPError)
	assert.True(t, ok)

	// Hub error (base type)
	err = &HubError{Message: "test"}
	_, ok = err.(*HubError)
	assert.True(t, ok)
}

// Test error inheritance hierarchy
func TestErrorInheritance(t *testing.T) {
	// GatedRepoError embeds RepositoryNotFoundError
	gatedErr := NewGatedRepoError("test/repo", RepoTypeModel, nil)

	// Should be able to access embedded fields
	assert.Equal(t, "test/repo", gatedErr.RepoID)
	assert.Equal(t, RepoTypeModel, gatedErr.RepoType)
	assert.NotNil(t, gatedErr.RepositoryNotFoundError)
	assert.NotNil(t, gatedErr.HTTPError)

	// DisabledRepoError should embed HTTPError
	disabledErr := NewDisabledRepoError("test/repo", RepoTypeModel, nil)
	assert.Equal(t, "test/repo", disabledErr.RepoID)
	assert.Equal(t, RepoTypeModel, disabledErr.RepoType)
	assert.NotNil(t, disabledErr.HTTPError)
}

// Test with actual HTTP responses
func TestErrorsWithHTTPResponses(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectedType interface{}
	}{
		{
			name:         "404 creates RepositoryNotFoundError",
			statusCode:   404,
			expectedType: &RepositoryNotFoundError{},
		},
		{
			name:         "403 creates GatedRepoError",
			statusCode:   403,
			expectedType: &GatedRepoError{},
		},
		{
			name:         "400 creates BadRequestError",
			statusCode:   400,
			expectedType: &BadRequestError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			var hubErr error
			switch tt.statusCode {
			case 404:
				_ = NewRepositoryNotFoundError("test/repo", RepoTypeModel, nil)
				hubErr = NewRepositoryNotFoundError("test/repo", RepoTypeModel, resp)
			case 403:
				hubErr = NewGatedRepoError("test/repo", RepoTypeModel, resp)
			case 400:
				_ = NewHTTPError("test message", 404, nil)
				hubErr = NewBadRequestError("test message", resp)
			}

			assert.IsType(t, tt.expectedType, hubErr)
		})
	}
}

// Test error message formatting consistency
func TestErrorMessageFormatting(t *testing.T) {
	tests := []struct {
		name   string
		error  error
		checks []string // substrings that should be in the message
	}{
		{
			name:   "repository not found",
			error:  NewRepositoryNotFoundError("user/repo", RepoTypeModel, nil),
			checks: []string{"Repository", "user/repo", "not found"},
		},
		{
			name:   "gated repository",
			error:  NewGatedRepoError("user/repo", RepoTypeModel, nil),
			checks: []string{"Repository", "user/repo", "gated", "authentication"},
		},
		{
			name:   "entry not found",
			error:  NewEntryNotFoundError("user/repo", RepoTypeModel, "main", "file.json", nil),
			checks: []string{"Entry", "file.json", "not found", "user/repo"},
		},
		{
			name:   "local entry not found",
			error:  NewLocalEntryNotFoundError("/path/to/file"),
			checks: []string{"Entry", "/path/to/file", "not found locally"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := tt.error.Error()
			for _, check := range tt.checks {
				assert.Contains(t, message, check, "Error message should contain '%s'", check)
			}
		})
	}
}

// Benchmark error creation
func BenchmarkNewRepositoryNotFoundError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewRepositoryNotFoundError("test/repo", RepoTypeModel, nil)
	}
}

func BenchmarkNewHTTPError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewHTTPError("test message", 404, nil)
	}
}

func BenchmarkErrorMessage(b *testing.B) {
	err := NewRepositoryNotFoundError("user/repository-name", RepoTypeDataset, nil)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}
