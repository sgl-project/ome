package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Organization represents an AI platform organization configuration
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendor"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec"`
	Status OrganizationStatus `json:"status,omitempty"`
}

// Vendor represents the vendors like OpenAI, Gemini or xAI
type Vendor string

const (
	VendorOpenAI      Vendor = "openai"
	VendorGoogle      Vendor = "google"
	VendorXAI         Vendor = "xai"
	VendorUnsupported Vendor = "unsupported"
)

type OrganizationSpec struct {
	// Vendor specifies the AI platform vendor (e.g., "openai", "anthropic")
	// +required
	Vendor *Vendor `json:"vendor"`
	// Disabled indicates whether the organization is disabled
	// +optional
	Disabled *bool `json:"disabled,omitempty"`
	// OrganizationID is the platform-specific organization ID
	// +required
	OrganizationID string `json:"organizationId"`
	// SecretRef references the secret containing the API key
	// optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type OrganizationStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Project represents an AI platform project
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="ProjectName",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="ProjectID",type="string",JSONPath=".status.projectId"
// +kubebuilder:printcolumn:name="Organization",type="string",JSONPath=".spec.organizationRef.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

type ProjectSpec struct {
	// Name is the project name
	// +required
	Name string `json:"name"`
	// Description is the project description
	Description string `json:"description,omitempty"`
	// OrganizationRef references the Organization
	// +required
	OrganizationRef CrossReference `json:"organizationRef"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type ProjectStatus struct {
	// ProjectID is the platform-specific project ID
	// +optional
	ProjectID string `json:"projectId"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the project was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
	// LastUpdatedTime is the time when the project was last updated
	LastUpdatedTime *metav1.Time `json:"lastUpdatedTime,omitempty"`
}

// ServiceAccount represents a service account within a project
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type="string",JSONPath=".spec.projectRef.name"
// +kubebuilder:printcolumn:name="KeyID",type="string",JSONPath=".status.apiKey.apiKeyId"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:storageversion
type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountSpec   `json:"spec,omitempty"`
	Status ServiceAccountStatus `json:"status,omitempty"`
}

type ServiceAccountSpec struct {
	// Name is the service account name
	// +required
	Name *string `json:"name"`
	// ProjectRef references the Project
	// +required
	ProjectRef CrossReference `json:"projectRef"`
	// Permissions defines the service account permissions
	// +optional
	Permissions []string `json:"permissions,omitempty"`
	// Role defines the service account's role in the project, owner or member
	// +optional
	Role *string `json:"role,omitempty"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type ServiceAccountStatus struct {
	// ServiceAccountID is the platform-specific service account ID
	// +optional
	ServiceAccountID *string `json:"serviceAccountId"`
	// +optional
	APIKey *APIKeySpec `json:"apiKey"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the service account was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

type APIKeySpec struct {
	// Name is the API key name
	// +required
	Name *string `json:"name"`
	// APIKeyId is the platform-specific API key ID
	// +required
	APIKeyId *string `json:"apiKeyId"`
	// APIKeySecretRef references the secret containing the API key
	// +required
	APIKeySecretRef *SecretReference `json:"apiKeySecretRef"`
}

// User represents a user within a project
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec"`
	Status UserStatus `json:"status,omitempty"`
}

type UserSpec struct {
	// Email is the user's email address
	Email string `json:"email"`
	// ProjectRef references the Project
	// +required
	ProjectRef CrossReference `json:"projectRef"`
	// Role defines the user's role in the project, owner or member
	// +optional
	Role string `json:"role"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type UserStatus struct {
	// UserID is the platform-specific user ID
	// +required
	UserID string `json:"userId"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the user was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// RateLimit represents rate limit configurations for a project
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type RateLimit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitSpec   `json:"spec"`
	Status RateLimitStatus `json:"status,omitempty"`
}

type RateLimitSpec struct {
	// ProjectRef references the Project
	// +required
	ProjectRef CrossReference `json:"projectRef"`
	// TargetRef references either a service account or user
	TargetRef CrossReference `json:"targetRef"`
	// Limits defines the rate limit configurations
	Limits []RateLimitConfig `json:"limits"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type RateLimitStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Common types used across resources

type SecretReference struct {
	// Name is the name of the secret
	// +required
	Name string `json:"name"`
	// Namespace is the namespace of the secret
	// +required
	Namespace string `json:"namespace"`
	// Key is the key in the secret
	// +required
	Key string `json:"key"`
}

type CrossReference struct {
	// Name is the name of the referenced resource
	// +required
	Name string `json:"name"`
	// Namespace is the namespace of the referenced resource (optional for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`
}

type RateLimitConfig struct {
	// Type is the type of rate limit (e.g., "requests", "tokens")
	Type string `json:"type"`
	// Limit is the maximum allowed value
	Limit int64 `json:"limit"`
	// Window is the time window for the limit (e.g., "1m", "1d")
	Window string `json:"window"`
}

// List types for all resources

// OrganizationList contains a list of Organization
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

// ProjectList contains a list of Project
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// ServiceAccountList contains a list of ServiceAccount
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// UserList contains a list of User
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// RateLimitList contains a list of RateLimit
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type RateLimitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
	SchemeBuilder.Register(&Project{}, &ProjectList{})
	SchemeBuilder.Register(&ServiceAccount{}, &ServiceAccountList{})
	SchemeBuilder.Register(&User{}, &UserList{})
	SchemeBuilder.Register(&RateLimit{}, &RateLimitList{})
}

const (
	// Common condition types
	ConditionTypeReady = "Ready"
	ConditionTypeError = "Error"
)

// ProjectStatusReason represents the status of a project operation
type ProjectStatusReason string

const (
	// ProjectStatusCreated indicates the project was successfully created
	ProjectStatusCreated ProjectStatusReason = "ProjectCreated"
	// ProjectStatusUpdated indicates the project was successfully updated
	ProjectStatusUpdated ProjectStatusReason = "ProjectUpdated"
	// ProjectStatusArchived indicates the project was successfully archived
	ProjectStatusArchived ProjectStatusReason = "ProjectArchived"
	// ProjectStatusInitError indicates an initialization error occurred
	ProjectStatusInitError ProjectStatusReason = "InitializationError"
	// ProjectStatusAPIError indicates an API error occurred
	ProjectStatusAPIError ProjectStatusReason = "APIError"
	// ProjectStatusOrgError indicates an organization-related error occurred
	ProjectStatusOrgError ProjectStatusReason = "OrganizationError"
)

// String returns the string representation of ProjectStatusReason
func (s ProjectStatusReason) String() string {
	return string(s)
}

// IsError returns true if the status represents an error condition
func (s ProjectStatusReason) IsError() bool {
	return s == ProjectStatusInitError || s == ProjectStatusAPIError || s == ProjectStatusOrgError
}

// IsSuccess returns true if the status represents a success condition
func (s ProjectStatusReason) IsSuccess() bool {
	return s == ProjectStatusCreated || s == ProjectStatusUpdated || s == ProjectStatusArchived
}

// ServiceAccountStatusReason represents the status of a service account operation
type ServiceAccountStatusReason string

const (
	// ServiceAccountStatusCreated indicates the service account was successfully created
	ServiceAccountStatusCreated ServiceAccountStatusReason = "ServiceAccountCreated"
	// ServiceAccountStatusDeleted indicates the service account was successfully deleted
	ServiceAccountStatusDeleted ServiceAccountStatusReason = "ServiceAccountDeleted"
	// ServiceAccountStatusProjectError indicates a project-related error occurred
	ServiceAccountStatusProjectError ServiceAccountStatusReason = "ProjectError"
	// ServiceAccountStatusInitError indicates an initialization error occurred
	ServiceAccountStatusInitError ServiceAccountStatusReason = "InitializationError"
	// ServiceAccountStatusAPIError indicates an API error occurred
	ServiceAccountStatusAPIError ServiceAccountStatusReason = "APIError"
	// ServiceAccountStatusSecretError indicates a secret-related error occurred
	ServiceAccountStatusSecretError ServiceAccountStatusReason = "SecretError"
	// ServiceAccountStatusConfigError indicates a configuration error occurred
	ServiceAccountStatusConfigError ServiceAccountStatusReason = "ConfigError"
)

// String returns the string representation of ServiceAccountStatusReason
func (s ServiceAccountStatusReason) String() string {
	return string(s)
}

// IsError returns true if the status represents an error condition
func (s ServiceAccountStatusReason) IsError() bool {
	return s == ServiceAccountStatusProjectError ||
		s == ServiceAccountStatusInitError ||
		s == ServiceAccountStatusAPIError ||
		s == ServiceAccountStatusSecretError ||
		s == ServiceAccountStatusConfigError
}

// IsSuccess returns true if the status represents a success condition
func (s ServiceAccountStatusReason) IsSuccess() bool {
	return s == ServiceAccountStatusCreated || s == ServiceAccountStatusDeleted
}
