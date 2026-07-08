package basemodel

import (
	"context"
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func newDecoder(t *testing.T) admission.Decoder {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return admission.NewDecoder(scheme)
}

func TestBaseModelValidator_Handle(t *testing.T) {
	v := &BaseModelValidator{Decoder: newDecoder(t)}

	tests := []struct {
		name        string
		uri         *string
		wantAllowed bool
	}{
		{name: "no storage allowed", uri: nil, wantAllowed: true},
		{name: "non-pvc allowed", uri: ptr.To("oci://n/ns/b/bucket/o/path"), wantAllowed: true},
		{name: "valid namespaced pvc allowed", uri: ptr.To("pvc://my-pvc/models/llama"), wantAllowed: true},
		{name: "namespaced pvc with namespace prefix denied", uri: ptr.To("pvc://shared:my-pvc/models/llama"), wantAllowed: false},
		{name: "malformed pvc denied", uri: ptr.To("pvc://"), wantAllowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bm := &v1beta1.BaseModel{ObjectMeta: metav1.ObjectMeta{Name: "m", Namespace: "models"}}
			if tc.uri != nil {
				bm.Spec.Storage = &v1beta1.StorageSpec{StorageUri: tc.uri}
			}
			raw, err := json.Marshal(bm)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			resp := v.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: raw}},
			})
			if resp.Allowed != tc.wantAllowed {
				t.Fatalf("allowed=%v, want %v (result: %+v)", resp.Allowed, tc.wantAllowed, resp.Result)
			}
		})
	}
}

func TestClusterBaseModelValidator_Handle(t *testing.T) {
	v := &ClusterBaseModelValidator{Decoder: newDecoder(t)}

	tests := []struct {
		name        string
		uri         *string
		wantAllowed bool
	}{
		{name: "valid cluster-scoped pvc allowed", uri: ptr.To("pvc://shared:my-pvc/models/llama"), wantAllowed: true},
		{name: "cluster-scoped pvc without namespace prefix denied", uri: ptr.To("pvc://my-pvc/models/llama"), wantAllowed: false},
		{name: "non-pvc allowed", uri: ptr.To("oci://n/ns/b/bucket/o/path"), wantAllowed: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cbm := &v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "m"}}
			if tc.uri != nil {
				cbm.Spec.Storage = &v1beta1.StorageSpec{StorageUri: tc.uri}
			}
			raw, err := json.Marshal(cbm)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			resp := v.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: raw}},
			})
			if resp.Allowed != tc.wantAllowed {
				t.Fatalf("allowed=%v, want %v (result: %+v)", resp.Allowed, tc.wantAllowed, resp.Result)
			}
		})
	}
}
