package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCIPostgresDBCluster is the Schema for the PostgresDBCluster API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="Lifecycle_State",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type OciPostgresCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OciPostgresClusterSpec   `json:"spec"`
	Status            OciPostgresClusterStatus `json:"status,omitempty"`
}

type OciPostgresClusterStatus struct {
	// LifecycleState represents the current lifecycle state of the ocipostgres_cluster (e.g., READY, CREATING)
	// +optional
	LifecycleState       LifecycleState `json:"lifecycleState,omitempty"`
	DbSystemId           string         `json:"dbSystemId,omitempty"`
	Endpoint             string         `json:"endpoint,omitempty"`
	AdminSecretName      string         `json:"adminSecretName,omitempty"`
	AdminSecretNamespace string         `json:"adminSecretNamespace,omitempty"`
	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// LifecycleState enum
// +kubebuilder:validation:Enum=READY;CREATING;DELETING;DELETED;FAILED;UPDATING
type LifecycleState string

const (
	LifecycleStateReady    LifecycleState = "READY"
	LifecycleStateCreating LifecycleState = "CREATING"
	LifecycleStateDeleting LifecycleState = "DELETING"
	LifecycleStateDeleted  LifecycleState = "DELETED"
	LifecycleStateFailed   LifecycleState = "FAILED"
	LifecycleStateUpdating LifecycleState = "UPDATING"
)

type OciPostgresClusterSpec struct {
	// +required
	CompartmentId string `json:"compartmentId"`

	// +required
	NetworkDetails DbSystemNetworkDetails `json:"networkDetails"`

	// +optional
	// AuthType selects how we authenticate to OCI.
	// +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
	// +kubebuilder:default=InstancePrincipal
	AuthType string `json:"authType,omitempty"`

	// +optional
	// +kubebuilder:default="16"
	DbVersion string `json:"dbVersion"`

	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// +optional
	// +kubebuilder:default="PostgreSQL.VM.Standard.E5.Flex"
	Shape string `json:"shape"`

	// # of nodes (writer + replicas). Size must match InstancesDetails if provided.
	// +optional
	// +kubebuilder:default=1
	InstanceCount int `json:"instanceCount,omitempty"`

	// per-node OCPU for Flex shapes.
	// +optional
	// +kubebuilder:default=2
	InstanceOcpuCount *int `json:"instanceOcpuCount,omitempty"`

	// per-node memory (GiB) for Flex shapes.
	// +optional
	// +kubebuilder:default=16
	InstanceMemorySizeInGbs *int `json:"instanceMemorySizeInGbs,omitempty"`

	// +optional
	StorageDetails DbSystemStorageDetails `json:"storageDetails"`

	// +optional
	Description string `json:"description,omitempty"`
}

type DbSystemNetworkDetails struct {
	// +required
	SubnetId string `json:"subnetId"`

	// +optional
	// +listType=atomic
	NsgIds []string `json:"nsgIds,omitempty"`

	// +optional
	PrimaryDbEndpointPrivateIp string `json:"primaryDbEndpointPrivateIp,omitempty"`
}

type DbSystemStorageDetails struct {
	// choose regional (true) or AD-local (false)
	// +optional
	// +kubebuilder:default=false
	IsRegionallyDurable bool `json:"isRegionallyDurable"`

	// +optional
	AvailabilityDomain string `json:"availabilityDomain,omitempty"`
}

// PostgresDBClusterList contains a list of PostgresDBCluster
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OciPostgresClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciPostgresCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OciPostgresCluster{}, &OciPostgresClusterList{})
}
