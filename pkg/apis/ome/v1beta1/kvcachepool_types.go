package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// KVCachePoolDeploymentMode describes how OME reconciles a KVCachePool.
// +kubebuilder:validation:Enum=RawDeployment;NodeLocal;DistributedStore;ProviderManaged
type KVCachePoolDeploymentMode string

const (
	KVCachePoolRawDeployment    KVCachePoolDeploymentMode = "RawDeployment"
	KVCachePoolNodeLocal        KVCachePoolDeploymentMode = "NodeLocal"
	KVCachePoolDistributedStore KVCachePoolDeploymentMode = "DistributedStore"
	KVCachePoolProviderManaged  KVCachePoolDeploymentMode = "ProviderManaged"
)

// KVCacheProvider identifies the primary KV cache integration layer.
// +kubebuilder:validation:Enum=LMCache;Mooncake;NIXL
type KVCacheProvider string

const (
	KVCacheProviderLMCache  KVCacheProvider = "LMCache"
	KVCacheProviderMooncake KVCacheProvider = "Mooncake"
	KVCacheProviderNIXL     KVCacheProvider = "NIXL"
)

// KVCacheBackendType identifies a storage or transfer backend used underneath a provider.
// +kubebuilder:validation:Enum=Local;Mooncake;NIXL;Redis;Filesystem
type KVCacheBackendType string

const (
	KVCacheBackendLocal      KVCacheBackendType = "Local"
	KVCacheBackendMooncake   KVCacheBackendType = "Mooncake"
	KVCacheBackendNIXL       KVCacheBackendType = "NIXL"
	KVCacheBackendRedis      KVCacheBackendType = "Redis"
	KVCacheBackendFilesystem KVCacheBackendType = "Filesystem"
)

// KVCacheEvictionPolicy is the desired cache eviction behavior. Leaving the
// field unset selects the provider's default policy.
// +kubebuilder:validation:Enum=LRU;LFU;FIFO
type KVCacheEvictionPolicy string

const (
	KVCacheEvictionLRU  KVCacheEvictionPolicy = "LRU"
	KVCacheEvictionLFU  KVCacheEvictionPolicy = "LFU"
	KVCacheEvictionFIFO KVCacheEvictionPolicy = "FIFO"
)

// KVCachePoolSpec defines the desired state of a KVCachePool.
type KVCachePoolSpec struct {
	// Provider identifies the primary KV cache integration layer.
	// +required
	Provider KVCacheProviderSpec `json:"provider"`

	// DeploymentMode describes how OME reconciles this pool.
	// +required
	DeploymentMode KVCachePoolDeploymentMode `json:"deploymentMode"`

	// Cache describes provider-neutral cache policy.
	// +optional
	Cache *KVCachePolicySpec `json:"cache,omitempty"`

	// Workloads contains pod and container configuration for OME-managed pool
	// roles. This is the only place where pool pod/container config appears.
	// +optional
	// +listType=map
	// +listMapKey=name
	Workloads []KVCachePoolWorkloadSpec `json:"workloads,omitempty"`
}

// KVCacheProviderSpec identifies the primary provider for a KVCachePool and any
// storage or transfer backends used underneath the provider.
type KVCacheProviderSpec struct {
	// +required
	Name KVCacheProvider `json:"name"`

	// Backends identifies storage or transfer backends used by the provider.
	// +optional
	// +listType=map
	// +listMapKey=name
	Backends []KVCacheBackendSpec `json:"backends,omitempty"`

	// Config contains provider-scoped settings that are not portable OME API.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`
}

// KVCacheBackendSpec identifies a storage or transfer backend used by a provider.
type KVCacheBackendSpec struct {
	// +required
	Name string `json:"name"`

	// Type identifies the backend implementation.
	// +required
	Type KVCacheBackendType `json:"type"`

	// Config contains backend-scoped settings that are not portable OME API.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`
}

// KVCachePolicySpec captures provider-neutral cache policy. Providers translate
// these settings to their native size, memory, segment, or storage knobs.
type KVCachePolicySpec struct {
	// Capacity is the intended total pool capacity.
	// +optional
	Capacity *resource.Quantity `json:"capacity,omitempty"`

	// EvictionPolicy is the desired eviction behavior.
	// +optional
	EvictionPolicy *KVCacheEvictionPolicy `json:"evictionPolicy,omitempty"`

	// ChunkSize is a provider-neutral chunk/page/block size hint.
	// +optional
	ChunkSize *resource.Quantity `json:"chunkSize,omitempty"`
}

// KVCachePoolWorkloadSpec describes pod and container configuration for an
// OME-managed pool role such as server, master, or store. It composes the same
// PodSpec and ComponentExtensionSpec used by engine, decoder, and router
// components so pool workloads inherit consistent replica, autoscaling, label,
// and PDB semantics.
type KVCachePoolWorkloadSpec struct {
	// Name identifies the provider or backend role, such as server, master, or
	// store.
	// +required
	Name string `json:"name"`

	// PodSpec provides pod-level customization for this pool workload.
	// Container configuration is expressed through PodSpec.Containers.
	// +optional
	PodSpec `json:",inline"`

	// ComponentExtensionSpec provides replicas, autoscaling, labels,
	// annotations, and PodDisruptionBudget configuration for this workload.
	ComponentExtensionSpec `json:",inline"`
}

// KVCachePoolRef is a reference from another resource to a KVCachePool.
type KVCachePoolRef struct {
	// Name of the KVCachePool being referenced.
	// +required
	Name string `json:"name"`

	// Kind of the referenced resource. Defaults to KVCachePool.
	// +optional
	// +kubebuilder:default="KVCachePool"
	Kind *string `json:"kind,omitempty"`

	// APIGroup of the referenced resource. Defaults to ome.io.
	// +optional
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`
}

// KVCacheConnectorSpec describes runtime-side support for attaching serving
// components to a KVCachePool with a specific provider.
type KVCacheConnectorSpec struct {
	// Provider identifies the pool provider this connector supports.
	// +required
	Provider KVCacheProvider `json:"provider"`

	// DeploymentModes lists supported pool deployment modes. An empty list
	// means all modes supported by this provider adapter.
	// +optional
	// +listType=atomic
	DeploymentModes []KVCachePoolDeploymentMode `json:"deploymentModes,omitempty"`

	// Components provides component-specific connector configuration. Keys
	// must be one of the InferenceService ComponentType values (engine,
	// decoder, router, predictor); other keys are rejected at admission.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['engine','decoder','router','predictor'])",message="components key must be one of engine, decoder, router, predictor"
	Components map[ComponentType]KVCacheConnectorComponentSpec `json:"components,omitempty"`
}

// KVCacheConnectorComponentSpec configures runtime-side connector behavior for
// a single serving component.
type KVCacheConnectorComponentSpec struct {
	// ConnectorConfig is typed connector intent interpreted by the
	// provider/runtime adapter.
	// +optional
	ConnectorConfig *KVCacheConnectorConfig `json:"connectorConfig,omitempty"`

	// RuntimeArgsOverride provides connector-specific runtime args. Matching
	// args replace existing values; missing args are appended.
	// +optional
	// +listType=atomic
	RuntimeArgsOverride []string `json:"runtimeArgsOverride,omitempty"`

	// EnvironmentOverride provides connector-specific environment variables.
	// +optional
	EnvironmentOverride map[string]string `json:"environmentOverride,omitempty"`
}

// KVCacheConnectorConfig mirrors the runtime --kv-transfer-config JSON
// payload. Either set the inline fields or reference a ConfigMap holding the
// full JSON.
type KVCacheConnectorConfig struct {
	// ConnectorClass maps to "kv_connector".
	// +optional
	ConnectorClass *string `json:"connectorClass,omitempty"`

	// Role maps to "kv_role".
	// +optional
	Role *string `json:"role,omitempty"`

	// ExtraConfig maps to "kv_connector_extra_config".
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtraConfig *runtime.RawExtension `json:"extraConfig,omitempty"`

	// ConfigMapRef sources the full --kv-transfer-config JSON from a
	// ConfigMap; when set, the inline fields are ignored.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`
}

// KVCachePoolStatus describes the observed state of a KVCachePool.
type KVCachePoolStatus struct {
	// Conditions for the KVCachePool. The controller sets the standard `Ready`
	// condition along with any provider- or workload-specific conditions.
	duckv1.Status `json:",inline"`

	// Connection contains normalized connection information consumed by
	// ServingRuntime connector adapters.
	// +optional
	Connection *KVCachePoolConnectionStatus `json:"connection,omitempty"`

	// Workloads reports provider workload status.
	// +optional
	// +listType=map
	// +listMapKey=name
	Workloads []KVCachePoolWorkloadStatus `json:"workloads,omitempty"`
}

// KVCachePoolConnectionStatus normalizes connection information for
// runtime-side connector injection.
type KVCachePoolConnectionStatus struct {
	// Endpoint is the primary in-cluster endpoint when one exists.
	// +optional
	Endpoint *apis.URL `json:"endpoint,omitempty"`

	// Ports lists named connection ports.
	// +optional
	// +listType=map
	// +listMapKey=name
	Ports []KVCachePoolPortStatus `json:"ports,omitempty"`

	// ConfigMapRef points to provider-generated connection config when needed.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// SecretRef points to provider-generated credentials when needed.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// ProviderStatus contains provider-scoped observed state, not desired
	// configuration.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
}

// KVCachePoolPortStatus is a named connection port published by a pool.
type KVCachePoolPortStatus struct {
	// +required
	Name string `json:"name"`
	// +required
	Port int32 `json:"port"`
}

// KVCachePoolWorkloadStatus reports the observed state of a pool workload
// role.
type KVCachePoolWorkloadStatus struct {
	// +required
	Name            string `json:"name"`
	ReadyReplicas   int32  `json:"readyReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
}

// KVCachePool is the Schema for distributed KV cache pools.
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider.name"
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.deploymentMode"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=kvcachepools,shortName=kvcp
type KVCachePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KVCachePoolSpec   `json:"spec,omitempty"`
	Status KVCachePoolStatus `json:"status,omitempty"`
}

// KVCachePoolList contains a list of KVCachePool.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type KVCachePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KVCachePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KVCachePool{}, &KVCachePoolList{})
}
