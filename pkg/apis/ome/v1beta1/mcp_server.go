package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPTransportType defines the transport method for MCP communication
// +kubebuilder:validation:Enum=stdio;streamable-http;sse
type MCPTransportType string

const (
	// MCPTransportStdio uses standard input/output for communication
	MCPTransportStdio MCPTransportType = "stdio"

	// MCPTransportStreamableHTTP uses HTTP with streaming support
	MCPTransportStreamableHTTP MCPTransportType = "streamable-http"

	// MCPTransportSSE uses Server-Sent Events for communication
	MCPTransportSSE MCPTransportType = "sse"
)

// MCPServerType defines the type of MCP server deployment
// +kubebuilder:validation:Enum=Hosted;Remote
type MCPServerType string

const (
	// MCPServerTypeHosted means the server is hosted within the Kubernetes cluster
	MCPServerTypeHosted MCPServerType = "Hosted"

	// MCPServerTypeRemote means the server is hosted externally and accessed via URL
	MCPServerTypeRemote MCPServerType = "Remote"
)

// MCPProtocolSpec defines the MCP protocol specification
type MCPProtocolSpec struct {
	// Name of the protocol (always "JSON-RPC" for MCP)
	// +kubebuilder:validation:Enum=JSON-RPC
	// +kubebuilder:default="JSON-RPC"
	// +optional
	Name string `json:"name,omitempty"`

	// Version of the JSON-RPC protocol (always "2.0" for MCP)
	// +kubebuilder:validation:Enum="2.0"
	// +kubebuilder:default="2.0"
	// +optional
	Version string `json:"version,omitempty"`
}

// MCPCapabilities defines the capabilities supported by the MCP server
type MCPCapabilities struct {
	// Tools indicates whether the server supports MCP tools
	// +kubebuilder:default=true
	// +optional
	Tools *bool `json:"tools,omitempty"`

	// Resources indicates whether the server supports MCP resources
	// +kubebuilder:default=false
	// +optional
	Resources *bool `json:"resources,omitempty"`

	// Prompts indicates whether the server supports MCP prompts
	// +kubebuilder:default=false
	// +optional
	Prompts *bool `json:"prompts,omitempty"`
}

// MCPServerSpec defines the desired state of MCPServer
// +kubebuilder:validation:XValidation:rule="self.type == 'Hosted' ? has(self.image) && self.image != ” : true",message="image is required for Hosted MCP servers"
// +kubebuilder:validation:XValidation:rule="self.type == 'Remote' ? has(self.url) && self.url != ” : true",message="url is required for Remote MCP servers"
type MCPServerSpec struct {
	// Type specifies whether this is a hosted or remote MCP server
	// Hosted servers run as containers in the cluster, Remote servers are accessed via URL
	// +kubebuilder:validation:Enum=Hosted;Remote
	// +kubebuilder:default=Hosted
	// +optional
	Type MCPServerType `json:"type,omitempty"`

	// URL is the external URL for remote MCP servers
	// Required for Remote servers, ignored for Hosted servers
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	URL string `json:"url,omitempty"`

	// Transport specifies the transport method for MCP communication
	// +kubebuilder:default=stdio
	// +optional
	Transport MCPTransportType `json:"transport,omitempty"`

	// Protocol defines the MCP protocol specification
	// +optional
	Protocol *MCPProtocolSpec `json:"protocol,omitempty"`

	// Capabilities defines the MCP capabilities supported by this server
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`

	// Version is the version of the MCP server software
	// +optional
	Version string `json:"version,omitempty"`

	// Replicas is the number of desired replicas for hosted MCP servers
	// Only applicable for Hosted servers with HTTP transport
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// TargetPort is the port that MCP server listens to
	// If not specified, defaults to the same value as Port (8080)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	TargetPort int32 `json:"targetPort,omitempty"`

	// PermissionProfile defines the permission profile to use
	// +optional
	PermissionProfile *PermissionProfileRef `json:"permissionProfile,omitempty"`

	// PodTemplateSpec defines the pod template to use for the MCP server
	// +optional
	PodTemplateSpec *corev1.PodTemplateSpec `json:"podTemplateSpec,omitempty"`

	// OIDCConfig defines OIDC authentication configuration for the MCP server
	// +optional
	OIDCConfig *OIDCConfigRef `json:"oidcConfig,omitempty"`

	// AuthzConfig defines authorization policy configuration for the MCP server
	// +optional
	AuthzConfig *AuthzConfigRef `json:"authzConfig,omitempty"`

	// ToolsFilter is the filter on tools applied to the MCP server
	// +optional
	// +listType=set
	ToolsFilter []string `json:"toolsFilter,omitempty"`
}

// Permission profile types
const (
	// PermissionProfileTypeBuiltin is the type for built-in permission profiles
	PermissionProfileTypeBuiltin = "builtin"

	// PermissionProfileTypeConfigMap is the type for permission profiles stored in ConfigMaps
	PermissionProfileTypeConfigMap = "configMap"

	// PermissionProfileTypeInline is the type for inline permission profiles
	PermissionProfileTypeInline = "inline"
)

// OIDC configuration types
const (
	// OIDCConfigTypeKubernetes is the type for Kubernetes service account token validation
	OIDCConfigTypeKubernetes = "kubernetes"

	// OIDCConfigTypeConfigMap is the type for OIDC configuration stored in ConfigMaps
	OIDCConfigTypeConfigMap = "configMap"

	// OIDCConfigTypeInline is the type for inline OIDC configuration
	OIDCConfigTypeInline = "inline"
)

// Authorization configuration types
const (
	// AuthzConfigTypeConfigMap is the type for authorization configuration stored in ConfigMaps
	AuthzConfigTypeConfigMap = "configMap"

	// AuthzConfigTypeInline is the type for inline authorization configuration
	AuthzConfigTypeInline = "inline"
)

// PermissionProfileRef defines a reference to a permission profile
// +kubebuilder:validation:XValidation:rule="self.type == 'builtin' ? has(self.name) && self.name != ” : true",message="name is required for builtin permission profiles"
// +kubebuilder:validation:XValidation:rule="self.type == 'configMap' ? has(self.configMap) : true",message="configMap is required for configMap permission profiles"
// +kubebuilder:validation:XValidation:rule="self.type == 'inline' ? has(self.inline) : true",message="inline is required for inline permission profiles"
type PermissionProfileRef struct {
	// Type is the type of permission profile reference
	// +kubebuilder:validation:Enum=builtin;configMap;inline
	// +kubebuilder:default=builtin
	Type string `json:"type"`

	// Name is the name of the built-in permission profile
	// If Type is "builtin", Name must be one of: "none", "network"
	// Only used when Type is "builtin"
	// +kubebuilder:validation:Enum=none;network
	// +optional
	Name string `json:"name,omitempty"`

	// ConfigMap references a ConfigMap containing permission profile configuration
	// Only used when Type is "configMap"
	// +optional
	ConfigMap *ConfigMapPermissionRef `json:"configMap,omitempty"`

	// Inline contains direct permission profile configuration
	// Only used when Type is "inline"
	// +optional
	Inline *PermissionProfileSpec `json:"inline,omitempty"`
}

// PermissionProfileSpec defines the permissions for an MCP server
type PermissionProfileSpec struct {
	// Read is a list of paths that the MCP server can read from
	// +optional
	// +listType=set
	Read []string `json:"read,omitempty"`

	// Write is a list of paths that the MCP server can write to
	// +optional
	// +listType=set
	Write []string `json:"write,omitempty"`

	// Network defines the network permissions for the MCP server
	// +optional
	Network *NetworkPermissions `json:"network,omitempty"`
}

// NetworkPermissions defines the network permissions for an MCP server
type NetworkPermissions struct {
	// Outbound defines the outbound network permissions
	// +optional
	Outbound *OutboundNetworkPermissions `json:"outbound,omitempty"`
}

// OutboundNetworkPermissions defines the outbound network permissions
type OutboundNetworkPermissions struct {
	// InsecureAllowAll allows all outbound network connections (not recommended)
	// +kubebuilder:default=false
	// +optional
	InsecureAllowAll bool `json:"insecureAllowAll,omitempty"`

	// AllowHost is a list of hosts to allow connections to
	// +optional
	// +listType=set
	AllowHost []string `json:"allowHost,omitempty"`

	// AllowPort is a list of ports to allow connections to
	// +optional
	// +listType=set
	AllowPort []int32 `json:"allowPort,omitempty"`
}

// OIDCConfigRef defines a reference to OIDC configuration
// +kubebuilder:validation:XValidation:rule="self.type == 'configMap' ? has(self.configMap) : true",message="configMap is required for configMap OIDC configuration"
// +kubebuilder:validation:XValidation:rule="self.type == 'inline' ? has(self.inline) : true",message="inline is required for inline OIDC configuration"
type OIDCConfigRef struct {
	// Type is the type of OIDC configuration
	// +kubebuilder:validation:Enum=kubernetes;configMap;inline
	// +kubebuilder:default=kubernetes
	Type string `json:"type"`

	// ResourceURL is the explicit resource URL for OAuth discovery endpoint (RFC 9728)
	// If not specified, defaults to the in-cluster Kubernetes service URL
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	ResourceURL string `json:"resourceURL,omitempty"`

	// Kubernetes configures OIDC for Kubernetes service account token validation
	// Only used when Type is "kubernetes"
	// +optional
	Kubernetes *KubernetesOIDCConfig `json:"kubernetes,omitempty"`

	// ConfigMap references a ConfigMap containing OIDC configuration
	// Only used when Type is "configMap"
	// +optional
	ConfigMap *ConfigMapOIDCRef `json:"configMap,omitempty"`

	// Inline contains direct OIDC configuration
	// Only used when Type is "inline"
	// +optional
	Inline *InlineOIDCConfig `json:"inline,omitempty"`
}

// KubernetesOIDCConfig configures OIDC for Kubernetes service account token validation
type KubernetesOIDCConfig struct {
	// ServiceAccount is deprecated and will be removed in a future release.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// Namespace is the namespace of the service account
	// If empty, uses the MCPServer's namespace
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Audience is the expected audience for the token
	// +kubebuilder:default=toolhive
	// +optional
	Audience string `json:"audience,omitempty"`

	// Issuer is the OIDC issuer URL
	// +kubebuilder:default="https://kubernetes.default.svc"
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// JWKSURL is the URL to fetch the JWKS from
	// If empty, OIDC discovery will be used to automatically determine the JWKS URL
	// +optional
	JWKSURL string `json:"jwksURL,omitempty"`

	// IntrospectionURL is the URL for token introspection endpoint
	// If empty, OIDC discovery will be used to automatically determine the introspection URL
	// +optional
	IntrospectionURL string `json:"introspectionURL,omitempty"`

	// UseClusterAuth enables using the Kubernetes cluster's CA bundle and service account token
	// When true, uses /var/run/secrets/kubernetes.io/serviceaccount/ca.crt for TLS verification
	// and /var/run/secrets/kubernetes.io/serviceaccount/token for bearer token authentication
	// Defaults to true if not specified
	// +optional
	UseClusterAuth *bool `json:"useClusterAuth"`
}

// ConfigMapOIDCRef references a ConfigMap containing OIDC configuration
type ConfigMapOIDCRef struct {
	// Name is the name of the ConfigMap
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key in the ConfigMap that contains the OIDC configuration
	// +kubebuilder:default=oidc.json
	// +kubebuilder:validation:MinLength=1
	// +optional
	Key string `json:"key,omitempty"`
}

// InlineOIDCConfig contains direct OIDC configuration
type InlineOIDCConfig struct {
	// Issuer is the OIDC issuer URL
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	Issuer string `json:"issuer"`

	// Audience is the expected audience for the token
	// +kubebuilder:validation:MinLength=1
	// +optional
	Audience string `json:"audience,omitempty"`

	// JWKSURL is the URL to fetch the JWKS from
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	JWKSURL string `json:"jwksURL,omitempty"`

	// IntrospectionURL is the URL for token introspection endpoint
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	IntrospectionURL string `json:"introspectionURL,omitempty"`

	// ClientID is deprecated and will be removed in a future release.
	// +optional
	ClientID string `json:"clientID,omitempty"`

	// ClientSecret is the client secret for introspection (optional)
	// +optional
	ClientSecret string `json:"clientSecret,omitempty"`

	// ThvCABundlePath is the path to CA certificate bundle file for HTTPS requests
	// The file must be mounted into the pod (e.g., via ConfigMap or Secret volume)
	// +optional
	ThvCABundlePath string `json:"thvCABundlePath,omitempty"`

	// JWKSAuthTokenPath is the path to file containing bearer token for JWKS/OIDC requests
	// The file must be mounted into the pod (e.g., via Secret volume)
	// +optional
	JWKSAuthTokenPath string `json:"jwksAuthTokenPath,omitempty"`

	// JWKSAllowPrivateIP allows JWKS/OIDC endpoints on private IP addresses
	// Use with caution - only enable for trusted internal IDPs
	// +kubebuilder:default=false
	// +optional
	JWKSAllowPrivateIP bool `json:"jwksAllowPrivateIP"`
}

// AuthzConfigRef defines a reference to authorization configuration
// +kubebuilder:validation:XValidation:rule="self.type == 'configMap' ? has(self.configMap) : true",message="configMap is required for configMap authorization configuration"
// +kubebuilder:validation:XValidation:rule="self.type == 'inline' ? has(self.inline) : true",message="inline is required for inline authorization configuration"
type AuthzConfigRef struct {
	// Type is the type of authorization configuration
	// +kubebuilder:validation:Enum=configMap;inline
	// +kubebuilder:default=configMap
	Type string `json:"type"`

	// ConfigMap references a ConfigMap containing authorization configuration
	// Only used when Type is "configMap"
	// +optional
	ConfigMap *ConfigMapAuthzRef `json:"configMap,omitempty"`

	// Inline contains direct authorization configuration
	// Only used when Type is "inline"
	// +optional
	Inline *InlineAuthzConfig `json:"inline,omitempty"`
}

// ConfigMapPermissionRef references a ConfigMap containing permission profile configuration
type ConfigMapPermissionRef struct {
	// Name is the name of the ConfigMap
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key in the ConfigMap that contains the permission profile configuration
	// +kubebuilder:default=permissions.json
	// +kubebuilder:validation:MinLength=1
	// +optional
	Key string `json:"key,omitempty"`
}

// ConfigMapAuthzRef references a ConfigMap containing authorization configuration
type ConfigMapAuthzRef struct {
	// Name is the name of the ConfigMap
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key in the ConfigMap that contains the authorization configuration
	// +kubebuilder:default=authz.json
	// +kubebuilder:validation:MinLength=1
	// +optional
	Key string `json:"key,omitempty"`
}

// InlineAuthzConfig contains direct authorization configuration
type InlineAuthzConfig struct {
	// Policies is a list of Cedar policy strings
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	Policies []string `json:"policies"`

	// EntitiesJSON is a JSON string representing Cedar entities
	// +kubebuilder:default="[]"
	// +optional
	EntitiesJSON string `json:"entitiesJSON,omitempty"`
}

// MCPServerStatus defines the observed state of MCPServer
type MCPServerStatus struct {
	// Conditions represent the latest available observations of the MCPServer's state
	// Standard condition types: Ready, Available, Progressing
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL is the URL where the MCP server can be accessed
	// For Hosted servers, this is the cluster-internal or external service URL
	// For Remote servers, this reflects the configured external URL
	// +optional
	URL string `json:"url,omitempty"`

	// Phase is the current phase of the MCPServer lifecycle
	// +optional
	Phase MCPServerPhase `json:"phase,omitempty"`

	// Message provides additional information about the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the most recently observed number of replicas for hosted servers
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas for hosted servers
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// MCPServerPhase is the phase of the MCPServer
// +kubebuilder:validation:Enum=Pending;Starting;Running;Updating;Failed;Terminating
type MCPServerPhase string

const (
	// MCPServerPhasePending means the MCPServer is being created
	MCPServerPhasePending MCPServerPhase = "Pending"

	// MCPServerPhaseStarting means the MCPServer is starting up
	MCPServerPhaseStarting MCPServerPhase = "Starting"

	// MCPServerPhaseRunning means the MCPServer is running and ready
	MCPServerPhaseRunning MCPServerPhase = "Running"

	// MCPServerPhaseUpdating means the MCPServer is being updated
	MCPServerPhaseUpdating MCPServerPhase = "Updating"

	// MCPServerPhaseFailed means the MCPServer failed to start or run
	MCPServerPhaseFailed MCPServerPhase = "Failed"

	// MCPServerPhaseTerminating means the MCPServer is being deleted
	MCPServerPhaseTerminating MCPServerPhase = "Terminating"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="has(self.spec.type) && (self.spec.type == 'Hosted' ? has(self.spec.image) && self.spec.image != '' : true)",message="image is required for Hosted MCP servers"
// +kubebuilder:validation:XValidation:rule="has(self.spec.type) && (self.spec.type == 'Remote' ? has(self.spec.url) && self.spec.url != '' : true)",message="url is required for Remote MCP servers"

// MCPServer is the Schema for the mcpservers API
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"` // nolint:revive
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec MCPServerSpec `json:"spec,omitempty"`
	// +optional
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="has(self.spec.type) && (self.spec.type == 'Hosted' ? has(self.spec.image) && self.spec.image != '' : true)",message="image is required for Hosted MCP servers"
// +kubebuilder:validation:XValidation:rule="has(self.spec.type) && (self.spec.type == 'Remote' ? has(self.spec.url) && self.spec.url != '' : true)",message="url is required for Remote MCP servers"

// ClusterMCPServer is the cluster-scoped Schema for the mcpservers API
type ClusterMCPServer struct {
	metav1.TypeMeta   `json:",inline"` // nolint:revive
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec MCPServerSpec `json:"spec,omitempty"`
	// +optional
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"` // nolint:revive
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

// +kubebuilder:object:root=true

// ClusterMCPServerList contains a list of ClusterMCPServer
type ClusterMCPServerList struct {
	metav1.TypeMeta `json:",inline"` // nolint:revive
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterMCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
	SchemeBuilder.Register(&ClusterMCPServer{}, &ClusterMCPServerList{})
}
