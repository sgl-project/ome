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

	// Federation defines automatic peer discovery and coordination for multiple MCP Gateway replicas.
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
