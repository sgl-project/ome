package runtimerevision

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/constants"
)

func newDecoder(t *testing.T) admission.Decoder {
	t.Helper()
	s := runtime.NewScheme()
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return admission.NewDecoder(s)
}

func encode(t *testing.T, obj *appsv1.ControllerRevision) []byte {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// omeRevision returns a baseline OME-owned ControllerRevision suitable
// for cloning + mutating in update-path tests.
func omeRevision() *appsv1.ControllerRevision {
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cr-srt-llama-pd-abc12345",
			Namespace: "ome",
			Annotations: map[string]string{
				constants.RuntimeRevisionCreatedByKey: constants.RuntimeRevisionCreatedByOMEValue,
			},
			Labels: map[string]string{
				constants.RuntimeRevisionOfLabelKey:   "srt-llama-pd",
				constants.RuntimeRevisionHashLabelKey: "abc12345",
			},
		},
		Data:     runtime.RawExtension{Raw: []byte(`{"k":"v"}`)},
		Revision: 7,
	}
}

func updateReq(t *testing.T, oldObj, newObj *appsv1.ControllerRevision) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		OldObject: runtime.RawExtension{Raw: encode(t, oldObj)},
		Object:    runtime.RawExtension{Raw: encode(t, newObj)},
	}}
}

func TestHandle_NonUpdate_PassesThrough(t *testing.T) {
	g := gomega.NewWithT(t)
	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
	}}
	g.Expect(v.Handle(context.Background(), req).Allowed).To(gomega.BeTrue())
}

func TestHandle_UpdateOnNonOMERevision_PassesThrough(t *testing.T) {
	g := gomega.NewWithT(t)
	old := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "sts-foo-1", Namespace: "team-a"},
	}
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		OldObject: runtime.RawExtension{Raw: encode(t, old)},
	}}
	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	g.Expect(v.Handle(context.Background(), req).Allowed).To(gomega.BeTrue(),
		"revisions not annotated as OME-created should be ignored")
}

func TestHandle_DataChange_Denied(t *testing.T) {
	g := gomega.NewWithT(t)
	old := omeRevision()
	new := omeRevision()
	new.Data = runtime.RawExtension{Raw: []byte(`{"k":"tampered"}`)}

	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	resp := v.Handle(context.Background(), updateReq(t, old, new))
	g.Expect(resp.Allowed).To(gomega.BeFalse())
	g.Expect(resp.Result.Message).To(gomega.ContainSubstring(".data is immutable"))
}

func TestHandle_LabelChange_Denied(t *testing.T) {
	g := gomega.NewWithT(t)
	old := omeRevision()
	new := omeRevision()
	new.Labels[constants.RuntimeRevisionOfLabelKey] = "srt-other-runtime"

	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	resp := v.Handle(context.Background(), updateReq(t, old, new))
	g.Expect(resp.Allowed).To(gomega.BeFalse())
	g.Expect(resp.Result.Message).To(gomega.ContainSubstring(".labels are immutable"))
}

func TestHandle_RevisionChange_Denied(t *testing.T) {
	g := gomega.NewWithT(t)
	old := omeRevision()
	new := omeRevision()
	new.Revision = 8

	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	resp := v.Handle(context.Background(), updateReq(t, old, new))
	g.Expect(resp.Allowed).To(gomega.BeFalse())
	g.Expect(resp.Result.Message).To(gomega.ContainSubstring(".revision is immutable"))
}

func TestHandle_AnnotationOnlyChange_Allowed(t *testing.T) {
	g := gomega.NewWithT(t)
	old := omeRevision()
	new := omeRevision()
	// This is the path the GC controller relies on: Mark sets
	// ome.io/gc-eligible-since via an annotation update.
	new.Annotations[constants.RuntimeRevisionGCEligibleSinceKey] = "2026-01-01T00:00:00Z"

	v := &ImmutabilityValidator{Decoder: newDecoder(t)}
	resp := v.Handle(context.Background(), updateReq(t, old, new))
	g.Expect(resp.Allowed).To(gomega.BeTrue(),
		"annotation-only updates must be permitted so GC can mark/clear")
}
