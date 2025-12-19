package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCICache is the Schema for the RedisCluster API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="Lifecycle_State",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type OciCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OciCacheSpec   `json:"spec"`
	Status            OciCacheStatus `json:"status,omitempty"`
}

type OciCacheStatus struct {
	// LifecycleState represents the current lifecycle state of the oci_redis_cluster (e.g., READY, CREATING)
	// +optional
	LifecycleState           LifecycleState `json:"lifecycleState,omitempty"`
	CacheClusterId           string         `json:"cacheClusterId,omitempty"`
	PrimaryFqdn              string         `json:"primaryFqdn,omitempty"`
	PrimaryEndpointIpAddress string         `json:"primaryEndpointIpAddress,omitempty"`
	SecretName               string         `json:"secretName,omitempty"`
	SecretNamespace          string         `json:"secretNamespace,omitempty"`
	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=cacheUserName
	CacheUsers []CacheUserStatus `json:"cacheUsers,omitempty"`
}

type CacheUserStatus struct {
	// +required
	CacheUserName string `json:"cacheUserName,omitempty"`
	// CacheUserID is the identifier of the user in the underlying OCI Cache service,
	// +optional
	CacheUserId string `json:"cacheUserId,omitempty"`
	// UserSecretName is the name of the Secret that stores this user's credentials.
	// +optional
	UserSecretName string `json:"userSecretName,omitempty"`
	// UserSecretNamespace is the namespace where the credentials Secret is stored.
	// +optional
	UserSecretNamespace string `json:"userSecretNamespace,omitempty"`
	// IsUserAttached indicates whether the user has been successfully attached
	// to the configured Redis cluster at least once.
	// +optional
	IsUserAttached bool `json:"isUserAttached,omitempty"`
}

type OciCacheSpec struct {
	ClusterSpec   CacheClusterSpec `json:"clusterSpec"`
	CacheUserSpec CacheUserSpec    `json:"cacheUserSpec"`
}

type CacheUserSpec struct {
	// +required
	// +listType=set
	CacheUsers []string `json:"cacheUsers"`
}

type CacheClusterSpec struct {
	// +required
	CompartmentId string `json:"compartmentId"`

	// +optional
	// AuthType selects how we authenticate to OCI.
	// +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
	// +kubebuilder:default=InstancePrincipal
	AuthType string `json:"authType,omitempty"`

	// +optional
	DisplayName string `json:"displayName"`

	// +required
	SubnetId string `json:"subnetId"`

	// +optional
	// +listType=atomic
	NsgIds []string `json:"nsgIds,omitempty"`

	// +optional
	// +kubebuilder:default=3
	NodeCount int `json:"nodeCount,omitempty"`

	// +optional
	// +kubebuilder:validation:Enum=V7_0_5;REDIS_7_0
	// +kubebuilder:default="V7_0_5"
	SoftwareVersion string `json:"softwareVersion,omitempty"`

	// +optional
	// +kubebuilder:default="16"
	NodeMemoryInGBs string `json:"nodeMemoryInGBs,omitempty"`
}

// OCIRedisList contains a list of Redis Cluster
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OciCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciCache `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OciCache{}, &OciCacheList{})
}
