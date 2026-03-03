package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPTransportType defines the transport protocol for MCP communication.
// +kubebuilder:validation:Enum=stdio;streamable-http;sse
type MCPTransportType string

const (
	TransportStdio          MCPTransportType = "stdio"
	TransportStreamableHTTP MCPTransportType = "streamable-http"
	TransportSSE            MCPTransportType = "sse"
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

	// ToolsFilter restricts the tools exposed by this server.
	// +optional
	// +listType=set
	ToolsFilter []string `json:"toolsFilter,omitempty"`
}

type HostedMCPServer struct {
	// PodSpec defines the pod template to use for the MCP server.
	PodSpec corev1.PodTemplateSpec `json:"podSpec"`

	// Replicas is the number of desired replicas for the server.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

type RemoteMCPServer struct {
	// URL is the external URL of the remote MCP server.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`
}

type MCPCapabilities struct {
	// Add capabilities here as needed
	Tools     bool `json:"tools,omitempty"`
	Resources bool `json:"resources,omitempty"`
	Prompts   bool `json:"prompts,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.builtin) ? 1 : 0) + (has(self.configMap) ? 1 : 0) + (has(self.inline) ? 1 : 0) <= 1",message="at most one of builtin, configMap, or inline can be set"
type PermissionProfileSource struct {
	Builtin   *BuiltinPermissionProfile    `json:"builtin,omitempty"`
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`
	Inline    *PermissionProfileSpec       `json:"inline,omitempty"`
}

type BuiltinPermissionProfile string

type PermissionProfileSpec struct {
	// Allow specifies the permissions granted to the server.
	// +listType=atomic
	Allow []PermissionRule `json:"allow"`
}

type PermissionRule struct {
	KubeResources *KubeResourcePermission `json:"kubeResources,omitempty"`
	Network       *NetworkPermission      `json:"network,omitempty"`
}

type KubeResourcePermission struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
	// Namespaces restricts permissions to specific namespaces
	// If empty, permissions apply to the MCPServer's namespace only
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
}

type NetworkPermission struct {
	// AllowHost specifies allowed destination hosts
	// Supports wildcards: "*.internal.svc.cluster.local"
	// +optional
	AllowHost []string `json:"allowHost,omitempty"`

	// AllowCIDR specifies allowed destination CIDR blocks (optional for future)
	// +optional
	AllowCIDR []string `json:"allowCIDR,omitempty"`
}

type MCPServerStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// NetworkEnforcement indicates how network permissions are enforced
	// +kubebuilder:validation:Enum=NetworkPolicy;ServiceMesh;None
	// +optional
	NetworkEnforcement *string `json:"networkEnforcement,omitempty"`

	// Ready indicates the server is ready to accept requests
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mcps
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`

// MCPServer is the Schema for the mcpservers API
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPServerSpec   `json:"spec,omitempty"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
}
