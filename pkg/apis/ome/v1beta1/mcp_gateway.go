package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPGatewaySpec defines the desired state of MCPGateway.
// MCPGateway provides AI-aware routing, context management, and orchestration
// capabilities for Model Context Protocol (MCP) servers with federation support.
// +kubebuilder:validation:XValidation:rule="has(self.mcpServers.static) || has(self.mcpServers.selector)", message="either static MCP server references or dynamic selector must be specified"
// +kubebuilder:validation:XValidation:rule="!has(self.federation.enabled) || !self.federation.enabled || size(self.federation.peers) > 0", message="federation peers required when federation is enabled"
type MCPGatewaySpec struct {
	// Replicas is the number of desired replicas for the gateway.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// PodSpec defines the pod template for the gateway deployment.
	// +optional
	PodSpec *corev1.PodTemplateSpec `json:"podSpec,omitempty"`

	// MCPServers defines how the gateway discovers and connects to MCP servers.
	// +kubebuilder:validation:Required
	MCPServers MCPServerDiscoveryConfig `json:"mcpServers"`

	// Federation defines automatic peer discovery and coordination for distributed MCP access.
	// automatic discovery, metadata exchange, and registry synchronization.
	// +optional
	Federation *FederationConfig `json:"federation,omitempty"`

	// Transport defines the supported transport protocols for MCP communication.
	// +optional
	Transport *MCPTransportType `json:"transport,omitempty"`

	// Memory defines context and memory management with bucket support.
	// +optional
	Memory *MemoryBucketConfig `json:"memory,omitempty"`

	// Registry defines MCP tool, resource, and prompt registry management.
	// +optional
	Registry *RegistryConfig `json:"registry,omitempty"`

	// Routing defines how requests are routed to different MCP servers.
	// +optional
	Routing *RoutingConfig `json:"routing,omitempty"`

	// Policy defines unified security, authentication, authorization, and traffic policies.
	// +optional
	Policy *GatewayPolicyConfig `json:"policy,omitempty"`

	// Observability defines monitoring, metrics, and tracing configuration.
	// +optional
	Observability *ObservabilityConfig `json:"observability,omitempty"`

	// Network defines service exposure and ingress settings.
	// +optional
	Network *GatewayNetworkConfig `json:"network,omitempty"`

	// Orchestration defines tool selection strategies and workflow orchestration support.
	// +optional
	Orchestration *OrchestrationConfig `json:"orchestration,omitempty"`

	// SessionContext defines context storage and session management policies.
	// +optional
	SessionContext *SessionContextConfig `json:"sessionContext,omitempty"`
}

// MCPServerDiscoveryConfig defines how the gateway discovers and connects to MCP servers.
type MCPServerDiscoveryConfig struct {
	// Static provides a fixed list of MCP server references.
	// +optional
	Static []MCPServerRef `json:"static,omitempty"`

	// Selector allows dynamic discovery of MCPServer resources using a label selector.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// NamespaceSelector restricts server discovery to specific namespaces.
	// Only applicable when using Selector. If empty, searches all accessible namespaces.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// AutoDiscovery enables automatic discovery of MCP servers through federation.
	// +kubebuilder:default=true
	// +optional
	AutoDiscovery *bool `json:"autoDiscovery,omitempty"`

	// HealthCheck defines health checking configuration for discovered servers.
	// +optional
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`
}

// MCPServerRef defines a reference to an upstream MCP server with routing parameters.
type MCPServerRef struct {
	// Name of the referenced MCPServer resource.
	Name string `json:"name"`

	// Namespace of the referenced MCPServer resource.
	// If empty, assumes the gateway's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Weight for traffic distribution among servers (0-100).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=100
	// +optional
	Weight *int32 `json:"weight,omitempty"`

	// Priority for server selection (lower value is higher priority).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// Tags define semantic tags for routing decisions.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`

	// Capabilities override the server's advertised capabilities for routing.
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`

	// Auth defines the credentials for this specific server.
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`

	// Transport override for this specific server.
	// +optional
	Transport *MCPTransportType `json:"transport,omitempty"`
}

// HealthCheckConfig defines health checking configuration.
type HealthCheckConfig struct {
	// Enabled controls whether health checking is performed.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Interval is the time between health checks.
	// +kubebuilder:default="30s"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Timeout is the maximum time to wait for a health check response.
	// +kubebuilder:default="5s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// UnhealthyThreshold is the number of consecutive failures before marking unhealthy.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	UnhealthyThreshold *int32 `json:"unhealthyThreshold,omitempty"`

	// HealthyThreshold is the number of consecutive successes before marking healthy.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	// +optional
	HealthyThreshold *int32 `json:"healthyThreshold,omitempty"`
}

// FederationConfig defines automatic peer discovery and coordination for distributed MCP access.
type FederationConfig struct {
	// Enabled controls whether federation is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Discovery defines how to automatically discover peer gateways in Kubernetes.
	// +optional
	Discovery *FederationDiscoveryConfig `json:"discovery,omitempty"`

	// Registry defines how to synchronize tool and resource registries with peers.
	// +optional
	Registry *FederationRegistryConfig `json:"registry,omitempty"`

	// Communication defines peer-to-peer communication settings.
	// +optional
	Communication *FederationCommunicationConfig `json:"communication,omitempty"`
}

// FederationDiscoveryConfig defines Kubernetes-based automatic peer discovery.
type FederationDiscoveryConfig struct {
	// LabelSelector defines labels to identify peer gateway services.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// NamespaceSelector restricts discovery to specific namespaces.
	// If empty, searches the current namespace only.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// ServicePort defines the port to use for peer communication.
	// +kubebuilder:default=8080
	// +optional
	ServicePort *int32 `json:"servicePort,omitempty"`

	// WatchEnabled enables watching for dynamic peer changes.
	// +kubebuilder:default=true
	// +optional
	WatchEnabled *bool `json:"watchEnabled,omitempty"`

	// Interval defines how often to perform peer discovery.
	// +kubebuilder:default="30s"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`
}

// FederationRegistryConfig defines how to synchronize tool and resource registries with peers.
type FederationRegistryConfig struct {
	// SyncInterval defines how often to synchronize registries with peers.
	// +kubebuilder:default="60s"
	// +optional
	SyncInterval *metav1.Duration `json:"syncInterval,omitempty"`

	// Tools controls whether to synchronize tool registries.
	// +kubebuilder:default=true
	// +optional
	Tools *bool `json:"tools,omitempty"`

	// Resources controls whether to synchronize resource registries.
	// +kubebuilder:default=true
	// +optional
	Resources *bool `json:"resources,omitempty"`

	// Prompts controls whether to synchronize prompt registries.
	// +kubebuilder:default=true
	// +optional
	Prompts *bool `json:"prompts,omitempty"`
}

// FederationCommunicationConfig defines peer-to-peer communication settings.
type FederationCommunicationConfig struct {
	// TLS defines TLS settings for peer communication.
	// +kubebuilder:default=true
	// +optional
	TLS *bool `json:"tls,omitempty"`

	// Auth defines authentication settings for peer communication.
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`
}

// MemoryBucket defines an organized memory storage bucket.
type MemoryBucket struct {
	// Name is the bucket identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description provides a human-readable description of the bucket.
	// +optional
	Description string `json:"description,omitempty"`

	// TTL defines the time-to-live for entries in this bucket.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// MaxEntries defines the maximum number of entries in this bucket.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxEntries *int32 `json:"maxEntries,omitempty"`

	// AccessControl defines who can access this bucket.
	// +optional
	AccessControl *BucketAccessControl `json:"accessControl,omitempty"`
}

// BucketAccessControl defines access control for memory buckets.
type BucketAccessControl struct {
	// AllowedUsers defines users that can access this bucket.
	// +optional
	// +listType=set
	AllowedUsers []string `json:"allowedUsers,omitempty"`

	// AllowedTenants defines tenants that can access this bucket.
	// +optional
	// +listType=set
	AllowedTenants []string `json:"allowedTenants,omitempty"`
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

// GatewayPolicyConfig defines unified security, authentication, authorization, and traffic policies.
type GatewayPolicyConfig struct {
	// Authentication defines client authentication configuration.
	// +optional
	Authentication *MCPAuthenticationConfig `json:"authentication,omitempty"`

	// TLS defines TLS configuration for the gateway.
	// +optional
	TLS *GatewayTLSConfig `json:"tls,omitempty"`

	// RateLimit defines rate limiting configuration.
	// +optional
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"`

	// Audit defines audit logging configuration.
	// +optional
	Audit *AuditConfig `json:"audit,omitempty"`

	// RequestFiltering defines request filtering policies.
	// +optional
	RequestFiltering *RequestFilteringConfig `json:"requestFiltering,omitempty"`

	// ResponseFiltering defines response filtering policies.
	// +optional
	ResponseFiltering *ResponseFilteringConfig `json:"responseFiltering,omitempty"`

	// Compliance defines compliance-related policies.
	// +optional
	Compliance *ComplianceConfig `json:"compliance,omitempty"`
}

// MemoryBucketConfig defines context and memory management with bucket support.
type MemoryBucketConfig struct {
	// Storage defines the storage backend for memory and context.
	// +optional
	Storage *MemoryStorageConfig `json:"storage,omitempty"`

	// Buckets define organized memory storage buckets.
	// +optional
	// +listType=atomic
	Buckets []MemoryBucket `json:"buckets,omitempty"`

	// DefaultTTL defines the default time-to-live for memory entries.
	// +kubebuilder:default="1h"
	// +optional
	DefaultTTL *metav1.Duration `json:"defaultTTL,omitempty"`

	// MaxSize defines the maximum memory storage size.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`
}

// SessionContextConfig defines context storage and session management policies.
type SessionContextConfig struct {
	// Storage defines where and how context is persisted.
	// +optional
	Storage *ContextStorageConfig `json:"storage,omitempty"`

	// Sessions define session management policies.
	// +optional
	Sessions *SessionConfig `json:"sessions,omitempty"`

	// Isolation defines context isolation policies for multi-tenancy.
	// +optional
	Isolation *ContextIsolationConfig `json:"isolation,omitempty"`

	// Persistence defines context persistence policies.
	// +optional
	Persistence *ContextPersistenceConfig `json:"persistence,omitempty"`
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

// MCPAuthenticationConfig defines simplified client authentication configuration.
type MCPAuthenticationConfig struct {
	// Enabled controls whether authentication is required.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Methods define the supported authentication methods in order of preference.
	// +optional
	// +listType=atomic
	Methods []AuthConfig `json:"methods,omitempty"`

	// Default provides the default authentication method when none is specified.
	// +optional
	Default *AuthConfig `json:"default,omitempty"`
}

// RoutingConfig defines how requests are routed to different MCP servers.
// It supports rule-based routing, load balancing, and session affinity for stateful connections.
type RoutingConfig struct {
	// Rules is a list of routing rules that are evaluated in order of priority.
	// The first rule that matches a request will be used.
	// +optional
	// +listType=atomic
	Rules []RoutingRule `json:"rules,omitempty"`

	// DefaultTarget defines the routing behavior for requests that do not match any rules.
	// If not specified, a default load balancing strategy is used across all available servers.
	// +optional
	DefaultTarget *RouteTarget `json:"defaultTarget,omitempty"`

	// SessionAffinity ensures that requests from the same client session are consistently
	// routed to the same MCP server, which is crucial for stateful operations.
	// This setting applies to all traffic unless overridden by a specific routing rule.
	// +optional
	SessionAffinity *SessionAffinityConfig `json:"sessionAffinity,omitempty"`

	// CircuitBreaker defines circuit breaker configuration.
	// +optional
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`

	// Fallback defines fallback policies when primary servers fail.
	// +optional
	Fallback *FallbackConfig `json:"fallback,omitempty"`
}

// RoutingRule defines a condition for routing traffic to a specific target.
type RoutingRule struct {
	// Name provides a human-readable identifier for the rule.
	// +optional
	Name string `json:"name,omitempty"`

	// Priority determines the order of evaluation (higher values are evaluated first).
	// Rules with the same priority are evaluated in the order they appear in the list.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Match defines the conditions under which this rule applies.
	// A rule matches if any of its match conditions are met.
	// +kubebuilder:validation:Required
	Match RequestMatch `json:"match"`

	// Target defines where and how to route the traffic that matches this rule.
	// +kubebuilder:validation:Required
	Target RouteTarget `json:"target"`
}

// RequestMatch defines the criteria for matching an incoming request.
// At least one of the fields must be specified.
// +kubebuilder:validation:XValidation:rule="has(self.tool) || has(self.intent) || has(self.headers)", message="at least one match criteria must be specified"
type RequestMatch struct {
	// Tool specifies a match based on the requested tool.
	// +optional
	Tool *ToolMatch `json:"tool,omitempty"`

	// Intent specifies a match based on the semantic intent of the request.
	// Requires a semantic analysis model to be configured.
	// +optional
	Intent *IntentMatch `json:"intent,omitempty"`

	// Headers specifies a match based on request headers.
	// The map key is the header name and the map value is the header value.
	// The rule matches if all specified headers are present and their values match.
	// +optional
	// +listType=map
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`
}

// ToolMatch defines a match based on a tool name.
type ToolMatch struct {
	// Pattern is a glob pattern to match against the tool name.
	// For example, "image-generation-*" or "summarize".
	// +kubebuilder:validation:Required
	Pattern string `json:"pattern"`
}

// IntentMatch defines a match based on semantic intent.
type IntentMatch struct {
	// Intent is the specific intent to match (e.g., "customer_support_query", "code_generation").
	// +kubebuilder:validation:Required
	Intent string `json:"intent"`

	// ConfidenceThreshold is the minimum confidence score required for the intent to be considered a match.
	// If not set, a gateway-wide default is used.
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +optional
	ConfidenceThreshold *float64 `json:"confidenceThreshold,omitempty"`
}

// RouteTarget defines the destination for routed traffic.
type RouteTarget struct {
	// Servers defines the pool of MCP servers to which traffic should be routed.
	// +kubebuilder:validation:Required
	Servers ServerSelection `json:"servers"`

	// LoadBalancing defines the strategy for distributing traffic among the selected servers.
	// +optional
	LoadBalancing *LoadBalancingStrategy `json:"loadBalancing,omitempty"`

	// SessionAffinity can be used to override the global session affinity policy for this specific route.
	// +optional
	SessionAffinity *SessionAffinityConfig `json:"sessionAffinity,omitempty"`
}

// ServerSelection defines how to select a group of MCP servers.
// Exactly one field must be specified.
// +kubebuilder:validation:XValidation:rule="has(self.refs) || has(self.selector) || has(self.tags)", message="exactly one of refs, selector, or tags must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.refs) && has(self.selector)) && !(has(self.refs) && has(self.tags)) && !(has(self.selector) && has(self.tags))", message="only one of refs, selector, or tags can be specified"
type ServerSelection struct {
	// Refs explicitly lists servers by name. The names must correspond to entries
	// in the gateway's server discovery list.
	// +optional
	// +listType=atomic
	Refs []string `json:"refs,omitempty"`

	// Selector uses a label selector to dynamically choose servers.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Tags matches servers that have been configured with specific semantic tags.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`
}

// LoadBalancingStrategy defines the load balancing policy.
type LoadBalancingStrategy struct {
	// Algorithm specifies how to distribute traffic across the target servers.
	// +kubebuilder:validation:Enum=RoundRobin;WeightedRoundRobin;LeastConnections;ResourceBased
	// +kubebuilder:default=WeightedRoundRobin
	// +optional
	Algorithm LoadBalancingAlgorithm `json:"algorithm,omitempty"`

	// ResourceBased configures resource-based load balancing if the algorithm is 'ResourceBased'.
	// +optional
	ResourceBased *ResourceBasedLoadBalancing `json:"resourceBased,omitempty"`
}

// LoadBalancingAlgorithm defines supported load balancing algorithms.
type LoadBalancingAlgorithm string

const (
	LoadBalancingAlgorithmRoundRobin         LoadBalancingAlgorithm = "RoundRobin"
	LoadBalancingAlgorithmWeightedRoundRobin LoadBalancingAlgorithm = "WeightedRoundRobin"
	LoadBalancingAlgorithmLeastConnections   LoadBalancingAlgorithm = "LeastConnections"
	LoadBalancingAlgorithmResourceBased      LoadBalancingAlgorithm = "ResourceBased"
)

// SessionAffinityConfig defines session affinity configuration to ensure stateful connections.
type SessionAffinityConfig struct {
	// Type defines the session affinity method.
	// +kubebuilder:validation:Enum=None;ClientIP;Cookie;Header
	// +kubebuilder:default=None
	// +optional
	Type SessionAffinityType `json:"type,omitempty"`

	// CookieName specifies the cookie name for cookie-based affinity.
	// Required if Type is 'Cookie'.
	// +optional
	CookieName string `json:"cookieName,omitempty"`

	// HeaderName specifies the header name for header-based affinity.
	// Required if Type is 'Header'.
	// +optional
	HeaderName string `json:"headerName,omitempty"`

	// TTL specifies the session affinity timeout. After this period of inactivity,
	// the affinity may be broken.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`
}

// SessionAffinityType defines session affinity types.
type SessionAffinityType string

const (
	SessionAffinityTypeNone     SessionAffinityType = "None"
	SessionAffinityTypeClientIP SessionAffinityType = "ClientIP"
	SessionAffinityTypeCookie   SessionAffinityType = "Cookie"
	SessionAffinityTypeHeader   SessionAffinityType = "Header"
)

// ResourceBasedLoadBalancing defines resource-based load balancing.
type ResourceBasedLoadBalancing struct {
	// Metrics define which resource metrics to consider for load balancing decisions.
	// +optional
	// +listType=set
	Metrics []ResourceMetric `json:"metrics,omitempty"`

	// UpdateInterval defines how often to update resource metrics from servers.
	// +kubebuilder:default="30s"
	// +optional
	UpdateInterval *metav1.Duration `json:"updateInterval,omitempty"`
}

// ResourceMetric defines supported resource metrics for load balancing.
type ResourceMetric string

const (
	ResourceMetricCPU     ResourceMetric = "CPU"
	ResourceMetricMemory  ResourceMetric = "Memory"
	ResourceMetricLatency ResourceMetric = "Latency"
	ResourceMetricErrors  ResourceMetric = "Errors"
)

// FallbackConfig defines fallback policies when primary servers fail.
type FallbackConfig struct {
	// Enabled controls whether fallback is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Strategy defines the fallback strategy.
	// +kubebuilder:validation:Enum=NextAvailable;Backup;Degraded;Fail
	// +kubebuilder:default=NextAvailable
	// +optional
	Strategy FallbackStrategy `json:"strategy,omitempty"`

	// BackupServers define dedicated backup servers for fallback.
	// +optional
	// +listType=atomic
	BackupServers []MCPServerRef `json:"backupServers,omitempty"`

	// DegradedMode defines behavior when operating in degraded mode.
	// +optional
	DegradedMode *DegradedModeConfig `json:"degradedMode,omitempty"`
}

// FallbackStrategy defines fallback strategies.
type FallbackStrategy string

const (
	FallbackStrategyNextAvailable FallbackStrategy = "NextAvailable"
	FallbackStrategyBackup        FallbackStrategy = "Backup"
	FallbackStrategyDegraded      FallbackStrategy = "Degraded"
	FallbackStrategyFail          FallbackStrategy = "Fail"
)

// DegradedModeConfig defines degraded mode behavior.
type DegradedModeConfig struct {
	// MaxLatency defines maximum acceptable latency in degraded mode.
	// +optional
	MaxLatency *metav1.Duration `json:"maxLatency,omitempty"`

	// ReducedCapabilities defines which capabilities to disable.
	// +optional
	// +listType=set
	ReducedCapabilities []string `json:"reducedCapabilities,omitempty"`
}

// ContextStorageConfig defines context storage configuration.
type ContextStorageConfig struct {
	// Type defines the storage backend type.
	// +kubebuilder:validation:Enum=Memory;Redis;Database;File
	// +kubebuilder:default=Memory
	// +optional
	Type ContextStorageType `json:"type,omitempty"`

	// Memory defines in-memory storage configuration.
	// +optional
	Memory *MemoryStorageConfig `json:"memory,omitempty"`

	// Redis defines Redis storage configuration.
	// +optional
	Redis *RedisStorageConfig `json:"redis,omitempty"`

	// Database defines database storage configuration.
	// +optional
	Database *DatabaseStorageConfig `json:"database,omitempty"`

	// File defines file-based storage configuration.
	// +optional
	File *FileStorageConfig `json:"file,omitempty"`
}

// ContextStorageType defines context storage types.
type ContextStorageType string

const (
	ContextStorageTypeMemory   ContextStorageType = "Memory"
	ContextStorageTypeRedis    ContextStorageType = "Redis"
	ContextStorageTypeDatabase ContextStorageType = "Database"
	ContextStorageTypeFile     ContextStorageType = "File"
)

// MemoryStorageConfig defines in-memory storage configuration.
type MemoryStorageConfig struct {
	// MaxSize defines maximum memory usage for context storage.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// EvictionPolicy defines how to evict contexts when memory is full.
	// +kubebuilder:validation:Enum=LRU;LFU;TTL
	// +kubebuilder:default=LRU
	// +optional
	EvictionPolicy MemoryEvictionPolicy `json:"evictionPolicy,omitempty"`
}

// MemoryEvictionPolicy defines memory eviction policies.
type MemoryEvictionPolicy string

const (
	MemoryEvictionPolicyLRU MemoryEvictionPolicy = "LRU"
	MemoryEvictionPolicyLFU MemoryEvictionPolicy = "LFU"
	MemoryEvictionPolicyTTL MemoryEvictionPolicy = "TTL"
)

// RedisStorageConfig defines Redis storage configuration.
type RedisStorageConfig struct {
	// Address specifies the Redis server address.
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// Database specifies the Redis database number.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	Database *int32 `json:"database,omitempty"`

	// Auth defines Redis authentication.
	// +optional
	Auth *RedisAuthConfig `json:"auth,omitempty"`

	// TLS defines Redis TLS configuration.
	// +optional
	TLS *RedisTLSConfig `json:"tls,omitempty"`

	// Pool defines Redis connection pooling.
	// +optional
	Pool *RedisPoolConfig `json:"pool,omitempty"`
}

// RedisAuthConfig defines Redis authentication.
type RedisAuthConfig struct {
	// PasswordSecretRef references a secret containing the Redis password.
	// +optional
	PasswordSecretRef *corev1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// Username specifies the Redis username.
	// +optional
	Username string `json:"username,omitempty"`
}

// RedisTLSConfig defines Redis TLS configuration.
type RedisTLSConfig struct {
	// Enabled controls whether TLS is used.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// InsecureSkipVerify skips certificate verification.
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

	// CertSecretRef references a secret containing TLS certificates.
	// +optional
	CertSecretRef *corev1.SecretKeySelector `json:"certSecretRef,omitempty"`
}

// RedisPoolConfig defines Redis connection pooling.
type RedisPoolConfig struct {
	// MinIdleConnections defines the minimum number of idle connections.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=2
	// +optional
	MinIdleConnections *int32 `json:"minIdleConnections,omitempty"`

	// MaxConnections defines the maximum number of connections.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`
}

// DatabaseStorageConfig defines database storage configuration.
type DatabaseStorageConfig struct {
	// Type defines the database type.
	// +kubebuilder:validation:Enum=PostgreSQL;MySQL;SQLite
	// +kubebuilder:default=PostgreSQL
	// +optional
	Type DatabaseType `json:"type,omitempty"`

	// ConnectionString defines the database connection string.
	// +kubebuilder:validation:Required
	ConnectionString string `json:"connectionString"`

	// ConnectionSecretRef references a secret containing database credentials.
	// +optional
	ConnectionSecretRef *corev1.SecretKeySelector `json:"connectionSecretRef,omitempty"`

	// MaxConnections defines the maximum number of database connections.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`
}

// DatabaseType defines supported database types.
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "PostgreSQL"
	DatabaseTypeMySQL      DatabaseType = "MySQL"
	DatabaseTypeSQLite     DatabaseType = "SQLite"
)

// FileStorageConfig defines file-based storage configuration.
type FileStorageConfig struct {
	// Path defines the file system path for context storage.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// VolumeSource defines the volume source for persistent storage.
	// +optional
	VolumeSource *corev1.VolumeSource `json:"volumeSource,omitempty"`
}

// SessionConfig defines session management policies.
type SessionConfig struct {
	// DefaultTTL defines the default session time-to-live.
	// +kubebuilder:default="1h"
	// +optional
	DefaultTTL *metav1.Duration `json:"defaultTTL,omitempty"`

	// MaxTTL defines the maximum allowed session time-to-live.
	// +kubebuilder:default="24h"
	// +optional
	MaxTTL *metav1.Duration `json:"maxTTL,omitempty"`

	// IdleTimeout defines when to expire idle sessions.
	// +kubebuilder:default="15m"
	// +optional
	IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`

	// MaxConcurrentSessions defines the maximum concurrent sessions per user.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxConcurrentSessions *int32 `json:"maxConcurrentSessions,omitempty"`
}

// ContextIsolationConfig defines context isolation policies for multi-tenancy.
type ContextIsolationConfig struct {
	// Level defines the isolation level.
	// +kubebuilder:validation:Enum=None;User;Tenant;Namespace
	// +kubebuilder:default=User
	// +optional
	Level ContextIsolationLevel `json:"level,omitempty"`

	// TenantKey defines how to extract tenant information from requests.
	// +optional
	TenantKey *TenantKeyConfig `json:"tenantKey,omitempty"`

	// Encryption defines whether to encrypt context data.
	// +optional
	Encryption *ContextEncryptionConfig `json:"encryption,omitempty"`
}

// ContextIsolationLevel defines context isolation levels.
type ContextIsolationLevel string

const (
	ContextIsolationLevelNone      ContextIsolationLevel = "None"
	ContextIsolationLevelUser      ContextIsolationLevel = "User"
	ContextIsolationLevelTenant    ContextIsolationLevel = "Tenant"
	ContextIsolationLevelNamespace ContextIsolationLevel = "Namespace"
)

// TenantKeyConfig defines how to extract tenant information.
type TenantKeyConfig struct {
	// Source defines where to find tenant information.
	// +kubebuilder:validation:Enum=Header;JWT;Certificate;Query
	// +kubebuilder:default=Header
	// +optional
	Source TenantKeySource `json:"source,omitempty"`

	// Name specifies the header/query parameter/JWT claim name.
	// +optional
	Name string `json:"name,omitempty"`
}

// TenantKeySource defines tenant key sources.
type TenantKeySource string

const (
	TenantKeySourceHeader      TenantKeySource = "Header"
	TenantKeySourceJWT         TenantKeySource = "JWT"
	TenantKeySourceCertificate TenantKeySource = "Certificate"
	TenantKeySourceQuery       TenantKeySource = "Query"
)

// ContextEncryptionConfig defines context encryption configuration.
type ContextEncryptionConfig struct {
	// Enabled controls whether context encryption is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Algorithm defines the encryption algorithm.
	// +kubebuilder:validation:Enum=AES-256-GCM;ChaCha20-Poly1305
	// +kubebuilder:default=AES-256-GCM
	// +optional
	Algorithm EncryptionAlgorithm `json:"algorithm,omitempty"`

	// KeySecretRef references a secret containing encryption keys.
	// +optional
	KeySecretRef *corev1.SecretKeySelector `json:"keySecretRef,omitempty"`
}

// EncryptionAlgorithm defines supported encryption algorithms.
type EncryptionAlgorithm string

const (
	EncryptionAlgorithmAES256GCM        EncryptionAlgorithm = "AES-256-GCM"
	EncryptionAlgorithmChaCha20Poly1305 EncryptionAlgorithm = "ChaCha20-Poly1305"
)

// ContextPersistenceConfig defines context persistence policies.
type ContextPersistenceConfig struct {
	// Strategy defines the persistence strategy.
	// +kubebuilder:validation:Enum=None;Session;Workflow;Permanent
	// +kubebuilder:default=Session
	// +optional
	Strategy ContextPersistenceStrategy `json:"strategy,omitempty"`

	// WorkflowTTL defines how long to persist workflow contexts.
	// +kubebuilder:default="7d"
	// +optional
	WorkflowTTL *metav1.Duration `json:"workflowTTL,omitempty"`

	// MaxContextSize defines the maximum size of a persisted context.
	// +optional
	MaxContextSize *resource.Quantity `json:"maxContextSize,omitempty"`
}

// ContextPersistenceStrategy defines context persistence strategies.
type ContextPersistenceStrategy string

const (
	ContextPersistenceStrategyNone      ContextPersistenceStrategy = "None"
	ContextPersistenceStrategySession   ContextPersistenceStrategy = "Session"
	ContextPersistenceStrategyWorkflow  ContextPersistenceStrategy = "Workflow"
	ContextPersistenceStrategyPermanent ContextPersistenceStrategy = "Permanent"
)

// AuthorizationPolicies defines authorization policy sources.
type AuthorizationPolicies struct {
	// ConfigMapRefs reference ConfigMaps containing policies.
	// +optional
	// +listType=atomic
	ConfigMapRefs []corev1.ConfigMapKeySelector `json:"configMapRefs,omitempty"`

	// Inline contains embedded policy definitions.
	// +optional
	Inline *InlineAuthzConfig `json:"inline,omitempty"`

	// External references external policy sources.
	// +optional
	External *ExternalAuthzConfig `json:"external,omitempty"`
}

// ExternalAuthzConfig defines external authorization configuration.
type ExternalAuthzConfig struct {
	// Endpoint defines the external authorization service endpoint.
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// Timeout defines the authorization request timeout.
	// +kubebuilder:default="5s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Headers define additional headers to send with authorization requests.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`
}

// PermissionMappingConfig defines permission mapping configuration.
type PermissionMappingConfig struct {
	// UserClaim defines the JWT claim containing the user identifier.
	// +kubebuilder:default="sub"
	// +optional
	UserClaim string `json:"userClaim,omitempty"`

	// RoleClaim defines the JWT claim containing user roles.
	// +kubebuilder:default="roles"
	// +optional
	RoleClaim string `json:"roleClaim,omitempty"`

	// TenantClaim defines the JWT claim containing tenant information.
	// +kubebuilder:default="tenant"
	// +optional
	TenantClaim string `json:"tenantClaim,omitempty"`

	// StaticMappings define static user-to-role mappings.
	// +optional
	// +mapType=atomic
	StaticMappings map[string][]string `json:"staticMappings,omitempty"`
}

// MultiTenancyConfig defines multi-tenant isolation policies.
type MultiTenancyConfig struct {
	// Enabled controls whether multi-tenancy is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Strategy defines the multi-tenancy strategy.
	// +kubebuilder:validation:Enum=Namespace;Label;Header;JWT
	// +kubebuilder:default=Header
	// +optional
	Strategy MultiTenancyStrategy `json:"strategy,omitempty"`

	// TenantExtraction defines how to extract tenant information.
	// +optional
	TenantExtraction *TenantExtractionConfig `json:"tenantExtraction,omitempty"`

	// Isolation defines tenant isolation policies.
	// +optional
	Isolation *TenantIsolationConfig `json:"isolation,omitempty"`
}

// MultiTenancyStrategy defines multi-tenancy strategies.
type MultiTenancyStrategy string

const (
	MultiTenancyStrategyNamespace MultiTenancyStrategy = "Namespace"
	MultiTenancyStrategyLabel     MultiTenancyStrategy = "Label"
	MultiTenancyStrategyHeader    MultiTenancyStrategy = "Header"
	MultiTenancyStrategyJWT       MultiTenancyStrategy = "JWT"
)

// TenantExtractionConfig defines tenant extraction configuration.
type TenantExtractionConfig struct {
	// HeaderName defines the header containing tenant information.
	// +optional
	HeaderName string `json:"headerName,omitempty"`

	// JWTClaim defines the JWT claim containing tenant information.
	// +optional
	JWTClaim string `json:"jwtClaim,omitempty"`

	// LabelKey defines the label key for tenant identification.
	// +optional
	LabelKey string `json:"labelKey,omitempty"`
}

// TenantIsolationConfig defines tenant isolation configuration.
type TenantIsolationConfig struct {
	// NetworkIsolation controls whether to isolate tenants at the network level.
	// +kubebuilder:default=false
	// +optional
	NetworkIsolation *bool `json:"networkIsolation,omitempty"`

	// ContextIsolation controls whether to isolate tenant contexts.
	// +kubebuilder:default=true
	// +optional
	ContextIsolation *bool `json:"contextIsolation,omitempty"`

	// ServerIsolation controls whether to isolate tenant access to servers.
	// +kubebuilder:default=false
	// +optional
	ServerIsolation *bool `json:"serverIsolation,omitempty"`
}

// GatewayTLSConfig defines TLS configuration for the gateway.
type GatewayTLSConfig struct {
	// Enabled controls whether TLS is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// CertSecretRef references a secret containing TLS certificates.
	// +optional
	CertSecretRef *corev1.SecretKeySelector `json:"certSecretRef,omitempty"`

	// MinVersion defines the minimum TLS version.
	// +kubebuilder:validation:Enum=TLS1.0;TLS1.1;TLS1.2;TLS1.3
	// +kubebuilder:default=TLS1.2
	// +optional
	MinVersion TLSVersion `json:"minVersion,omitempty"`

	// CipherSuites define allowed cipher suites.
	// +optional
	// +listType=set
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

// TLSVersion defines supported TLS versions.
type TLSVersion string

const (
	TLSVersion10 TLSVersion = "TLS1.0"
	TLSVersion11 TLSVersion = "TLS1.1"
	TLSVersion12 TLSVersion = "TLS1.2"
	TLSVersion13 TLSVersion = "TLS1.3"
)

// AuditConfig defines audit logging configuration.
type AuditConfig struct {
	// Enabled controls whether audit logging is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Level defines the audit logging level.
	// +kubebuilder:validation:Enum=None;Request;Response;Full
	// +kubebuilder:default=Request
	// +optional
	Level AuditLevel `json:"level,omitempty"`

	// Destination defines where audit logs are sent.
	// +optional
	Destination *AuditDestinationConfig `json:"destination,omitempty"`

	// Format defines the audit log format.
	// +kubebuilder:validation:Enum=JSON;CEF;SYSLOG
	// +kubebuilder:default=JSON
	// +optional
	Format AuditFormat `json:"format,omitempty"`

	// IncludeMetadata controls whether to include request metadata in audit logs.
	// +kubebuilder:default=true
	// +optional
	IncludeMetadata *bool `json:"includeMetadata,omitempty"`
}

// AuditLevel defines audit logging levels.
type AuditLevel string

const (
	AuditLevelNone     AuditLevel = "None"
	AuditLevelRequest  AuditLevel = "Request"
	AuditLevelResponse AuditLevel = "Response"
	AuditLevelFull     AuditLevel = "Full"
)

// AuditFormat defines audit log formats.
type AuditFormat string

const (
	AuditFormatJSON   AuditFormat = "JSON"
	AuditFormatCEF    AuditFormat = "CEF"
	AuditFormatSyslog AuditFormat = "SYSLOG"
)

// AuditDestinationConfig defines audit log destinations.
type AuditDestinationConfig struct {
	// Type defines the destination type.
	// +kubebuilder:validation:Enum=File;Syslog;HTTP;S3;Database
	// +kubebuilder:default=File
	// +optional
	Type AuditDestinationType `json:"type,omitempty"`

	// File defines file-based audit logging.
	// +optional
	File *AuditFileConfig `json:"file,omitempty"`

	// HTTP defines HTTP-based audit logging.
	// +optional
	HTTP *AuditHTTPConfig `json:"http,omitempty"`

	// Syslog defines syslog-based audit logging.
	// +optional
	Syslog *AuditSyslogConfig `json:"syslog,omitempty"`
}

// AuditDestinationType defines audit destination types.
type AuditDestinationType string

const (
	AuditDestinationTypeFile     AuditDestinationType = "File"
	AuditDestinationTypeSyslog   AuditDestinationType = "Syslog"
	AuditDestinationTypeHTTP     AuditDestinationType = "HTTP"
	AuditDestinationTypeS3       AuditDestinationType = "S3"
	AuditDestinationTypeDatabase AuditDestinationType = "Database"
)

// AuditFileConfig defines file-based audit logging.
type AuditFileConfig struct {
	// Path defines the audit log file path.
	// +kubebuilder:default="/var/log/mcpgateway/audit.log"
	// +optional
	Path string `json:"path,omitempty"`

	// MaxSize defines the maximum log file size before rotation.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MaxBackups defines the maximum number of backup files to keep.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxBackups *int32 `json:"maxBackups,omitempty"`
}

// AuditHTTPConfig defines HTTP-based audit logging.
type AuditHTTPConfig struct {
	// Endpoint defines the HTTP endpoint for audit logs.
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// Method defines the HTTP method to use.
	// +kubebuilder:validation:Enum=POST;PUT
	// +kubebuilder:default=POST
	// +optional
	Method string `json:"method,omitempty"`

	// Headers define additional HTTP headers.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`

	// Timeout defines the HTTP request timeout.
	// +kubebuilder:default="30s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// AuditSyslogConfig defines syslog-based audit logging.
type AuditSyslogConfig struct {
	// Server defines the syslog server address.
	// +kubebuilder:validation:Required
	Server string `json:"server"`

	// Protocol defines the syslog protocol.
	// +kubebuilder:validation:Enum=UDP;TCP;TLS
	// +kubebuilder:default=UDP
	// +optional
	Protocol SyslogProtocol `json:"protocol,omitempty"`

	// Facility defines the syslog facility.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=23
	// +kubebuilder:default=16
	// +optional
	Facility *int32 `json:"facility,omitempty"`
}

// SyslogProtocol defines syslog protocols.
type SyslogProtocol string

const (
	SyslogProtocolUDP SyslogProtocol = "UDP"
	SyslogProtocolTCP SyslogProtocol = "TCP"
	SyslogProtocolTLS SyslogProtocol = "TLS"
)

// RequestFilteringConfig defines request filtering policies.
type RequestFilteringConfig struct {
	// SizeLimit defines maximum request size.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`

	// ContentTypeFilter defines allowed content types.
	// +optional
	// +listType=set
	ContentTypeFilter []string `json:"contentTypeFilter,omitempty"`

	// HeaderFilters define header filtering rules.
	// +optional
	// +listType=atomic
	HeaderFilters []HeaderFilter `json:"headerFilters,omitempty"`

	// BodyFilters define body content filtering rules.
	// +optional
	// +listType=atomic
	BodyFilters []BodyFilter `json:"bodyFilters,omitempty"`
}

// HeaderFilter defines header filtering rules.
type HeaderFilter struct {
	// Name is the header name to filter.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Action defines the filtering action.
	// +kubebuilder:validation:Enum=Allow;Deny;Remove;Redact
	// +kubebuilder:validation:Required
	Action FilterAction `json:"action"`

	// Pattern is a regex pattern to match header values.
	// +optional
	Pattern string `json:"pattern,omitempty"`
}

// BodyFilter defines body content filtering rules.
type BodyFilter struct {
	// Type defines the filter type.
	// +kubebuilder:validation:Enum=Regex;JSONPath;Size
	// +kubebuilder:validation:Required
	Type BodyFilterType `json:"type"`

	// Pattern defines the pattern to match.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Action defines the filtering action.
	// +kubebuilder:validation:Enum=Allow;Deny;Remove;Redact
	// +kubebuilder:validation:Required
	Action FilterAction `json:"action"`

	// Replacement defines the replacement value for redaction.
	// +optional
	Replacement string `json:"replacement,omitempty"`
}

// FilterAction defines filtering actions.
type FilterAction string

const (
	FilterActionAllow  FilterAction = "Allow"
	FilterActionDeny   FilterAction = "Deny"
	FilterActionRemove FilterAction = "Remove"
	FilterActionRedact FilterAction = "Redact"
)

// BodyFilterType defines body filter types.
type BodyFilterType string

const (
	BodyFilterTypeRegex    BodyFilterType = "Regex"
	BodyFilterTypeJSONPath BodyFilterType = "JSONPath"
	BodyFilterTypeSize     BodyFilterType = "Size"
)

// ResponseFilteringConfig defines response filtering policies.
type ResponseFilteringConfig struct {
	// SizeLimit defines maximum response size.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`

	// HeaderFilters define response header filtering rules.
	// +optional
	// +listType=atomic
	HeaderFilters []HeaderFilter `json:"headerFilters,omitempty"`

	// BodyFilters define response body filtering rules.
	// +optional
	// +listType=atomic
	BodyFilters []BodyFilter `json:"bodyFilters,omitempty"`

	// RemoveInternalHeaders controls whether to remove internal headers.
	// +kubebuilder:default=true
	// +optional
	RemoveInternalHeaders *bool `json:"removeInternalHeaders,omitempty"`
}

// ComplianceConfig defines compliance-related policies.
type ComplianceConfig struct {
	// DataRetention defines data retention policies.
	// +optional
	DataRetention *DataRetentionConfig `json:"dataRetention,omitempty"`

	// PIIDetection defines PII detection and handling.
	// +optional
	PIIDetection *PIIDetectionConfig `json:"piiDetection,omitempty"`

	// Encryption defines encryption requirements.
	// +optional
	Encryption *ComplianceEncryptionConfig `json:"encryption,omitempty"`
}

// DataRetentionConfig defines data retention policies.
type DataRetentionConfig struct {
	// AuditLogRetention defines how long to keep audit logs.
	// +kubebuilder:default="90d"
	// +optional
	AuditLogRetention *metav1.Duration `json:"auditLogRetention,omitempty"`

	// ContextRetention defines how long to keep context data.
	// +kubebuilder:default="30d"
	// +optional
	ContextRetention *metav1.Duration `json:"contextRetention,omitempty"`

	// SessionRetention defines how long to keep session data.
	// +kubebuilder:default="7d"
	// +optional
	SessionRetention *metav1.Duration `json:"sessionRetention,omitempty"`
}

// PIIDetectionConfig defines PII detection and handling.
type PIIDetectionConfig struct {
	// Enabled controls whether PII detection is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Patterns define PII detection patterns.
	// +optional
	// +listType=atomic
	Patterns []PIIPattern `json:"patterns,omitempty"`

	// Action defines the action to take when PII is detected.
	// +kubebuilder:validation:Enum=Log;Block;Redact
	// +kubebuilder:default=Log
	// +optional
	Action PIIAction `json:"action,omitempty"`
}

// PIIPattern defines a PII detection pattern.
type PIIPattern struct {
	// Type defines the PII type.
	// +kubebuilder:validation:Enum=SSN;CreditCard;Email;Phone;Custom
	// +kubebuilder:validation:Required
	Type PIIType `json:"type"`

	// Pattern is a regex pattern for custom PII detection.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Description provides a human-readable description.
	// +optional
	Description string `json:"description,omitempty"`
}

// PIIType defines PII types.
type PIIType string

const (
	PIITypeSSN        PIIType = "SSN"
	PIITypeCreditCard PIIType = "CreditCard"
	PIITypeEmail      PIIType = "Email"
	PIITypePhone      PIIType = "Phone"
	PIITypeCustom     PIIType = "Custom"
)

// PIIAction defines PII handling actions.
type PIIAction string

const (
	PIIActionLog    PIIAction = "Log"
	PIIActionBlock  PIIAction = "Block"
	PIIActionRedact PIIAction = "Redact"
)

// ComplianceEncryptionConfig defines compliance encryption configuration.
type ComplianceEncryptionConfig struct {
	// RequireEncryption controls whether encryption is required.
	// +kubebuilder:default=false
	// +optional
	RequireEncryption *bool `json:"requireEncryption,omitempty"`

	// EncryptionAtRest controls whether data must be encrypted at rest.
	// +kubebuilder:default=false
	// +optional
	EncryptionAtRest *bool `json:"encryptionAtRest,omitempty"`

	// EncryptionInTransit controls whether data must be encrypted in transit.
	// +kubebuilder:default=true
	// +optional
	EncryptionInTransit *bool `json:"encryptionInTransit,omitempty"`
}

// OrchestrationConfig defines tool selection strategies and workflow orchestration support.
type OrchestrationConfig struct {
	// Enabled controls whether orchestration is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ToolSelection defines tool selection strategies.
	// +optional
	ToolSelection *ToolSelectionConfig `json:"toolSelection,omitempty"`

	// MaxSteps defines the maximum number of steps in a workflow.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxSteps *int32 `json:"maxSteps,omitempty"`

	// StepTimeout defines the timeout for individual workflow steps.
	// +kubebuilder:default="5m"
	// +optional
	StepTimeout *metav1.Duration `json:"stepTimeout,omitempty"`

	// WorkflowTimeout defines the overall workflow timeout.
	// +kubebuilder:default="30m"
	// +optional
	WorkflowTimeout *metav1.Duration `json:"workflowTimeout,omitempty"`

	// Storage defines workflow state storage configuration.
	// +optional
	Storage *WorkflowStorageConfig `json:"storage,omitempty"`

	// Engine defines the workflow engine to use.
	// +kubebuilder:validation:Enum=Simple;Temporal;Argo
	// +kubebuilder:default=Simple
	// +optional
	Engine WorkflowEngine `json:"engine,omitempty"`
}

// ToolSelectionConfig defines tool selection strategies.
type ToolSelectionConfig struct {
	// Strategy defines the tool selection strategy.
	// +kubebuilder:validation:Enum=FirstMatch;BestMatch;Consensus;AIGuided
	// +kubebuilder:default=BestMatch
	// +optional
	Strategy ToolSelectionStrategy `json:"strategy,omitempty"`

	// CapabilityMatching defines how to match tool capabilities.
	// +optional
	CapabilityMatching *CapabilityMatchingConfig `json:"capabilityMatching,omitempty"`

	// Consensus defines consensus-based tool selection.
	// +optional
	Consensus *ConsensusConfig `json:"consensus,omitempty"`
}

// ToolSelectionStrategy defines tool selection strategies.
type ToolSelectionStrategy string

const (
	ToolSelectionStrategyFirstMatch ToolSelectionStrategy = "FirstMatch"
	ToolSelectionStrategyBestMatch  ToolSelectionStrategy = "BestMatch"
	ToolSelectionStrategyConsensus  ToolSelectionStrategy = "Consensus"
	ToolSelectionStrategyAIGuided   ToolSelectionStrategy = "AIGuided"
)

// CapabilityMatchingConfig defines capability matching configuration.
type CapabilityMatchingConfig struct {
	// WeightByRelevance controls whether to weight matches by relevance.
	// +kubebuilder:default=true
	// +optional
	WeightByRelevance *bool `json:"weightByRelevance,omitempty"`

	// RequireExactMatch controls whether to require exact capability matches.
	// +kubebuilder:default=false
	// +optional
	RequireExactMatch *bool `json:"requireExactMatch,omitempty"`

	// SimilarityThreshold defines the minimum similarity threshold for matches.
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.7
	// +optional
	SimilarityThreshold *float64 `json:"similarityThreshold,omitempty"`
}

// ConsensusConfig defines consensus-based tool selection.
type ConsensusConfig struct {
	// MinServers defines the minimum number of servers for consensus.
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:default=3
	// +optional
	MinServers *int32 `json:"minServers,omitempty"`

	// Timeout defines the consensus timeout.
	// +kubebuilder:default="30s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// AgreementThreshold defines the required agreement percentage.
	// +kubebuilder:validation:Minimum=0.5
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.67
	// +optional
	AgreementThreshold *float64 `json:"agreementThreshold,omitempty"`
}

// WorkflowEngine defines supported workflow engines.
type WorkflowEngine string

const (
	WorkflowEngineSimple   WorkflowEngine = "Simple"
	WorkflowEngineTemporal WorkflowEngine = "Temporal"
	WorkflowEngineArgo     WorkflowEngine = "Argo"
)

// DecisionStrategy defines decision making strategies.
type DecisionStrategy string

const (
	DecisionStrategyGreedyBest     DecisionStrategy = "GreedyBest"
	DecisionStrategyExploreExploit DecisionStrategy = "ExploreExploit"
	DecisionStrategyContextAware   DecisionStrategy = "ContextAware"
)

// GatewayNetworkConfig defines service exposure, transport protocols, and ingress settings.
type GatewayNetworkConfig struct {
	// Service defines the service configuration for the gateway.
	// +optional
	Service *GatewayServiceConfig `json:"service,omitempty"`

	// Transport defines the transport protocol configuration.
	// +optional
	Transport *GatewayTransportConfig `json:"transport,omitempty"`

	// Ingress defines ingress configuration for external access.
	// +optional
	Ingress *GatewayIngressConfig `json:"ingress,omitempty"`

	// LoadBalancer defines load balancer configuration.
	// +optional
	LoadBalancer *GatewayLoadBalancerConfig `json:"loadBalancer,omitempty"`
}

// GatewayServiceConfig defines service configuration.
type GatewayServiceConfig struct {
	// Type defines the service type.
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Ports define the service ports.
	// +optional
	// +listType=atomic
	Ports []GatewayServicePort `json:"ports,omitempty"`

	// Annotations define service annotations.
	// +optional
	// +mapType=atomic
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayServicePort defines a service port.
type GatewayServicePort struct {
	// Name is the port name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Port is the service port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	Port int32 `json:"port"`

	// TargetPort is the target port on pods.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`

	// Protocol is the port protocol.
	// +kubebuilder:validation:Enum=TCP;UDP
	// +kubebuilder:default=TCP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// GatewayTransportConfig defines transport protocol configuration.
type GatewayTransportConfig struct {
	// HTTP defines HTTP transport configuration.
	// +optional
	HTTP *HTTPTransportConfig `json:"http,omitempty"`

	// GRPC defines gRPC transport configuration.
	// +optional
	GRPC *GRPCTransportConfig `json:"grpc,omitempty"`

	// WebSocket defines WebSocket transport configuration.
	// +optional
	WebSocket *WebSocketTransportConfig `json:"webSocket,omitempty"`
}

// HTTPTransportConfig defines HTTP transport configuration.
type HTTPTransportConfig struct {
	// Port defines the HTTP port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	Port *int32 `json:"port,omitempty"`

	// ReadTimeout defines the HTTP read timeout.
	// +kubebuilder:default="30s"
	// +optional
	ReadTimeout *metav1.Duration `json:"readTimeout,omitempty"`

	// WriteTimeout defines the HTTP write timeout.
	// +kubebuilder:default="30s"
	// +optional
	WriteTimeout *metav1.Duration `json:"writeTimeout,omitempty"`

	// MaxHeaderSize defines the maximum header size.
	// +optional
	MaxHeaderSize *resource.Quantity `json:"maxHeaderSize,omitempty"`
}

// GRPCTransportConfig defines gRPC transport configuration.
type GRPCTransportConfig struct {
	// Port defines the gRPC port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9090
	// +optional
	Port *int32 `json:"port,omitempty"`

	// MaxMessageSize defines the maximum message size.
	// +optional
	MaxMessageSize *resource.Quantity `json:"maxMessageSize,omitempty"`

	// ConnectionTimeout defines the connection timeout.
	// +kubebuilder:default="10s"
	// +optional
	ConnectionTimeout *metav1.Duration `json:"connectionTimeout,omitempty"`
}

// WebSocketTransportConfig defines WebSocket transport configuration.
type WebSocketTransportConfig struct {
	// Port defines the WebSocket port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8081
	// +optional
	Port *int32 `json:"port,omitempty"`

	// ReadBufferSize defines the read buffer size.
	// +optional
	ReadBufferSize *resource.Quantity `json:"readBufferSize,omitempty"`

	// WriteBufferSize defines the write buffer size.
	// +optional
	WriteBufferSize *resource.Quantity `json:"writeBufferSize,omitempty"`

	// PingInterval defines the ping interval for keep-alive.
	// +kubebuilder:default="30s"
	// +optional
	PingInterval *metav1.Duration `json:"pingInterval,omitempty"`
}

// GatewayIngressConfig defines ingress configuration.
type GatewayIngressConfig struct {
	// Enabled controls whether ingress is created.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ClassName defines the ingress class name.
	// +optional
	ClassName string `json:"className,omitempty"`

	// Hosts define the ingress hosts.
	// +optional
	// +listType=atomic
	Hosts []GatewayIngressHost `json:"hosts,omitempty"`

	// TLS defines TLS configuration for ingress.
	// +optional
	// +listType=atomic
	TLS []GatewayIngressTLS `json:"tls,omitempty"`

	// Annotations define ingress annotations.
	// +optional
	// +mapType=atomic
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayIngressHost defines an ingress host.
type GatewayIngressHost struct {
	// Host is the host name.
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Paths define the host paths.
	// +optional
	// +listType=atomic
	Paths []GatewayIngressPath `json:"paths,omitempty"`
}

// GatewayIngressPath defines an ingress path.
type GatewayIngressPath struct {
	// Path is the URL path.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// PathType defines the path type.
	// +kubebuilder:validation:Enum=Exact;Prefix;ImplementationSpecific
	// +kubebuilder:default=Prefix
	// +optional
	PathType string `json:"pathType,omitempty"`

	// ServiceName is the backend service name.
	// +kubebuilder:validation:Required
	ServiceName string `json:"serviceName"`

	// ServicePort is the backend service port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	ServicePort int32 `json:"servicePort"`
}

// GatewayIngressTLS defines ingress TLS configuration.
type GatewayIngressTLS struct {
	// Hosts define the TLS hosts.
	// +optional
	// +listType=set
	Hosts []string `json:"hosts,omitempty"`

	// SecretName references the TLS secret.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// GatewayLoadBalancerConfig defines load balancer configuration.
type GatewayLoadBalancerConfig struct {
	// Type defines the load balancer type.
	// +kubebuilder:validation:Enum=Internal;External
	// +kubebuilder:default=External
	// +optional
	Type LoadBalancerType `json:"type,omitempty"`

	// SourceRanges define allowed source IP ranges.
	// +optional
	// +listType=set
	SourceRanges []string `json:"sourceRanges,omitempty"`

	// Annotations define load balancer annotations.
	// +optional
	// +mapType=atomic
	Annotations map[string]string `json:"annotations,omitempty"`
}

// LoadBalancerType defines load balancer types.
type LoadBalancerType string

const (
	LoadBalancerTypeInternal LoadBalancerType = "Internal"
	LoadBalancerTypeExternal LoadBalancerType = "External"
)

// CircuitBreakerConfig defines circuit breaker configuration.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures to open the circuit.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes to close the circuit.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`

	// OpenStateTimeout is the time to wait before transitioning to half-open.
	// +kubebuilder:default="30s"
	// +optional
	OpenStateTimeout *metav1.Duration `json:"openStateTimeout,omitempty"`

	// MaxRequestsHalfOpen is the maximum requests allowed in half-open state.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxRequestsHalfOpen *int32 `json:"maxRequestsHalfOpen,omitempty"`

	// RequestTimeout defines the timeout for requests in various states.
	// +kubebuilder:default="30s"
	// +optional
	RequestTimeout *metav1.Duration `json:"requestTimeout,omitempty"`

	// ErrorRateThreshold defines the error rate percentage to open the circuit.
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=100.0
	// +kubebuilder:default=50.0
	// +optional
	ErrorRateThreshold *float64 `json:"errorRateThreshold,omitempty"`

	// MinRequestsThreshold is the minimum requests before error rate is calculated.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=20
	// +optional
	MinRequestsThreshold *int32 `json:"minRequestsThreshold,omitempty"`
}

// RateLimitConfig defines rate limiting policies.
type RateLimitConfig struct {
	// Global rate limit applied to all requests.
	// +optional
	Global *RateLimitPolicy `json:"global,omitempty"`

	// PerUser defines rate limits per authenticated user.
	// +optional
	PerUser *RateLimitPolicy `json:"perUser,omitempty"`

	// PerIP defines rate limits per client IP address.
	// +optional
	PerIP *RateLimitPolicy `json:"perIP,omitempty"`

	// PerServer defines rate limits per upstream server.
	// +optional
	PerServer *RateLimitPolicy `json:"perServer,omitempty"`
}

// RateLimitPolicy defines a rate limiting policy.
type RateLimitPolicy struct {
	// RequestsPerSecond is the number of requests allowed per second.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerSecond *int32 `json:"requestsPerSecond,omitempty"`

	// RequestsPerMinute is the number of requests allowed per minute.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerMinute *int32 `json:"requestsPerMinute,omitempty"`

	// RequestsPerHour is the number of requests allowed per hour.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerHour *int32 `json:"requestsPerHour,omitempty"`

	// Burst is the burst capacity for rate limiting.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Burst *int32 `json:"burst,omitempty"`
}

// ObservabilityConfig defines monitoring, metrics, and tracing configuration.
type ObservabilityConfig struct {
	// Metrics defines metrics collection and export configuration.
	// +optional
	Metrics *MetricsConfig `json:"metrics,omitempty"`

	// Tracing defines distributed tracing configuration.
	// +optional
	Tracing *TracingConfig `json:"tracing,omitempty"`

	// Logging defines structured logging configuration.
	// +optional
	Logging *LoggingConfig `json:"logging,omitempty"`

	// Health defines health check endpoint configuration.
	// +optional
	Health *HealthEndpointConfig `json:"health,omitempty"`
}

// MetricsConfig defines metrics collection and export configuration.
type MetricsConfig struct {
	// Enabled controls whether metrics collection is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port defines the metrics endpoint port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9090
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Path defines the metrics endpoint path.
	// +kubebuilder:default="/metrics"
	// +optional
	Path string `json:"path,omitempty"`

	// Format defines the metrics format.
	// +kubebuilder:validation:Enum=Prometheus;OpenMetrics
	// +kubebuilder:default=Prometheus
	// +optional
	Format MetricsFormat `json:"format,omitempty"`

	// CustomMetrics define additional custom metrics to collect.
	// +optional
	// +listType=atomic
	CustomMetrics []CustomMetric `json:"customMetrics,omitempty"`
}

// TracingConfig defines distributed tracing configuration.
type TracingConfig struct {
	// Enabled controls whether distributed tracing is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Provider defines the tracing backend provider.
	// +kubebuilder:validation:Enum=Jaeger;Zipkin;OpenTelemetry;DataDog
	// +kubebuilder:default=OpenTelemetry
	// +optional
	Provider TracingProvider `json:"provider,omitempty"`

	// Endpoint defines the tracing collector endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// SamplingRate defines the sampling rate for traces (0.0-1.0).
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.1
	// +optional
	SamplingRate *float64 `json:"samplingRate,omitempty"`

	// Headers define additional headers to send with traces.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`
}

// LoggingConfig defines structured logging configuration.
type LoggingConfig struct {
	// Level defines the logging level.
	// +kubebuilder:validation:Enum=Debug;Info;Warn;Error
	// +kubebuilder:default=Info
	// +optional
	Level LogLevel `json:"level,omitempty"`

	// Format defines the log format.
	// +kubebuilder:validation:Enum=JSON;Text
	// +kubebuilder:default=JSON
	// +optional
	Format LogFormat `json:"format,omitempty"`

	// Output defines where logs are sent.
	// +kubebuilder:validation:Enum=Stdout;File;Syslog
	// +kubebuilder:default=Stdout
	// +optional
	Output LogOutput `json:"output,omitempty"`

	// File defines file-based logging configuration.
	// +optional
	File *LogFileConfig `json:"file,omitempty"`
}

// HealthEndpointConfig defines health check endpoint configuration.
type HealthEndpointConfig struct {
	// Enabled controls whether health endpoints are exposed.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port defines the health endpoint port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	Port *int32 `json:"port,omitempty"`

	// LivenessPath defines the liveness probe path.
	// +kubebuilder:default="/healthz"
	// +optional
	LivenessPath string `json:"livenessPath,omitempty"`

	// ReadinessPath defines the readiness probe path.
	// +kubebuilder:default="/readyz"
	// +optional
	ReadinessPath string `json:"readinessPath,omitempty"`
}

// MetricsFormat defines supported metrics formats.
type MetricsFormat string

const (
	MetricsFormatPrometheus  MetricsFormat = "Prometheus"
	MetricsFormatOpenMetrics MetricsFormat = "OpenMetrics"
)

// TracingProvider defines supported tracing providers.
type TracingProvider string

const (
	TracingProviderJaeger        TracingProvider = "Jaeger"
	TracingProviderZipkin        TracingProvider = "Zipkin"
	TracingProviderOpenTelemetry TracingProvider = "OpenTelemetry"
	TracingProviderDataDog       TracingProvider = "DataDog"
)

// LogLevel defines logging levels.
type LogLevel string

const (
	LogLevelDebug LogLevel = "Debug"
	LogLevelInfo  LogLevel = "Info"
	LogLevelWarn  LogLevel = "Warn"
	LogLevelError LogLevel = "Error"
)

// LogFormat defines log formats.
type LogFormat string

const (
	LogFormatJSON LogFormat = "JSON"
	LogFormatText LogFormat = "Text"
)

// LogOutput defines log output destinations.
type LogOutput string

const (
	LogOutputStdout LogOutput = "Stdout"
	LogOutputFile   LogOutput = "File"
	LogOutputSyslog LogOutput = "Syslog"
)

// CustomMetric defines a custom metric to collect.
type CustomMetric struct {
	// Name is the metric name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type defines the metric type.
	// +kubebuilder:validation:Enum=Counter;Gauge;Histogram;Summary
	// +kubebuilder:validation:Required
	Type MetricType `json:"type"`

	// Help provides a description of the metric.
	// +optional
	Help string `json:"help,omitempty"`

	// Labels define metric labels.
	// +optional
	// +listType=set
	Labels []string `json:"labels,omitempty"`
}

// MetricType defines metric types.
type MetricType string

const (
	MetricTypeCounter   MetricType = "Counter"
	MetricTypeGauge     MetricType = "Gauge"
	MetricTypeHistogram MetricType = "Histogram"
	MetricTypeSummary   MetricType = "Summary"
)

// LogFileConfig defines file-based logging configuration.
type LogFileConfig struct {
	// Path defines the log file path.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// MaxSize defines the maximum log file size before rotation.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MaxFiles defines the maximum number of log files to keep.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	MaxFiles *int32 `json:"maxFiles,omitempty"`

	// Compress controls whether rotated logs are compressed.
	// +kubebuilder:default=true
	// +optional
	Compress *bool `json:"compress,omitempty"`
}

// MCPGatewayServerStatus defines the status of a connected MCP server.
type MCPGatewayServerStatus struct {
	// Name is the name of the MCP server.
	Name string `json:"name"`

	// Namespace is the namespace of the MCP server.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// State is the current connection state.
	// +kubebuilder:validation:Enum=Connected;Disconnected;Connecting;Unhealthy;CircuitOpen
	State string `json:"state"`

	// LastConnected is the timestamp of the last successful connection.
	// +optional
	LastConnected *metav1.Time `json:"lastConnected,omitempty"`

	// LastError contains the last error encountered.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// RequestCount is the total number of requests sent to this server.
	// +optional
	RequestCount int64 `json:"requestCount,omitempty"`

	// ErrorCount is the total number of errors from this server.
	// +optional
	ErrorCount int64 `json:"errorCount,omitempty"`

	// AverageResponseTime is the average response time in milliseconds.
	// +optional
	AverageResponseTime int32 `json:"averageResponseTime,omitempty"`

	// CircuitBreakerState is the current circuit breaker state.
	// +kubebuilder:validation:Enum=Closed;Open;HalfOpen
	// +optional
	CircuitBreakerState string `json:"circuitBreakerState,omitempty"`

	// Weight is the current effective weight for load balancing.
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Tags are the current tags associated with this server.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`

	// Capabilities are the server's current capabilities.
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`
}

// MCPGatewayStatus defines the observed state of MCPGateway.
type MCPGatewayStatus struct {
	// Conditions represent the latest available observations of the MCPGateway's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase is the current phase of the MCPGateway lifecycle.
	// +optional
	Phase MCPGatewayPhase `json:"phase,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available replicas.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ActiveSessions is the current number of active sessions.
	// +optional
	ActiveSessions int32 `json:"activeSessions,omitempty"`

	// TotalRequests is the total number of requests processed.
	// +optional
	TotalRequests int64 `json:"totalRequests,omitempty"`

	// TotalErrors is the total number of errors encountered.
	// +optional
	TotalErrors int64 `json:"totalErrors,omitempty"`

	// AverageLatency is the average request latency in milliseconds.
	// +optional
	AverageLatency int32 `json:"averageLatency,omitempty"`

	// ConnectedServers is the list of currently connected servers.
	// +optional
	// +listType=map
	// +listMapKey=name
	ConnectedServers []MCPGatewayServerStatus `json:"connectedServers,omitempty"`

	// ServerStatusSummary provides an aggregated summary of server statuses.
	// +optional
	ServerStatusSummary *ServerStatusSummary `json:"serverStatusSummary,omitempty"`

	// UnhealthyServers lists servers that are currently unhealthy.
	// +optional
	// +listType=map
	// +listMapKey=name
	UnhealthyServers []MCPGatewayServerStatus `json:"unhealthyServers,omitempty"`

	// ServiceURL is the URL where the gateway service can be accessed.
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// LastUpdateTime is the last time the status was updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Metrics provides real-time performance metrics.
	// +optional
	Metrics *GatewayMetrics `json:"metrics,omitempty"`

	// ConfigStatus provides configuration validation status.
	// +optional
	ConfigStatus *ConfigValidationStatus `json:"configStatus,omitempty"`

	// ResourceUsage provides current resource utilization information.
	// +optional
	ResourceUsage *ResourceUsageStatus `json:"resourceUsage,omitempty"`

	// FederationPeers provides status of federated peer gateways.
	// +optional
	// +listType=map
	// +listMapKey=name
	FederationPeers []FederationPeerStatus `json:"federationPeers,omitempty"`

	// MemoryBuckets provides status of memory buckets.
	// +optional
	// +listType=map
	// +listMapKey=name
	MemoryBuckets []MemoryBucketStatus `json:"memoryBuckets,omitempty"`

	// ToolRegistry provides status of the tool registry.
	// +optional
	ToolRegistry *ToolRegistryStatus `json:"toolRegistry,omitempty"`

	// ResourceRegistry provides status of the resource registry.
	// +optional
	ResourceRegistry *ResourceRegistryStatus `json:"resourceRegistry,omitempty"`

	// PromptRegistry provides status of the prompt registry.
	// +optional
	PromptRegistry *PromptRegistryStatus `json:"promptRegistry,omitempty"`

	// WorkflowStatus provides status of workflow orchestration.
	// +optional
	WorkflowStatus *WorkflowStatus `json:"workflowStatus,omitempty"`
}

// FederationPeerStatus provides status of a federated peer gateway.
type FederationPeerStatus struct {
	// Name is the peer identifier.
	Name string `json:"name"`

	// URL is the peer gateway URL.
	URL string `json:"url"`

	// State is the current connection state.
	// +kubebuilder:validation:Enum=Connected;Disconnected;Connecting;Unhealthy
	State string `json:"state"`

	// LastSynced is the timestamp of the last successful sync.
	// +optional
	LastSynced *metav1.Time `json:"lastSynced,omitempty"`

	// LastError contains the last error encountered.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// AvailableTools is the number of tools available through this peer.
	// +optional
	AvailableTools int32 `json:"availableTools,omitempty"`

	// AvailableResources is the number of resources available through this peer.
	// +optional
	AvailableResources int32 `json:"availableResources,omitempty"`

	// AvailablePrompts is the number of prompts available through this peer.
	// +optional
	AvailablePrompts int32 `json:"availablePrompts,omitempty"`
}

// MemoryBucketStatus provides status of a memory bucket.
type MemoryBucketStatus struct {
	// Name is the bucket identifier.
	Name string `json:"name"`

	// EntryCount is the current number of entries in the bucket.
	// +optional
	EntryCount int32 `json:"entryCount,omitempty"`

	// SizeBytes is the current size of the bucket in bytes.
	// +optional
	SizeBytes int64 `json:"sizeBytes,omitempty"`

	// LastAccessed is the timestamp of the last access.
	// +optional
	LastAccessed *metav1.Time `json:"lastAccessed,omitempty"`
}

// ToolRegistryStatus provides status of the tool registry.
type ToolRegistryStatus struct {
	// TotalTools is the total number of registered tools.
	// +optional
	TotalTools int32 `json:"totalTools,omitempty"`

	// MCPTools is the number of native MCP tools.
	// +optional
	MCPTools int32 `json:"mcpTools,omitempty"`

	// RestTools is the number of virtualized REST API tools.
	// +optional
	RestTools int32 `json:"restTools,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// ResourceRegistryStatus provides status of the resource registry.
type ResourceRegistryStatus struct {
	// TotalResources is the total number of registered resources.
	// +optional
	TotalResources int32 `json:"totalResources,omitempty"`

	// CacheHitRate is the resource cache hit rate as a percentage.
	// +optional
	CacheHitRate float64 `json:"cacheHitRate,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// PromptRegistryStatus provides status of the prompt registry.
type PromptRegistryStatus struct {
	// TotalPrompts is the total number of registered prompts.
	// +optional
	TotalPrompts int32 `json:"totalPrompts,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// WorkflowStatus provides status of workflow orchestration.
type WorkflowStatus struct {
	// ActiveWorkflows is the number of currently active workflows.
	// +optional
	ActiveWorkflows int32 `json:"activeWorkflows,omitempty"`

	// CompletedWorkflows is the total number of completed workflows.
	// +optional
	CompletedWorkflows int64 `json:"completedWorkflows,omitempty"`

	// FailedWorkflows is the total number of failed workflows.
	// +optional
	FailedWorkflows int64 `json:"failedWorkflows,omitempty"`

	// AverageExecutionTime is the average workflow execution time in seconds.
	// +optional
	AverageExecutionTime float64 `json:"averageExecutionTime,omitempty"`
}

// ServerStatusSummary provides an aggregated summary of server statuses.
type ServerStatusSummary struct {
	// Total number of configured servers.
	Total int32 `json:"total"`

	// Number of servers that are connected and healthy.
	Connected int32 `json:"connected"`

	// Number of servers that are disconnected.
	Disconnected int32 `json:"disconnected"`

	// Number of servers that are unhealthy.
	Unhealthy int32 `json:"unhealthy"`

	// Number of servers with open circuit breakers.
	CircuitOpen int32 `json:"circuitOpen"`
}

// GatewayMetrics provides real-time performance metrics for the gateway.
type GatewayMetrics struct {
	// RequestsPerSecond is the current requests per second rate.
	// +optional
	RequestsPerSecond float64 `json:"requestsPerSecond,omitempty"`

	// P50Latency is the 50th percentile latency in milliseconds.
	// +optional
	P50Latency int32 `json:"p50Latency,omitempty"`

	// P90Latency is the 99th percentile latency in milliseconds.
	// +optional
	P90Latency int32 `json:"p90Latency,omitempty"`

	// ErrorRate is the current error rate as a percentage.
	// +optional
	ErrorRate float64 `json:"errorRate,omitempty"`

	// ActiveConnections is the number of currently active connections.
	// +optional
	ActiveConnections int32 `json:"activeConnections,omitempty"`

	// CacheHitRate is the cache hit rate as a percentage.
	// +optional
	CacheHitRate float64 `json:"cacheHitRate,omitempty"`

	// ToolInvocations tracks tool invocation statistics.
	// +optional
	// +mapType=atomic
	ToolInvocations map[string]int64 `json:"toolInvocations,omitempty"`
}

// ConfigValidationStatus provides configuration validation status.
type ConfigValidationStatus struct {
	// Valid indicates whether the current configuration is valid.
	Valid bool `json:"valid"`

	// ValidationErrors contains any configuration validation errors.
	// +optional
	// +listType=atomic
	ValidationErrors []ConfigValidationError `json:"validationErrors,omitempty"`

	// Warnings contains non-blocking configuration warnings.
	// +optional
	// +listType=atomic
	Warnings []string `json:"warnings,omitempty"`

	// LastValidated is when the configuration was last validated.
	// +optional
	LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}

// ConfigValidationError represents a configuration validation error.
type ConfigValidationError struct {
	// Field is the configuration field that failed validation.
	Field string `json:"field"`

	// Message describes the validation error.
	Message string `json:"message"`

	// Severity indicates the error severity.
	// +kubebuilder:validation:Enum=Error;Warning
	Severity ValidationSeverity `json:"severity"`
}

// ValidationSeverity defines validation error severities.
type ValidationSeverity string

const (
	ValidationSeverityError   ValidationSeverity = "Error"
	ValidationSeverityWarning ValidationSeverity = "Warning"
)

// ResourceUsageStatus provides current resource utilization information.
type ResourceUsageStatus struct {
	// CPU usage as a percentage of allocated resources.
	// +optional
	CPUUsage float64 `json:"cpuUsage,omitempty"`

	// Memory usage as a percentage of allocated resources.
	// +optional
	MemoryUsage float64 `json:"memoryUsage,omitempty"`

	// Storage usage for context and cache storage.
	// +optional
	StorageUsage *StorageUsageInfo `json:"storageUsage,omitempty"`

	// Network usage statistics.
	// +optional
	NetworkUsage *NetworkUsageInfo `json:"networkUsage,omitempty"`
}

// StorageUsageInfo provides storage usage information.
type StorageUsageInfo struct {
	// ContextStorage usage information.
	// +optional
	ContextStorage *StorageMetrics `json:"contextStorage,omitempty"`

	// CacheStorage usage information.
	// +optional
	CacheStorage *StorageMetrics `json:"cacheStorage,omitempty"`
}

// StorageMetrics provides storage metrics.
type StorageMetrics struct {
	// Used storage in bytes.
	Used resource.Quantity `json:"used"`

	// Available storage in bytes.
	Available resource.Quantity `json:"available"`

	// Usage percentage.
	UsagePercent float64 `json:"usagePercent"`
}

// NetworkUsageInfo provides network usage information.
type NetworkUsageInfo struct {
	// BytesIn is the total bytes received.
	BytesIn int64 `json:"bytesIn"`

	// BytesOut is the total bytes sent.
	BytesOut int64 `json:"bytesOut"`

	// ConnectionsPerSecond is the rate of new connections.
	ConnectionsPerSecond float64 `json:"connectionsPerSecond"`
}

// MCPGatewayPhase is the phase of the MCPGateway.
// +kubebuilder:validation:Enum=Pending;Starting;Running;Updating;Failed;Terminating
type MCPGatewayPhase string

const (
	// MCPGatewayPhasePending means the MCPGateway is being created.
	MCPGatewayPhasePending MCPGatewayPhase = "Pending"

	// MCPGatewayPhaseStarting means the MCPGateway is starting up.
	MCPGatewayPhaseStarting MCPGatewayPhase = "Starting"

	// MCPGatewayPhaseRunning means the MCPGateway is running and ready.
	MCPGatewayPhaseRunning MCPGatewayPhase = "Running"

	// MCPGatewayPhaseUpdating means the MCPGateway is being updated.
	MCPGatewayPhaseUpdating MCPGatewayPhase = "Updating"

	// MCPGatewayPhaseFailed means the MCPGateway failed to start or run.
	MCPGatewayPhaseFailed MCPGatewayPhase = "Failed"

	// MCPGatewayPhaseTerminating means the MCPGateway is being deleted.
	MCPGatewayPhaseTerminating MCPGatewayPhase = "Terminating"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.readyReplicas/.status.replicas"
// +kubebuilder:printcolumn:name="Servers",type="string",JSONPath=".status.serverStatusSummary.connected/.status.serverStatusSummary.total"
// +kubebuilder:printcolumn:name="Tools",type="integer",JSONPath=".status.toolRegistry.totalTools"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MCPGateway is the Schema for the mcpgateways API.
// MCPGateway provides AI-aware routing, context management, and federation
// capabilities for Model Context Protocol (MCP) servers and tools.
type MCPGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPGatewaySpec   `json:"spec,omitempty"`
	Status MCPGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPGatewayList contains a list of MCPGateway.
type MCPGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPGateway `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.readyReplicas/.status.replicas"
// +kubebuilder:printcolumn:name="Servers",type="string",JSONPath=".status.serverStatusSummary.connected/.status.serverStatusSummary.total"
// +kubebuilder:printcolumn:name="Tools",type="integer",JSONPath=".status.toolRegistry.totalTools"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterMCPGateway is the cluster-scoped Schema for the mcpgateways API.
// ClusterMCPGateway provides AI-aware routing, context management, and federation
// capabilities for Model Context Protocol (MCP) servers across the entire cluster.
type ClusterMCPGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPGatewaySpec   `json:"spec,omitempty"`
	Status MCPGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterMCPGatewayList contains a list of ClusterMCPGateway.
type ClusterMCPGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterMCPGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPGateway{}, &MCPGatewayList{})
	SchemeBuilder.Register(&ClusterMCPGateway{}, &ClusterMCPGatewayList{})
}
