package testfixture

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// FakeClient is a fake apiserver holding the given objects, with the
// schemes an engine pass touches (core, apps, discovery and ome.io) and the
// status subresource of the service and the replica.
func FakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return FakeClientWithInterceptors(t, interceptor.Funcs{}, objs...)
}

// FakeClientWithInterceptors is FakeClient with the given hooks in front
// of every call, for a test that records the writes a pass makes or ends
// the pass between two of them.
func FakeClientWithInterceptors(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1beta1.AddToScheme, discoveryv1.AddToScheme, appsv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatalf("add to scheme: %v", err)
		}
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.InferenceService{}, &v1beta1.InferenceReplica{}).
		WithObjects(objs...).
		WithInterceptorFuncs(funcs).
		Build()
}
