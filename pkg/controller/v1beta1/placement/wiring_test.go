package placement

// validateWiring is the composition-root guard: production setup must
// fail fast when the authoritative reader is missing instead of
// silently degrading every live read to the lagging cache.

import (
	"testing"

	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateWiring(t *testing.T) {
	r := &Reconciler{}
	if err := r.validateWiring(); err == nil {
		t.Fatal("nil APIReader must be rejected at setup")
	}
	r.APIReader = fakeclient.NewClientBuilder().Build()
	if err := r.validateWiring(); err != nil {
		t.Fatalf("wired APIReader must pass: %v", err)
	}
}
