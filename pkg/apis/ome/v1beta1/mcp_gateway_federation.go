package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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