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

	// Transport defines the supported transport protocols for MCP communication.
	// +optional
	Transport *MCPTransportType `json:"transport,omitempty"`

	// Policy defines unified security, authentication, authorization, and traffic policies.
	// +optional
	Policy *MCPGatewayPolicyConfig `json:"policy,omitempty"`

	// Observability defines monitoring, metrics, and tracing configuration.
	// +optional
	Observability *MCPGatewayObservabilityConfig `json:"observability,omitempty"`

	// Network defines service exposure and ingress settings.
	// +optional
	Network *MCPGatewayNetworkConfig `json:"network,omitempty"`

	// ProtocolVersion defines MCP protocol version constraints and negotiation settings.
	// +optional
	ProtocolVersion *MCPProtocolVersionConfig `json:"protocolVersion,omitempty"`
}

// MCPServerDiscoveryConfig defines how the gateway discovers and connects to MCP servers.
type MCPServerDiscoveryConfig struct {
	// Static provides a fixed list of MCP server references.
	// +optional
	// +listType=atomic
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

// GatewayNetworkConfig defines service exposure, transport protocols, and ingress settings.
type MCPGatewayNetworkConfig struct {
	// Service defines the service configuration for the gateway.
	// +optional
	Service *GatewayServiceConfig `json:"service,omitempty"`

	// Transport defines the transport protocol configuration.
	// +optional
	Transport *GatewayTransportConfig `json:"transport,omitempty"`

	// Ingress defines ingress configuration for external access.
	// +optional
	Ingress *GatewayIngressConfig `json:"ingress,omitempty"`
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

// MCPProtocolVersionConfig defines MCP protocol version constraints and negotiation settings.
type MCPProtocolVersionConfig struct {
	// Supported defines the list of supported MCP protocol versions.
	// If empty, the gateway will support all known versions.
	// +optional
	// +listType=set
	Supported []string `json:"supported,omitempty"`

	// MinVersion defines the minimum acceptable MCP protocol version.
	// +kubebuilder:default="2025-06-18"
	// +optional
	MinVersion string `json:"minVersion,omitempty"`

	// MaxVersion defines the maximum acceptable MCP protocol version.
	// +optional
	MaxVersion string `json:"maxVersion,omitempty"`

	// PreferredVersion defines the preferred protocol version for new connections.
	// +kubebuilder:default="2025-06-18"
	// +optional
	PreferredVersion string `json:"preferredVersion,omitempty"`

	// AllowVersionNegotiation controls whether version negotiation is allowed.
	// +kubebuilder:default=true
	// +optional
	AllowVersionNegotiation *bool `json:"allowVersionNegotiation,omitempty"`

	// StrictVersioning controls whether to reject connections with unsupported versions.
	// +kubebuilder:default=false
	// +optional
	StrictVersioning *bool `json:"strictVersioning,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MCPGateway{}, &MCPGatewayList{})
	SchemeBuilder.Register(&ClusterMCPGateway{}, &ClusterMCPGatewayList{})
}
