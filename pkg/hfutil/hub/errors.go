package hub

import (
	"fmt"
	"net/http"
)

// HubError represents a generic Hub error
type HubError struct {
	Message string
	Cause   error
}

func (e *HubError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *HubError) Unwrap() error {
	return e.Cause
}

// HTTPError represents an HTTP error from the Hub
type HTTPError struct {
	*HubError
	StatusCode int
	Response   *http.Response
}

func NewHTTPError(message string, statusCode int, response *http.Response) *HTTPError {
	return &HTTPError{
		HubError:   &HubError{Message: message},
		StatusCode: statusCode,
		Response:   response,
	}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// RepositoryNotFoundError is raised when a repository is not found
type RepositoryNotFoundError struct {
	*HTTPError
	RepoID   string
	RepoType string
}

func NewRepositoryNotFoundError(repoID, repoType string, response *http.Response) *RepositoryNotFoundError {
	message := fmt.Sprintf("Repository '%s' not found", repoID)
	if repoType != "" && repoType != RepoTypeModel {
		message = fmt.Sprintf("%s repository '%s' not found", repoType, repoID)
	}

	statusCode := 404
	if response != nil {
		statusCode = response.StatusCode
	}

	return &RepositoryNotFoundError{
		HTTPError: NewHTTPError(message, statusCode, response),
		RepoID:    repoID,
		RepoType:  repoType,
	}
}

// GatedRepoError is raised when trying to access a gated repository
type GatedRepoError struct {
	*RepositoryNotFoundError
}

func NewGatedRepoError(repoID, repoType string, response *http.Response) *GatedRepoError {
	base := NewRepositoryNotFoundError(repoID, repoType, response)
	base.Message = fmt.Sprintf("Repository '%s' is gated and requires authentication", repoID)
	if repoType != "" && repoType != RepoTypeModel {
		base.Message = fmt.Sprintf("%s repository '%s' is gated and requires authentication", repoType, repoID)
	}

	return &GatedRepoError{
		RepositoryNotFoundError: base,
	}
}

// DisabledRepoError is raised when trying to access a disabled repository
type DisabledRepoError struct {
	*HTTPError
	RepoID   string
	RepoType string
}

func NewDisabledRepoError(repoID, repoType string, response *http.Response) *DisabledRepoError {
	message := fmt.Sprintf("Repository '%s' is disabled", repoID)
	if repoType != "" && repoType != RepoTypeModel {
		message = fmt.Sprintf("%s repository '%s' is disabled", repoType, repoID)
	}

	statusCode := 403
	if response != nil {
		statusCode = response.StatusCode
	}

	return &DisabledRepoError{
		HTTPError: NewHTTPError(message, statusCode, response),
		RepoID:    repoID,
		RepoType:  repoType,
	}
}

// RevisionNotFoundError is raised when a revision is not found
type RevisionNotFoundError struct {
	*HTTPError
	RepoID   string
	RepoType string
	Revision string
}

func NewRevisionNotFoundError(repoID, repoType, revision string, response *http.Response) *RevisionNotFoundError {
	message := fmt.Sprintf("Revision '%s' not found for repository '%s'", revision, repoID)
	if repoType != "" && repoType != RepoTypeModel {
		message = fmt.Sprintf("Revision '%s' not found for %s repository '%s'", revision, repoType, repoID)
	}

	statusCode := 404
	if response != nil {
		statusCode = response.StatusCode
	}

	return &RevisionNotFoundError{
		HTTPError: NewHTTPError(message, statusCode, response),
		RepoID:    repoID,
		RepoType:  repoType,
		Revision:  revision,
	}
}

// EntryNotFoundError is raised when a file or directory is not found
type EntryNotFoundError struct {
	*HTTPError
	RepoID   string
	RepoType string
	Revision string
	Path     string
}

func NewEntryNotFoundError(repoID, repoType, revision, path string, response *http.Response) *EntryNotFoundError {
	message := fmt.Sprintf("Entry '%s' not found in repository '%s'", path, repoID)
	if revision != "" && revision != DefaultRevision {
		message = fmt.Sprintf("Entry '%s' not found in repository '%s' at revision '%s'", path, repoID, revision)
	}
	if repoType != "" && repoType != RepoTypeModel {
		message = fmt.Sprintf("Entry '%s' not found in %s repository '%s'", path, repoType, repoID)
		if revision != "" && revision != DefaultRevision {
			message = fmt.Sprintf("Entry '%s' not found in %s repository '%s' at revision '%s'", path, repoType, repoID, revision)
		}
	}

	statusCode := 404
	if response != nil {
		statusCode = response.StatusCode
	}

	return &EntryNotFoundError{
		HTTPError: NewHTTPError(message, statusCode, response),
		RepoID:    repoID,
		RepoType:  repoType,
		Revision:  revision,
		Path:      path,
	}
}

// LocalEntryNotFoundError is raised when a file is not found locally
type LocalEntryNotFoundError struct {
	*HubError
	Path string
}

func NewLocalEntryNotFoundError(path string) *LocalEntryNotFoundError {
	return &LocalEntryNotFoundError{
		HubError: &HubError{Message: fmt.Sprintf("Entry '%s' not found locally", path)},
		Path:     path,
	}
}

// BadRequestError is raised for HTTP 400 errors
type BadRequestError struct {
	*HTTPError
}

func NewBadRequestError(message string, response *http.Response) *BadRequestError {
	return &BadRequestError{
		HTTPError: NewHTTPError(message, 400, response),
	}
}

// FileMetadataError is raised when file metadata is invalid or missing
type FileMetadataError struct {
	*HubError
	Path string
}

func NewFileMetadataError(path, message string) *FileMetadataError {
	return &FileMetadataError{
		HubError: &HubError{Message: message},
		Path:     path,
	}
}

// OfflineModeIsEnabledError is raised when offline mode is enabled but network is required
type OfflineModeIsEnabledError struct {
	*HubError
}

func NewOfflineModeIsEnabledError(message string) *OfflineModeIsEnabledError {
	return &OfflineModeIsEnabledError{
		HubError: &HubError{Message: message},
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	*HubError
	Field string
	Value interface{}
}

func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		HubError: &HubError{Message: message},
		Field:    field,
		Value:    value,
	}
}
