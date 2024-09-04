// NOTE: Boilerplate only.  Ignore this file.

// Package v1beta1 contains API Schema definitions for the serving v1beta1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=ome/pkg/apis/serving
// +k8s:defaulter-gen=TypeMeta
// +groupName=ome.io
package v1beta1

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// APIVersion is the current API version used to register these objects
	APIVersion = "v1beta1"

	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: constants.OMEAPIGroupName, Version: APIVersion}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme is required by pkg/client/...
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource is required by pkg/client/listers/...
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
