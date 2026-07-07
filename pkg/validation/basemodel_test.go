package validation

import (
	"testing"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestValidatePVCStorage(t *testing.T) {
	tests := []struct {
		name            string
		spec            *v1beta1.BaseModelSpec
		isClusterScoped bool
		wantErr         bool
	}{
		{name: "nil spec is allowed", spec: nil},
		{name: "no storage is allowed", spec: &v1beta1.BaseModelSpec{}},
		{
			name: "non-pvc URI is allowed",
			spec: &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("oci://n/ns/b/bucket/o/path")}},
		},
		{
			name: "valid namespaced PVC URI",
			spec: &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("pvc://my-pvc/models/llama")}},
		},
		{
			name:            "valid cluster-scoped PVC URI",
			spec:            &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("pvc://shared:my-pvc/models/llama")}},
			isClusterScoped: true,
		},
		{
			name:    "namespaced BaseModel must not include namespace prefix",
			spec:    &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("pvc://shared:my-pvc/models/llama")}},
			wantErr: true,
		},
		{
			name:            "ClusterBaseModel must include namespace prefix",
			spec:            &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("pvc://my-pvc/models/llama")}},
			isClusterScoped: true,
			wantErr:         true,
		},
		{
			name:    "malformed PVC URI",
			spec:    &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: ptr.To("pvc://")}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePVCStorage(tc.spec, tc.isClusterScoped)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
