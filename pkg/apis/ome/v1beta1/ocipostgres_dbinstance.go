package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type OciPostgresDBInstanceSpec struct {
	// Reference to owning cluster ID
	DBClusterId string `json:"dbClusterId"`
	// +optional
	// AuthType selects how we authenticate to OCI.
	// +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
	// +kubebuilder:default=InstancePrincipal
	AuthType string `json:"authType,omitempty"`
	// +required
	// AdminSecreteName used to fetch admin credential for the cluster to provision db instance
	AdminSecretName string `json:"adminSecretName"`
	// +required
	AdminSecretNamespace string `json:"adminSecretNamespace"`
}

type OciPostgresDBInstanceStatus struct {
	// IsReady indicates whether the storage is ready for use.
	// +required
	IsReady bool `json:"isReady"`
	// LifecycleState represents the current lifecycle state of the PostgreSQLStatus (e.g., READY, CREATING)
	// +optional
	LifecycleState         LifecycleState `json:"lifecycleState,omitempty"`
	Endpoint               string         `json:"endpoint,omitempty"`
	AppUserSecretName      string         `json:"appUserSecretName,omitempty"`
	AppUserSecretNamespace string         `json:"appUserSecretNamespace,omitempty"`
	DatabaseName           string         `json:"databaseName,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OciPostgresDBInstance is the Schema for the PostgresDBInstance API
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Lifecycle_State",type="string",JSONPath=".status.lifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type OciPostgresDBInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OciPostgresDBInstanceSpec   `json:"spec,omitempty"`
	Status            OciPostgresDBInstanceStatus `json:"status,omitempty"`
}

// OciPostgresDBInstanceList contains a list of PostgresDBInstance
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type OciPostgresDBInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciPostgresDBInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OciPostgresDBInstance{}, &OciPostgresDBInstanceList{})
}
