package rolloutpolicy

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	policycore "sigs.k8s.io/ome/pkg/rolloutpolicy"
)

// fakeBindings is a test ProviderBindings: a fixed name set, interval, and
// optional transient error.
type fakeBindings struct {
	names    map[string]struct{}
	interval time.Duration
	err      error
}

func (f *fakeBindings) Providers() (map[string]struct{}, error) { return f.names, f.err }
func (f *fakeBindings) RecheckInterval() time.Duration          { return f.interval }

func boundBindings(names ...string) *fakeBindings {
	set := map[string]struct{}{}
	for _, name := range names {
		set[name] = struct{}{}
	}
	return &fakeBindings{names: set}
}

func newReconciler(t *testing.T, bindings ProviderBindings, objs ...client.Object) *StatusReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.RolloutPolicy{}).
		WithIndex(&v1beta1.InferenceService{}, PolicyRefIndexField, PolicyRefIndexer).
		WithObjects(objs...).
		Build()
	return &StatusReconciler{
		Client:    c,
		Providers: bindings,
		Log:       testr.New(t),
		Scheme:    scheme,
	}
}

func runReconcile(t *testing.T, r *StatusReconciler, namespace, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}); err != nil {
		t.Fatalf("Reconcile(%s/%s): %v", namespace, name, err)
	}
}

func getPolicy(t *testing.T, r *StatusReconciler, namespace, name string) *v1beta1.RolloutPolicy {
	t.Helper()
	policy := &v1beta1.RolloutPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, policy); err != nil {
		t.Fatalf("Get(%s/%s): %v", namespace, name, err)
	}
	return policy
}

// canarySpec is a minimal valid canary body; a non-empty provider adds the
// providerRef metrics source.
func canarySpec(provider string) v1beta1.RolloutPolicySpec {
	c := &v1beta1.GroupCanary{
		Steps: []v1beta1.RolloutGroupStep{
			{Capacity: intstr.FromString("50%"), Traffic: 50},
			{Capacity: intstr.FromString("100%"), Traffic: 100},
		},
	}
	if provider != "" {
		c.Prometheus = &v1beta1.AnalysisPrometheus{
			ProviderRef: &v1beta1.MetricProviderRef{Name: provider},
		}
	}
	return v1beta1.RolloutPolicySpec{Canary: c}
}

func mkPolicy(namespace, name string, generation int64, spec v1beta1.RolloutPolicySpec) *v1beta1.RolloutPolicy {
	return &v1beta1.RolloutPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Generation: generation},
		Spec:       spec,
	}
}

// mkISVC builds an InferenceService with one single-Component rollout group
// per ref; an empty ref yields a group with no policyRef.
func mkISVC(namespace, name string, refs ...string) *v1beta1.InferenceService {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
	if len(refs) == 0 {
		return isvc
	}
	rollout := &v1beta1.RolloutSpec{}
	for _, ref := range refs {
		group := v1beta1.RolloutGroup{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}}
		if ref != "" {
			group.PolicyRef = &v1beta1.RolloutPolicyRef{
				Name:        ref,
				Progression: v1beta1.RolloutProgressionCanary,
			}
		}
		rollout.Groups = append(rollout.Groups, group)
	}
	isvc.Spec.Rollout = rollout
	return isvc
}

func TestReconcileValidPolicy(t *testing.T) {
	g := gomega.NewWithT(t)
	policy := mkPolicy("ns1", "canary-std", 3, canarySpec("llm-metrics"))
	r := newReconciler(t, boundBindings("llm-metrics"),
		policy,
		// Two groups on one consumer plus one on another: the count is per
		// rollout group, not per InferenceService.
		mkISVC("ns1", "consumer-a", "canary-std", "canary-std"),
		mkISVC("ns1", "consumer-b", "canary-std"),
		// Same name in another namespace, a different policy in the same
		// namespace, and a ref-less group must not count.
		mkISVC("ns2", "other-ns", "canary-std"),
		mkISVC("ns1", "other-policy", "something-else", ""),
	)

	runReconcile(t, r, "ns1", "canary-std")
	got := getPolicy(t, r, "ns1", "canary-std")

	g.Expect(got.Status.ObservedGeneration).To(gomega.Equal(int64(3)))
	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(3)))

	wantDigest, err := policycore.PortableDigest(&policy.Spec)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(got.Status.PortableDigest).To(gomega.Equal(wantDigest))

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonBodyValid))
	g.Expect(ready.ObservedGeneration).To(gomega.Equal(int64(3)))

	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyInUseCondition)
	g.Expect(inUse).NotTo(gomega.BeNil())
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(inUse.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonAttached))
	g.Expect(inUse.ObservedGeneration).To(gomega.Equal(int64(3)))
}

func TestReadyFlipsOnInvalidBody(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")))

	runReconcile(t, r, "ns1", "canary-std")
	got := getPolicy(t, r, "ns1", "canary-std")
	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	validDigest := got.Status.PortableDigest

	// Final step below 100% traffic fails the plan rules.
	got.Spec.Canary.Steps = []v1beta1.RolloutGroupStep{
		{Capacity: intstr.FromString("50%"), Traffic: 50},
	}
	g.Expect(r.Update(context.Background(), got)).To(gomega.Succeed())

	runReconcile(t, r, "ns1", "canary-std")
	got = getPolicy(t, r, "ns1", "canary-std")

	ready = apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonBodyInvalid))
	g.Expect(ready.Message).To(gomega.ContainSubstring("traffic"))
	// The digest is still reported and tracks the new spec: a digest
	// describes the spec, not its validity.
	g.Expect(got.Status.PortableDigest).NotTo(gomega.BeEmpty())
	g.Expect(got.Status.PortableDigest).NotTo(gomega.Equal(validDigest))
	g.Expect(apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyInUseCondition)).NotTo(gomega.BeNil())
}

func TestReadyFalseOnPolicyOnlyRules(t *testing.T) {
	g := gomega.NewWithT(t)
	// A plan-valid canary whose first step uses an absolute capacity: legal
	// inline, rejected in a policy body (percent-only portability rule).
	spec := canarySpec("")
	spec.Canary.Steps[0].Capacity = intstr.FromInt32(3)
	r := newReconciler(t, boundBindings(), mkPolicy("ns1", "absolute", 1, spec))

	runReconcile(t, r, "ns1", "absolute")
	got := getPolicy(t, r, "ns1", "absolute")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonBodyInvalid))
	g.Expect(ready.Message).To(gomega.ContainSubstring("percentage"))
}

// The portable digest is a function of the spec alone: two policies with
// separately constructed but identical bodies must report the same digest,
// and a different body must not.
func TestDigestStableAcrossIdenticalSpecs(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "copy-a", 1, canarySpec("llm-metrics")),
		mkPolicy("ns1", "copy-b", 5, canarySpec("llm-metrics")),
		mkPolicy("ns1", "different", 1, v1beta1.RolloutPolicySpec{BlueGreen: &v1beta1.GroupBlueGreen{}}))

	runReconcile(t, r, "ns1", "copy-a")
	runReconcile(t, r, "ns1", "copy-b")
	runReconcile(t, r, "ns1", "different")

	a := getPolicy(t, r, "ns1", "copy-a").Status.PortableDigest
	b := getPolicy(t, r, "ns1", "copy-b").Status.PortableDigest
	other := getPolicy(t, r, "ns1", "different").Status.PortableDigest

	g.Expect(a).NotTo(gomega.BeEmpty())
	g.Expect(a).To(gomega.Equal(b))
	g.Expect(other).NotTo(gomega.BeEmpty())
	g.Expect(other).NotTo(gomega.Equal(a))
}

func TestAttachDetachFlipsInUse(t *testing.T) {
	g := gomega.NewWithT(t)
	consumer := mkISVC("ns1", "consumer-a", "canary-std")
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")),
		consumer)

	runReconcile(t, r, "ns1", "canary-std")
	got := getPolicy(t, r, "ns1", "canary-std")
	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(1)))

	g.Expect(r.Delete(context.Background(), consumer)).To(gomega.Succeed())
	runReconcile(t, r, "ns1", "canary-std")
	got = getPolicy(t, r, "ns1", "canary-std")

	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(0)))
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyInUseCondition)
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(inUse.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonNoConsumers))
}

// An unbound provider name is a per-cluster caveat, not a body defect:
// members of one fleet legitimately bind different provider sets. Ready must
// stay True with the ProviderUnbound reason naming the provider.
func TestUnboundProviderIsWarningNotFailure(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings("some-other-provider"),
		mkPolicy("ns1", "canary-std", 2, canarySpec("unbound-provider")))

	runReconcile(t, r, "ns1", "canary-std")
	got := getPolicy(t, r, "ns1", "canary-std")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonProviderUnbound))
	g.Expect(ready.Message).To(gomega.ContainSubstring("unbound-provider"))
	g.Expect(got.Status.PortableDigest).NotTo(gomega.BeEmpty())
}

// A nil bindings seam means the wiring supplies no binding source — the same
// observable state as an empty binding set, degraded but never an error.
func TestNilBindingsReportsProviderUnbound(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, nil,
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")))

	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "canary-std"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))

	got := getPolicy(t, r, "ns1", "canary-std")
	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.RolloutPolicyReasonProviderUnbound))
}

// Provider bindings emit no watch event toward this controller — a reconcile
// that consults them must self-schedule a re-check on the bindings' cadence,
// for both binding outcomes, so the Ready reason converges after a binding
// edit without a policy write.
func TestReconcileRequeuesAfterConsultingBindings(t *testing.T) {
	const interval = 30 * time.Second
	cases := []struct {
		name   string
		policy *v1beta1.RolloutPolicy
	}{
		{"unbound provider", mkPolicy("ns1", "canary-std", 1, canarySpec("unbound-provider"))},
		{"bound provider", mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			bindings := boundBindings("llm-metrics")
			bindings.interval = interval
			r := newReconciler(t, bindings, tc.policy)

			result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "canary-std"}})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result.RequeueAfter).To(gomega.Equal(interval))

			// The unchanged-status early return must keep the periodic pass.
			result, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "canary-std"}})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result.RequeueAfter).To(gomega.Equal(interval))
		})
	}
}

// No periodic requeue when the bindings report no cadence, or when the body
// references no provider and bindings are never consulted.
func TestReconcileSkipsBindingRequeue(t *testing.T) {
	g := gomega.NewWithT(t)

	// Zero interval, provider-consulting policy.
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")))
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "canary-std"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))

	// Positive interval, body with no provider reference.
	bindings := boundBindings("llm-metrics")
	bindings.interval = 30 * time.Second
	r = newReconciler(t, bindings,
		mkPolicy("ns1", "bluegreen", 1, v1beta1.RolloutPolicySpec{BlueGreen: &v1beta1.GroupBlueGreen{}}))
	result, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "bluegreen"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
}

func TestBindingFetchErrorIsTransient(t *testing.T) {
	g := gomega.NewWithT(t)
	// A provider-referencing policy whose binding source cannot be read is a
	// transient fetch failure: the reconcile must error (and retry), not mark
	// the policy off missing data.
	r := newReconciler(t, &fakeBindings{err: context.DeadlineExceeded},
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "canary-std"}})
	g.Expect(err).To(gomega.HaveOccurred())

	got := getPolicy(t, r, "ns1", "canary-std")
	g.Expect(got.Status.Conditions).To(gomega.BeEmpty())
}

func TestReconcileSkipsUnchangedStatus(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")),
		mkISVC("ns1", "consumer-a", "canary-std"))

	runReconcile(t, r, "ns1", "canary-std")
	first := getPolicy(t, r, "ns1", "canary-std")

	runReconcile(t, r, "ns1", "canary-std")
	second := getPolicy(t, r, "ns1", "canary-std")

	g.Expect(second.ResourceVersion).To(gomega.Equal(first.ResourceVersion))
}

// A status rewrite for an unrelated field (here: a spec edit that moves the
// digest) must not reset LastTransitionTime on conditions whose status did
// not change.
func TestLastTransitionTimePreservedOnUnchangedConditions(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings("llm-metrics"),
		mkPolicy("ns1", "canary-std", 1, canarySpec("llm-metrics")))

	runReconcile(t, r, "ns1", "canary-std")

	// Backdate the transition times so a reset is distinguishable from a
	// same-second rewrite.
	got := getPolicy(t, r, "ns1", "canary-std")
	past := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
	for i := range got.Status.Conditions {
		got.Status.Conditions[i].LastTransitionTime = past
	}
	g.Expect(r.Status().Update(context.Background(), got)).To(gomega.Succeed())

	got = getPolicy(t, r, "ns1", "canary-std")
	digestBefore := got.Status.PortableDigest
	delay := int32(60)
	got.Spec.Canary.ScaleDownDelaySeconds = &delay
	g.Expect(r.Update(context.Background(), got)).To(gomega.Succeed())

	runReconcile(t, r, "ns1", "canary-std")
	got = getPolicy(t, r, "ns1", "canary-std")

	// The digest moved, so this reconcile really rewrote status.
	g.Expect(got.Status.PortableDigest).NotTo(gomega.Equal(digestBefore))
	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyReadyCondition)
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.LastTransitionTime.Time.Equal(past.Time)).To(gomega.BeTrue())
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.RolloutPolicyInUseCondition)
	g.Expect(inUse.LastTransitionTime.Time.Equal(past.Time)).To(gomega.BeTrue())
}

func TestReconcileMissingPolicyIsNoop(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings())
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "gone"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
}

func TestPoliciesReferencedBy(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, boundBindings())

	// Distinct names only: two groups share a policy, a third names another.
	reqs := r.policiesReferencedBy(context.Background(), mkISVC("ns1", "consumer-a", "policy-a", "policy-a", "policy-b"))
	g.Expect(reqs).To(gomega.ConsistOf(
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-a"}},
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-b"}},
	))

	g.Expect(r.policiesReferencedBy(context.Background(), mkISVC("ns1", "no-rollout"))).To(gomega.BeEmpty())
	g.Expect(r.policiesReferencedBy(context.Background(), mkISVC("ns1", "no-refs", ""))).To(gomega.BeEmpty())
	g.Expect(r.policiesReferencedBy(context.Background(), &corev1.Pod{})).To(gomega.BeNil())
}

func TestPolicyRefIndexer(t *testing.T) {
	g := gomega.NewWithT(t)
	g.Expect(PolicyRefIndexer(mkISVC("ns1", "a", "policy-a", "policy-a", "policy-b"))).
		To(gomega.ConsistOf("policy-a", "policy-b"))
	g.Expect(PolicyRefIndexer(mkISVC("ns1", "b"))).To(gomega.BeNil())
	g.Expect(PolicyRefIndexer(&corev1.Pod{})).To(gomega.BeNil())
}
