package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCIRedisCluster is the Schema for the RedisCluster API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="Lifecycle_State",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type OciRedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OciRedisClusterSpec   `json:"spec"`
	Status            OciRedisClusterStatus `json:"status,omitempty"`
}

type OciRedisClusterStatus struct {
	// LifecycleState represents the current lifecycle state of the oci_redis_cluster (e.g., READY, CREATING)
	// +optional
	LifecycleState  LifecycleState `json:"lifecycleState,omitempty"`
	RedisClusterId  string         `json:"redisClusterId,omitempty"`
	SecretName      string         `json:"secretName,omitempty"`
	SecretNamespace string         `json:"secretNamespace,omitempty"`
	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type OciRedisClusterSpec struct {
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

// OCIRedisClusterList contains a list of Redis Cluster
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OciRedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciRedisCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OciRedisCluster{}, &OciRedisClusterList{})
}
