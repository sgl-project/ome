package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Organization represents an AI platform organization configuration
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec"`
	Status OrganizationStatus `json:"status,omitempty"`
}

type OrganizationSpec struct {
	// Vendor specifies the AI platform vendor (e.g., "openai", "anthropic")
	Vendor string `json:"vendor"`
	// OrganizationID is the platform-specific organization ID
	OrganizationID string `json:"organizationId"`
	// SecretRef references the secret containing the API key
	SecretRef SecretReference `json:"secretRef"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type OrganizationStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Project represents an AI platform project
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

type ProjectSpec struct {
	// Name is the project name
	Name string `json:"name"`
	// Description is the project description
	Description string `json:"description,omitempty"`
	// OrganizationRef references the Organization
	OrganizationRef CrossReference `json:"organizationRef"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type ProjectStatus struct {
	// ProjectID is the platform-specific project ID
	ProjectID string `json:"projectId,omitempty"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the project was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
	// ArchivalTime is the time when the project was archived
	ArchivalTime *metav1.Time `json:"archivalTime,omitempty"`
}

// ServiceAccount represents a service account within a project
// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountSpec   `json:"spec"`
	Status ServiceAccountStatus `json:"status,omitempty"`
}

type ServiceAccountSpec struct {
	// Name is the service account name
	Name string `json:"name"`
	// ProjectRef references the Project
	ProjectRef CrossReference `json:"projectRef"`
	// Permissions defines the service account permissions
	Permissions []string `json:"permissions,omitempty"`
	// Role defines the service account's role in the project, owner or member
	Role string `json:"role"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type ServiceAccountStatus struct {
	// ServiceAccountID is the platform-specific service account ID
	ServiceAccountID string `json:"serviceAccountId,omitempty"`
	// APIKeySecretRef references the secret containing the API key
	APIKeySecretRef *SecretReference `json:"apiKeySecretRef,omitempty"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the service account was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// User represents a user within a project
// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
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
	ProjectRef CrossReference `json:"projectRef"`
	// Role defines the user's role in the project, owner or member
	Role string `json:"role"`
	// Config contains vendor-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type UserStatus struct {
	// UserID is the platform-specific user ID
	UserID string `json:"userId,omitempty"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CreationTime is the time when the user was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// RateLimit represents rate limit configurations for a project
// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RateLimit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitSpec   `json:"spec"`
	Status RateLimitStatus `json:"status,omitempty"`
}

type RateLimitSpec struct {
	// ProjectRef references the Project
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
	Name string `json:"name"`
	// Namespace is the namespace of the secret
	Namespace string `json:"namespace"`
	// Key is the key in the secret
	Key string `json:"key"`
}

type CrossReference struct {
	// Name is the name of the referenced resource
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RateLimitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimit `json:"items"`
}
