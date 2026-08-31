package servingruntime

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func newReconciler(t *testing.T, objs ...client.Object) (*InheritanceReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.ClusterServingRuntime{}, &v1beta1.ServingRuntime{}).
		WithObjects(objs...).
		Build()
	return &InheritanceReconciler{
		Client:   c,
		Log:      testr.New(t),
		Scheme:   scheme,
		Recorder: &record.FakeRecorder{},
	}, c
}

func runReconcile(t *testing.T, r *InheritanceReconciler, namespace, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	if err != nil {
		t.Fatalf("Reconcile(%s/%s): %v", namespace, name, err)
	}
}

func envSpec(envs ...corev1.EnvVar) v1beta1.ServingRuntimeSpec {
	return v1beta1.ServingRuntimeSpec{
		ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
			Containers: []corev1.Container{{Name: "ome-container", Env: envs}},
		},
	}
}

func mkCSR(name, inheritFrom string, spec v1beta1.ServingRuntimeSpec) *v1beta1.ClusterServingRuntime {
	annotations := map[string]string{}
	if inheritFrom != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = inheritFrom
	}
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations, Generation: 1},
		Spec:       spec,
	}
}

func mkSR(name, namespace, inheritFrom string, spec v1beta1.ServingRuntimeSpec) *v1beta1.ServingRuntime {
	annotations := map[string]string{}
	if inheritFrom != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = inheritFrom
	}
	return &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations, Generation: 1},
		Spec:       spec,
	}
}

func mustGetCSR(t *testing.T, c client.Client, name string) *v1beta1.ClusterServingRuntime {
	t.Helper()
	out := &v1beta1.ClusterServingRuntime{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, out); err != nil {
		t.Fatalf("Get(%s): %v", name, err)
	}
	return out
}

func mustGetSR(t *testing.T, c client.Client, ns, name string) *v1beta1.ServingRuntime {
	t.Helper()
	out := &v1beta1.ServingRuntime{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, out); err != nil {
		t.Fatalf("Get(%s/%s): %v", ns, name, err)
	}
	return out
}

// Cluster scope

func TestReconcile_Cluster_NoInheritance(t *testing.T) {
	g := gomega.NewWithT(t)
	solo := mkCSR("solo", "", v1beta1.ServingRuntimeSpec{Disabled: ptr.To(true)})
	r, c := newReconciler(t, solo)

	runReconcile(t, r, "", "solo")

	got := mustGetCSR(t, c, "solo")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"solo"}))
	g.Expect(got.Status.Conditions).To(gomega.HaveLen(1))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

func TestReconcile_Cluster_TwoLevelChain(t *testing.T) {
	g := gomega.NewWithT(t)
	profile := mkCSR("profile-infra", "",
		envSpec(corev1.EnvVar{Name: "NCCL_DEBUG", Value: "INFO"}))
	rt := mkCSR("rt-sglang", "profile-infra",
		envSpec(corev1.EnvVar{Name: "FROM_RT", Value: "yes"}))
	r, c := newReconciler(t, profile, rt)

	runReconcile(t, r, "", "rt-sglang")

	got := mustGetCSR(t, c, "rt-sglang")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"profile-infra", "rt-sglang"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

func TestReconcile_Cluster_ParentMissing_PreservesPriorChain(t *testing.T) {
	g := gomega.NewWithT(t)
	rt := mkCSR("rt", "ghost-parent", v1beta1.ServingRuntimeSpec{})
	rt.Status = v1beta1.ServingRuntimeStatus{
		InheritanceChain: []string{"some-old-parent", "rt"},
	}
	r, c := newReconciler(t, rt)

	runReconcile(t, r, "", "rt")

	got := mustGetCSR(t, c, "rt")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"some-old-parent", "rt"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(got.Status.Conditions[0].Reason).To(gomega.Equal(ReasonParentNotFound))
}

func TestReconcile_Cluster_Cycle(t *testing.T) {
	g := gomega.NewWithT(t)
	a := mkCSR("a", "b", v1beta1.ServingRuntimeSpec{})
	b := mkCSR("b", "a", v1beta1.ServingRuntimeSpec{})
	r, c := newReconciler(t, a, b)

	runReconcile(t, r, "", "a")

	got := mustGetCSR(t, c, "a")
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(got.Status.Conditions[0].Reason).To(gomega.Equal(ReasonCycle))
}

func TestReconcile_Cluster_Idempotent(t *testing.T) {
	g := gomega.NewWithT(t)
	solo := mkCSR("solo", "", v1beta1.ServingRuntimeSpec{Disabled: ptr.To(true)})
	r, c := newReconciler(t, solo)

	runReconcile(t, r, "", "solo")
	firstRV := mustGetCSR(t, c, "solo").ResourceVersion
	runReconcile(t, r, "", "solo")
	g.Expect(mustGetCSR(t, c, "solo").ResourceVersion).To(gomega.Equal(firstRV),
		"second reconcile must be a no-op (no status diff → no API write)")
}

// Namespaced scope

func TestReconcile_Namespaced_NoInheritance(t *testing.T) {
	g := gomega.NewWithT(t)
	sr := mkSR("solo", "team-a", "", v1beta1.ServingRuntimeSpec{Disabled: ptr.To(false)})
	r, c := newReconciler(t, sr)

	runReconcile(t, r, "team-a", "solo")

	got := mustGetSR(t, c, "team-a", "solo")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"solo"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

func TestReconcile_Namespaced_SameNamespaceParent(t *testing.T) {
	g := gomega.NewWithT(t)
	parent := mkSR("ns-profile", "team-a", "",
		envSpec(corev1.EnvVar{Name: "FROM_NS_PROFILE", Value: "yes"}))
	child := mkSR("rt", "team-a", "ns-profile",
		envSpec(corev1.EnvVar{Name: "FROM_CHILD", Value: "yes"}))
	r, c := newReconciler(t, parent, child)

	runReconcile(t, r, "team-a", "rt")

	got := mustGetSR(t, c, "team-a", "rt")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"ns-profile", "rt"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

func TestReconcile_Namespaced_ClusterFallback(t *testing.T) {
	g := gomega.NewWithT(t)
	clusterProfile := mkCSR("cluster-infra", "",
		envSpec(corev1.EnvVar{Name: "FROM_CLUSTER", Value: "yes"}))
	child := mkSR("rt", "team-a", "cluster-infra", v1beta1.ServingRuntimeSpec{})
	r, c := newReconciler(t, clusterProfile, child)

	runReconcile(t, r, "team-a", "rt")

	got := mustGetSR(t, c, "team-a", "rt")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"cluster-infra", "rt"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

func TestReconcile_Namespaced_CrossNamespaceRejected(t *testing.T) {
	g := gomega.NewWithT(t)
	otherNS := mkSR("shared", "other-team", "", v1beta1.ServingRuntimeSpec{})
	child := mkSR("rt", "team-a", "shared", v1beta1.ServingRuntimeSpec{})
	r, c := newReconciler(t, otherNS, child)

	runReconcile(t, r, "team-a", "rt")

	got := mustGetSR(t, c, "team-a", "rt")
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(got.Status.Conditions[0].Reason).To(gomega.Equal(ReasonParentNotFound))
}

func TestReconcile_Namespaced_NamespacedShadowsCluster(t *testing.T) {
	g := gomega.NewWithT(t)
	// Same name in both scopes; namespaced wins — the reconciler's
	// chain reflects the local parent, not the cluster one.
	clusterParent := mkCSR("infra", "", envSpec(corev1.EnvVar{Name: "FROM_CLUSTER", Value: "yes"}))
	nsParent := mkSR("infra", "team-a", "", envSpec(corev1.EnvVar{Name: "FROM_NS", Value: "yes"}))
	child := mkSR("rt", "team-a", "infra", v1beta1.ServingRuntimeSpec{})
	r, c := newReconciler(t, clusterParent, nsParent, child)

	runReconcile(t, r, "team-a", "rt")

	got := mustGetSR(t, c, "team-a", "rt")
	g.Expect(got.Status.InheritanceChain).To(gomega.Equal([]string{"infra", "rt"}))
	g.Expect(got.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
}

// Cascade fan-out (handler-level)

func TestOnClusterEvent_FanoutToBothScopes(t *testing.T) {
	g := gomega.NewWithT(t)
	root := mkCSR("root", "", v1beta1.ServingRuntimeSpec{})
	clusterChild := mkCSR("c1", "root", v1beta1.ServingRuntimeSpec{})
	nsChild1 := mkSR("n1", "team-a", "root", v1beta1.ServingRuntimeSpec{})
	nsChild2 := mkSR("n2", "team-b", "root", v1beta1.ServingRuntimeSpec{})
	unrelated := mkCSR("other", "", v1beta1.ServingRuntimeSpec{})
	r, _ := newReconciler(t, root, clusterChild, nsChild1, nsChild2, unrelated)

	reqs := r.dependentsOfCluster(context.Background(), root)
	keys := []string{}
	for _, req := range reqs {
		keys = append(keys, req.Namespace+"/"+req.Name)
	}
	g.Expect(keys).To(gomega.ConsistOf("/root", "/c1", "team-a/n1", "team-b/n2"),
		"CSR event should enqueue self + cluster dependents + ns dependents across all namespaces")
}

func TestOnNamespacedEvent_FanoutWithinNamespace(t *testing.T) {
	g := gomega.NewWithT(t)
	parent := mkSR("parent", "team-a", "", v1beta1.ServingRuntimeSpec{})
	sibling := mkSR("sibling", "team-a", "parent", v1beta1.ServingRuntimeSpec{})
	otherNS := mkSR("other-ns-child", "team-b", "parent", v1beta1.ServingRuntimeSpec{})
	unrelated := mkSR("unrelated", "team-a", "", v1beta1.ServingRuntimeSpec{})
	r, _ := newReconciler(t, parent, sibling, otherNS, unrelated)

	reqs := r.dependentsOfNamespaced(context.Background(), parent)
	keys := []string{}
	for _, req := range reqs {
		keys = append(keys, req.Namespace+"/"+req.Name)
	}
	g.Expect(keys).To(gomega.ConsistOf("team-a/parent", "team-a/sibling"),
		"SR event should enqueue self + same-ns dependents only (cross-ns inheritance is disallowed)")
}
