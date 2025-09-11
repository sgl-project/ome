package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistryConfig defines MCP tool, resource, and prompt registry management.
type RegistryConfig struct {
	// Tools defines tool virtualization and management capabilities.
	// +optional
	Tools *MCPToolConfig `json:"tools,omitempty"`

	// Resources defines MCP resource management and access.
	// +optional
	Resources *MCPResourceConfig `json:"resources,omitempty"`

	// Prompts defines MCP prompt template management.
	// +optional
	Prompts *MCPPromptConfig `json:"prompts,omitempty"`
}

// PromptTemplate defines a prompt template.
type PromptTemplate struct {
	// Name is the template identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Content is the prompt template content.
	// +kubebuilder:validation:Required
	Content string `json:"content"`

	// Description provides a human-readable description of the prompt.
	// +optional
	Description string `json:"description,omitempty"`

	// Parameters define template parameters.
	// +optional
	// +listType=atomic
	Parameters []PromptParameter `json:"parameters,omitempty"`
}

// PromptParameter defines a prompt template parameter.
type PromptParameter struct {
	// Name is the parameter name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type defines the parameter type.
	// +kubebuilder:validation:Enum=string;number;boolean;array;object
	// +kubebuilder:default=string
	// +optional
	Type string `json:"type,omitempty"`

	// Required indicates if the parameter is required.
	// +kubebuilder:default=false
	// +optional
	Required *bool `json:"required,omitempty"`

	// Description provides a human-readable description of the parameter.
	// +optional
	Description string `json:"description,omitempty"`
}

// PromptRegistryConfig defines prompt registry configuration.
type PromptRegistryConfig struct {
	// Enabled controls whether the prompt registry is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AutoRegister enables automatic prompt registration from connected servers.
	// +kubebuilder:default=true
	// +optional
	AutoRegister *bool `json:"autoRegister,omitempty"`
}

// MCPToolConfig defines tool virtualization and management capabilities.
type MCPToolConfig struct {
	// Virtualization enables REST API to MCP tool conversion.
	// +kubebuilder:default=true
	// +optional
	Virtualization *bool `json:"virtualization,omitempty"`

	// RestTools define REST APIs to expose as MCP tools.
	// +optional
	// +listType=atomic
	RestTools []RestToolConfig `json:"restTools,omitempty"`

	// Registry defines tool registry configuration.
	// +optional
	Registry *ToolRegistryConfig `json:"registry,omitempty"`
}

// RestToolConfig defines a REST API exposed as an MCP tool.
type RestToolConfig struct {
	// Name is the tool identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// URL is the REST API endpoint URL.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Method defines the HTTP method to use.
	// +kubebuilder:validation:Enum=GET;POST;PUT;DELETE;PATCH
	// +kubebuilder:default=POST
	// +optional
	Method string `json:"method,omitempty"`

	// Headers define additional HTTP headers to send.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`

	// InputSchema defines the JSON schema for tool input.
	// +optional
	InputSchema *string `json:"inputSchema,omitempty"`

	// Description provides a human-readable description of the tool.
	// +optional
	Description string `json:"description,omitempty"`

	// Auth defines authentication for the REST API.
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`
}

// RestAuthType defines REST tool authentication types.
type RestAuthType string

const (
	RestAuthTypeBearer RestAuthType = "Bearer"
	RestAuthTypeApiKey RestAuthType = "ApiKey"
	RestAuthTypeBasic  RestAuthType = "Basic"
)

// ToolRegistryConfig defines tool registry configuration.
type ToolRegistryConfig struct {
	// Enabled controls whether the tool registry is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AutoRegister enables automatic tool registration from connected servers.
	// +kubebuilder:default=true
	// +optional
	AutoRegister *bool `json:"autoRegister,omitempty"`
}

// OrchestrationWorkflowStorageConfig defines workflow state storage.
type WorkflowStorageConfig struct {
	// Type defines the storage backend type.
	// +kubebuilder:validation:Enum=Memory;Redis;Database
	// +kubebuilder:default=Memory
	// +optional
	Type WorkflowStorageType `json:"type,omitempty"`

	// TTL defines how long to keep workflow state.
	// +kubebuilder:default="1h"
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`
}

// WorkflowStorageType defines workflow storage types.
type WorkflowStorageType string

const (
	WorkflowStorageTypeMemory   WorkflowStorageType = "Memory"
	WorkflowStorageTypeRedis    WorkflowStorageType = "Redis"
	WorkflowStorageTypeDatabase WorkflowStorageType = "Database"
)

// MCPResourceConfig defines MCP resource management and access.
type MCPResourceConfig struct {
	// AutoSync enables automatic resource synchronization from connected servers.
	// +kubebuilder:default=true
	// +optional
	AutoSync *bool `json:"autoSync,omitempty"`

	// Storage defines resource storage configuration.
	// +optional
	Storage *ResourceStorageConfig `json:"storage,omitempty"`

	// Cache defines resource caching configuration.
	// +optional
	Cache *ResourceCacheConfig `json:"cache,omitempty"`
}

// ResourceStorageConfig defines resource storage configuration.
type ResourceStorageConfig struct {
	// Type defines the storage backend type.
	// +kubebuilder:validation:Enum=Memory;File;S3;Database
	// +kubebuilder:default=Memory
	// +optional
	Type ResourceStorageType `json:"type,omitempty"`

	// Path defines the file system path for file storage.
	// +optional
	Path string `json:"path,omitempty"`
}

// ResourceStorageType defines resource storage types.
type ResourceStorageType string

const (
	ResourceStorageTypeMemory        ResourceStorageType = "Memory"
	ResourceStorageTypeFile          ResourceStorageType = "File"
	ResourceStorageTypeObjectStorage ResourceStorageType = "ObjectStorage"
	ResourceStorageTypeDatabase      ResourceStorageType = "Database"
)

// ResourceCacheConfig defines resource caching configuration.
type ResourceCacheConfig struct {
	// Enabled controls whether resource caching is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// TTL defines the cache time-to-live.
	// +kubebuilder:default="5m"
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// MaxSize defines the maximum cache size.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`
}

// MCPPromptConfig defines MCP prompt template management.
type MCPPromptConfig struct {
	// AutoSync enables automatic prompt synchronization from connected servers.
	// +kubebuilder:default=true
	// +optional
	AutoSync *bool `json:"autoSync,omitempty"`

	// Templates define static prompt templates.
	// +optional
	// +listType=atomic
	Templates []PromptTemplate `json:"templates,omitempty"`

	// Registry defines prompt registry configuration.
	// +optional
	Registry *PromptRegistryConfig `json:"registry,omitempty"`
}
