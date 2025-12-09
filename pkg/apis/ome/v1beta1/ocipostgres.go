package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OciPostgres is the Schema for the PostgresDB API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="Lifecycle_State",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type OciPostgres struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OciPostgresSpec   `json:"spec"`
	Status            OciPostgresStatus `json:"status,omitempty"`
}

type OciPostgresStatus struct {
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
	// +optional
	// +listType=map
	// +listMapKey=databaseName
	DbInstances []DbInstanceStatus `json:"dbInstances,omitempty"`
}

type DbInstanceStatus struct {
	// +required
	DatabaseName           string `json:"databaseName"`
	AppUserSecretName      string `json:"appUserSecretName,omitempty"`
	AppUserSecretNamespace string `json:"appUserSecretNamespace,omitempty"`
	// LifecycleState represents the current lifecycle state of the ocipostgres_dbinstance (e.g., READY, CREATING)
	// +optional
	LifecycleState LifecycleState `json:"lifecycleState,omitempty"`
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

type OciPostgresSpec struct {
	ClusterSpec    OciPostgresClusterSpec `json:"clusterSpec"`
	DbInstanceSpec DbInstanceSpec         `json:"dbInstanceSpec"`
}

type DbInstanceSpec struct {
	// +required
	// +listType=set
	LogicalDatabases []string `json:"logicalDatabases"`
}

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
	DbVersion string `json:"dbVersion,omitempty"`

	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// +optional
	// +kubebuilder:default="PostgreSQL.VM.Standard.E5.Flex"
	Shape string `json:"shape,omitempty"`

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
	StorageDetails DbSystemStorageDetails `json:"storageDetails,omitempty"`

	// +optional
	Description string `json:"description,omitempty"`
}

type DbSystemNetworkDetails struct {
	// +required
	SubnetId string `json:"subnetId"`

	// +required
	// +listType=atomic
	NsgIds []string `json:"nsgIds"`

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

// PostgresList contains a list of PostgresDBCluster
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OciPostgresList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciPostgres `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OciPostgres{}, &OciPostgresList{})
}
