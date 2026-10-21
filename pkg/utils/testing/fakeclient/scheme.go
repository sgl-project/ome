package fakeclient

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Scheme returns a fresh scheme with the Kubernetes built-in types and the
// ome.io API group registered, plus every extra group handed in.
func Scheme(t testing.TB, extra ...func(*runtime.Scheme) error) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	addTypes(t, s, clientgoscheme.AddToScheme, v1beta1.AddToScheme)
	addTypes(t, s, extra...)
	return s
}

func addTypes(t testing.TB, s *runtime.Scheme, add ...func(*runtime.Scheme) error) {
	t.Helper()
	for _, f := range add {
		if err := f(s); err != nil {
			t.Fatalf("fakeclient: register types: %v", err)
		}
	}
}
