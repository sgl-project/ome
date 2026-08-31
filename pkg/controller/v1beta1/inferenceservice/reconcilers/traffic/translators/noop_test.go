package translators

import (
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/traffic"
)

func TestNoop_Name(t *testing.T) {
	if got := NewNoop().Name(); got != NoopName {
		t.Fatalf("Name() = %q, want %q", got, NoopName)
	}
}

func TestNoop_SupportedAnnotations_IsEmpty(t *testing.T) {
	// The Noop translator advertises no annotation support so the
	// reconciler reports every ome.io/* annotation as UnsupportedField.
	got := NewNoop().SupportedAnnotations()
	if got == nil {
		t.Fatalf("SupportedAnnotations() must return non-nil set")
	}
	if got.Len() != 0 {
		t.Fatalf("SupportedAnnotations() must be empty, got: %v", got.UnsortedList())
	}
}

func TestNoop_Translate_ReturnsNil(t *testing.T) {
	// (nil, nil, nil) is the documented "no resource needed" signal.
	// The reconciler treats this as "skip apply" and surfaces
	// NoTranslatorAvailable separately based on the translator name.
	obj, passthroughs, err := NewNoop().Translate(
		&v1beta1.InferenceService{},
		[]string{"isvc", "isvc-engine"},
		&traffic.ResolvedIntent{},
	)
	if err != nil {
		t.Fatalf("Translate() err = %v, want nil", err)
	}
	if obj != nil {
		t.Fatalf("Translate() obj = %v, want nil", obj)
	}
	if passthroughs != nil {
		t.Fatalf("Translate() passthroughs = %v, want nil", passthroughs)
	}
}

func TestNoop_Watches_ReturnsNil(t *testing.T) {
	if got := NewNoop().Watches(); got != nil {
		t.Fatalf("Watches() = %v, want nil", got)
	}
}

func TestNoop_SupportedTrafficFields_IsEmpty(t *testing.T) {
	// The Noop translator emits no typed spec.traffic field; declaring
	// any capability would make the reconciler and webhook treat
	// intent as honored when nothing is emitted at all.
	got := NewNoop().SupportedTrafficFields()
	if got == nil {
		t.Fatalf("SupportedTrafficFields() must return a non-nil set")
	}
	if got.Len() != 0 {
		t.Fatalf("SupportedTrafficFields() must be empty, got: %v", got.UnsortedList())
	}
}
