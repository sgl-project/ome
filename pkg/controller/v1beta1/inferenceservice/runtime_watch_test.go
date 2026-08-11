package inferenceservice

import (
	"context"
	"sort"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func rtRefISVC(ns, name, runtimeName string, autoSync *bool) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: runtimeName, AutoSync: autoSync},
		},
	}
}

func reqNames(reqs []reconcile.Request) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.Namespace+"/"+r.Name)
	}
	sort.Strings(out)
	return out
}

// TestIsvcsReferencingRuntime_EnqueuesAutoSyncTrue is the regression guard
// for the fix: a runtime change must fan out to EVERY referencing ISVC,
// including autoSync=true (float) ones — previously they were skipped, so
// runtime edits silently never reached them.
func TestIsvcsReferencingRuntime_EnqueuesAutoSyncTrue(t *testing.T) {
	s := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(s))
	c := ctrlclientfake.NewClientBuilder().WithScheme(s).
		WithIndex(&v1beta1.InferenceService{}, isvcRuntimeNameIndexField, isvcRuntimeNameIndexExtractor).
		WithObjects(
			rtRefISVC("prod", "float-default", "rt-a", nil),          // default float -> enqueue
			rtRefISVC("prod", "float-true", "rt-a", ptr.To(true)),    // explicit float -> enqueue (the fix)
			rtRefISVC("prod", "pinned", "rt-a", ptr.To(false)),       // pinned -> enqueue (unchanged)
			rtRefISVC("prod", "other-runtime", "rt-b", ptr.To(true)), // different runtime -> skip
		).Build()
	r := &InferenceServiceReconciler{Client: c, Log: logr.Discard()}

	// Cluster-scoped runtime event (empty namespace) matches any namespace.
	got := r.isvcsReferencingRuntime(context.Background(), &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-a"},
	})
	assert.Equal(t, []string{"prod/float-default", "prod/float-true", "prod/pinned"}, reqNames(got),
		"every ISVC referencing rt-a must enqueue regardless of autoSync; the rt-b consumer must not")
}

// TestIsvcsReferencingRuntime_NamespacedScope pins that a namespaced
// ServingRuntime only fans out to same-namespace ISVCs.
func TestIsvcsReferencingRuntime_NamespacedScope(t *testing.T) {
	s := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(s))
	c := ctrlclientfake.NewClientBuilder().WithScheme(s).
		WithIndex(&v1beta1.InferenceService{}, isvcRuntimeNameIndexField, isvcRuntimeNameIndexExtractor).
		WithObjects(
			rtRefISVC("prod", "same-ns", "rt-ns", ptr.To(true)),
			rtRefISVC("dev", "other-ns", "rt-ns", ptr.To(true)),
		).Build()
	r := &InferenceServiceReconciler{Client: c, Log: logr.Discard()}

	got := r.isvcsReferencingRuntime(context.Background(), &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "rt-ns"},
	})
	assert.Equal(t, []string{"prod/same-ns"}, reqNames(got),
		"a namespaced ServingRuntime must only enqueue ISVCs in its own namespace")
}
