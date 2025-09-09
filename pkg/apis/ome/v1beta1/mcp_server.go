package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPTransportType defines the transport method for MCP communication.
// +kubebuilder:validation:Enum=stdio;streamable-http;sse
type MCPTransportType string

const (
	// MCPTransportStdio uses standard input/output for communication.
	MCPTransportStdio MCPTransportType = "stdio"
	// MCPTransportStreamableHTTP uses HTTP with streaming support.
	MCPTransportStreamableHTTP MCPTransportType = "streamable-http"
	// MCPTransportSSE uses Server-Sent Events for communication.
	MCPTransportSSE MCPTransportType = "sse"
)

// MCPServerSpec defines the desired state of an MCPServer.
// An MCPServer can either be 'Hosted' within the cluster or a 'Remote' external service.
// +kubebuilder:validation:XValidation:rule="has(self.hosted) || has(self.remote)", message="either hosted or remote must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.hosted) && has(self.remote))", message="hosted and remote are mutually exclusive"
type MCPServerSpec struct {
	// Hosted defines a server that runs as pods within the cluster.
	// +optional
	Hosted *HostedMCPServer `json:"hosted,omitempty"`

	// Remote defines a server that is accessed via an external URL.
	// +optional
	Remote *RemoteMCPServer `json:"remote,omitempty"`

	// Transport specifies the transport protocol for MCP communication.
	// +kubebuilder:default=stdio
	// +optional
	Transport MCPTransportType `json:"transport,omitempty"`

	// Capabilities defines the features supported by this server.
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`

	// Version of the MCP server software.
	// +optional
	Version string `json:"version,omitempty"`

	// PermissionProfile defines the operational permissions for the server.
	// +optional
	PermissionProfile *PermissionProfileSource `json:"permissionProfile,omitempty"`

	// OIDCConfig defines OIDC authentication for authenticating clients.
	// +optional
	OIDCConfig *OIDCConfigSource `json:"oidcConfig,omitempty"`

	// AuthzConfig defines authorization policies for the server.
	// +optional
	AuthzConfig *AuthzConfigSource `json:"authzConfig,omitempty"`

	// ToolsFilter restricts the tools exposed by this server.
	// +optional
	// +listType=set
	ToolsFilter []string `json:"toolsFilter,omitempty"`
}

// HostedMCPServer defines a server that runs as pods in the cluster.
type HostedMCPServer struct {
	// PodTemplateSpec defines the pod template to use for the MCP server.
	PodTemplateSpec corev1.PodTemplateSpec `json:"podTemplateSpec"`

	// Replicas is the number of desired replicas for the server.
	// Only applicable for servers with network-based transports (e.g., http, sse).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// RemoteMCPServer defines a server that is accessed via an external URL.
type RemoteMCPServer struct {
	// URL is the external URL of the remote MCP server.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`
}

// MCPCapabilities defines the features supported by the MCP server.
type MCPCapabilities struct {
	// Tools indicates whether the server supports tool execution.
	// +kubebuilder:default=true
	// +optional
	Tools *bool `json:"tools,omitempty"`

	// Resources indicates whether the server supports exposing resources.
	// +kubebuilder:default=false
	// +optional
	Resources *bool `json:"resources,omitempty"`

	// Prompts indicates whether the server supports prompt elicitation.
	// +kubebuilder:default=false
	// +optional
	Prompts *bool `json:"prompts,omitempty"`
}

// PermissionProfileSource defines the source of a permission profile.
// Only one of the fields may be set.
// +kubebuilder:validation:XValidation:rule="(has(self.builtin) + has(self.configMap) + has(self.inline)) <= 1",message="at most one of builtin, configMap, or inline can be set"
type PermissionProfileSource struct {
	// Builtin selects a pre-defined, named permission profile.
	// +optional
	Builtin *BuiltinPermissionProfile `json:"builtin,omitempty"`

	// ConfigMap references a ConfigMap containing a permission profile specification.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Inline contains an embedded permission profile specification.
	// +optional
	Inline *PermissionProfileSpec `json:"inline,omitempty"`
}

// BuiltinPermissionProfile defines a built-in permission profile.
type BuiltinPermissionProfile struct {
	// Name of the built-in permission profile.
	// +kubebuilder:validation:Enum=none;network-only;full-access
	Name string `json:"name"`
}

// PermissionProfileSpec defines the permissions for an MCP server.
type PermissionProfileSpec struct {
	// Allow specifies the permissions granted to the server.
	// +listType=atomic
	Allow []PermissionRule `json:"allow"`
}

// PermissionRule defines a single permission grant.
type PermissionRule struct {
	// KubeResources defines permissions for accessing Kubernetes resources.
	// +optional
	KubeResources *KubeResourcePermission `json:"kubeResources,omitempty"`
	// Network defines permissions for making outbound network calls.
	// +optional
	Network *NetworkPermission `json:"network,omitempty"`
}

// KubeResourcePermission defines permissions for a set of Kubernetes resources.
type KubeResourcePermission struct {
	// APIGroups is the list of API groups. "*" means all.
	// +listType=set
	APIGroups []string `json:"apiGroups"`
	// Resources is the list of resource names. "*" means all.
	// +listType=set
	Resources []string `json:"resources"`
	// Verbs is the list of allowed verbs.
	// +listType=set
	Verbs []string `json:"verbs"`
}

// NetworkPermission defines outbound network permissions.
type NetworkPermission struct {
	// AllowHost is a list of glob patterns for hosts to allow connections to.
	// +listType=set
	AllowHost []string `json:"allowHost"`
}

// OIDCConfigSource defines the source of OIDC configuration.
// Only one of the fields may be set.
// +kubebuilder:validation:XValidation:rule="(has(self.kubernetes) + has(self.inline)) <= 1",message="at most one of kubernetes or inline can be set"
type OIDCConfigSource struct {
	// Kubernetes configures OIDC to validate Kubernetes service account tokens.
	// +optional
	Kubernetes *KubernetesOIDCConfig `json:"kubernetes,omitempty"`

	// Inline contains a direct OIDC provider configuration.
	// +optional
	Inline *InlineOIDCConfig `json:"inline,omitempty"`
}

// KubernetesOIDCConfig configures OIDC for Kubernetes service account token validation.
type KubernetesOIDCConfig struct {
	// Issuer is the OIDC issuer URL of the Kubernetes cluster.
	// If not specified, it defaults to the cluster's issuer URL.
	// +optional
	Issuer string `json:"issuer,omitempty"`
}

// InlineOIDCConfig contains direct OIDC provider configuration.
type InlineOIDCConfig struct {
	// Issuer is the OIDC issuer URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	Issuer string `json:"issuer"`

	// Audience is the expected audience for the token.
	// +optional
	Audience string `json:"audience,omitempty"`

	// JWKSURL is the URL to fetch the JSON Web Key Set from.
	// If empty, OIDC discovery will be used.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	JWKSURL string `json:"jwksURL,omitempty"`
}

// AuthzConfigSource defines the source of an authorization policy.
// Only one of the fields may be set.
// +kubebuilder:validation:XValidation:rule="(has(self.configMap) + has(self.inline)) <= 1",message="at most one of configMap or inline can be set"
type AuthzConfigSource struct {
	// ConfigMap references a ConfigMap containing the authorization policy.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Inline contains an embedded authorization policy.
	// +optional
	Inline *InlineAuthzConfig `json:"inline,omitempty"`
}

// InlineAuthzConfig contains an embedded authorization policy.
type InlineAuthzConfig struct {
	// Policies is a list of Cedar policy strings.
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	Policies []string `json:"policies"`

	// EntitiesJSON is a JSON string representing Cedar entities.
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

// MCPServer is the Schema for the mcpservers API
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   MCPServerSpec   `json:"spec"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterMCPServer is the cluster-scoped Schema for the mcpservers API
type ClusterMCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   MCPServerSpec   `json:"spec"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

// +kubebuilder:object:root=true

// ClusterMCPServerList contains a list of ClusterMCPServer
type ClusterMCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterMCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
	SchemeBuilder.Register(&ClusterMCPServer{}, &ClusterMCPServerList{})
}
