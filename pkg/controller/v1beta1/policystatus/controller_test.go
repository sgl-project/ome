package policystatus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

// testKind drives the controller over RolloutPolicy objects with rules of
// its own: a service references the policies its annotation lists, the
// digest is one fixed string, Ready is True, and the test chooses whether
// Ready reports a configuration read or fails with a transient error.
type testKind struct {
	consulted bool
	readyErr  error
}

const (
	testIndexField      = "test.policyRefs"
	testRefsAnnotation  = "policies"
	testDigest          = "digest-1"
	testReadyReason     = "Valid"
	testAttachedReason  = "Attached"
	testNoConsumersCode = "NoConsumers"
)

func (testKind) NewPolicy() *v1beta1.RolloutPolicy { return &v1beta1.RolloutPolicy{} }

func (testKind) ReferenceIndexField() string { return testIndexField }

func (testKind) References(isvc *v1beta1.InferenceService) []string {
	list := isvc.Annotations[testRefsAnnotation]
	if list == "" {
		return nil
	}
	return strings.Split(list, ",")
}

func (testKind) Digest(*v1beta1.RolloutPolicy) (string, error) { return testDigest, nil }

func (k testKind) Ready(policy *v1beta1.RolloutPolicy, _ ConfigSource, _ error) (metav1.Condition, bool, error) {
	if k.readyErr != nil {
		return metav1.Condition{}, k.consulted, k.readyErr
	}
	return metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		Reason:             testReadyReason,
	}, k.consulted, nil
}

func (testKind) InUse(generation int64, attached int32) metav1.Condition {
	condition := metav1.Condition{
		Type:               "InUse",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             testNoConsumersCode,
	}
	if attached > 0 {
		condition.Status = metav1.ConditionTrue
		condition.Reason = testAttachedReason
	}
	return condition
}

func (testKind) Status(policy *v1beta1.RolloutPolicy) StatusFields {
	return StatusFields{
		ObservedGeneration: &policy.Status.ObservedGeneration,
		PortableDigest:     &policy.Status.PortableDigest,
		Attached:           &policy.Status.AttachedGroups,
		Conditions:         &policy.Status.Conditions,
	}
}

func newReconciler(t *testing.T, kind testKind, objs ...client.Object) *Reconciler[*v1beta1.RolloutPolicy] {
	t.Helper()
	c := fakeclient.New(t,
		fakeclient.WithStatusSubresource(&v1beta1.RolloutPolicy{}),
		fakeclient.WithIndex(&v1beta1.InferenceService{}, testIndexField, func(obj client.Object) []string {
			return ReferencedPolicies[*v1beta1.RolloutPolicy](kind, obj)
		}),
		fakeclient.WithObjects(objs...),
	)
	return &Reconciler[*v1beta1.RolloutPolicy]{
		Client: c,
		Kind:   kind,
		Config: ConfigSource{Cache: controllerconfig.NewConfigCache(0)},
	}
}

func mkPolicy(namespace, name string, generation int64) *v1beta1.RolloutPolicy {
	return &v1beta1.RolloutPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Generation: generation},
	}
}

// mkConsumer builds an InferenceService that references the named policies,
// one entry per referencing slot.
func mkConsumer(namespace, name string, refs ...string) *v1beta1.InferenceService {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
	if len(refs) > 0 {
		isvc.Annotations = map[string]string{testRefsAnnotation: strings.Join(refs, ",")}
	}
	return isvc
}

func runReconcile(t *testing.T, r *Reconciler[*v1beta1.RolloutPolicy], namespace, name string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	if err != nil {
		t.Fatalf("Reconcile(%s/%s): %v", namespace, name, err)
	}
	return result
}

func getPolicy(t *testing.T, r *Reconciler[*v1beta1.RolloutPolicy], namespace, name string) *v1beta1.RolloutPolicy {
	t.Helper()
	policy := &v1beta1.RolloutPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, policy); err != nil {
		t.Fatalf("Get(%s/%s): %v", namespace, name, err)
	}
	return policy
}

func TestReconcilePublishesWhatThePassObserved(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{},
		mkPolicy("ns1", "policy", 3),
		// Two references from one service plus one from another: the count
		// is per reference, not per InferenceService.
		mkConsumer("ns1", "consumer-a", "policy", "policy"),
		mkConsumer("ns1", "consumer-b", "policy"),
		// The same name in another namespace and another policy in this
		// namespace do not count.
		mkConsumer("ns2", "other-namespace", "policy"),
		mkConsumer("ns1", "other-policy", "something-else"),
	)

	runReconcile(t, r, "ns1", "policy")
	got := getPolicy(t, r, "ns1", "policy")

	g.Expect(got.Status.ObservedGeneration).To(gomega.Equal(int64(3)))
	g.Expect(got.Status.PortableDigest).To(gomega.Equal(testDigest))
	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(3)))

	ready := apimeta.FindStatusCondition(got.Status.Conditions, "Ready")
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(testReadyReason))
	g.Expect(ready.ObservedGeneration).To(gomega.Equal(int64(3)))

	inUse := apimeta.FindStatusCondition(got.Status.Conditions, "InUse")
	g.Expect(inUse).NotTo(gomega.BeNil())
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(inUse.Reason).To(gomega.Equal(testAttachedReason))
	g.Expect(inUse.ObservedGeneration).To(gomega.Equal(int64(3)))
}

func TestReconcileFollowsReferencesAsTheyComeAndGo(t *testing.T) {
	g := gomega.NewWithT(t)
	consumer := mkConsumer("ns1", "consumer-a", "policy")
	r := newReconciler(t, testKind{}, mkPolicy("ns1", "policy", 1), consumer)

	runReconcile(t, r, "ns1", "policy")
	g.Expect(getPolicy(t, r, "ns1", "policy").Status.AttachedGroups).To(gomega.Equal(int32(1)))

	g.Expect(r.Delete(context.Background(), consumer)).To(gomega.Succeed())
	runReconcile(t, r, "ns1", "policy")
	got := getPolicy(t, r, "ns1", "policy")

	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(0)))
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, "InUse")
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(inUse.Reason).To(gomega.Equal(testNoConsumersCode))
}

func TestReconcileSkipsAnUnchangedStatus(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{}, mkPolicy("ns1", "policy", 1), mkConsumer("ns1", "consumer-a", "policy"))

	runReconcile(t, r, "ns1", "policy")
	first := getPolicy(t, r, "ns1", "policy")
	runReconcile(t, r, "ns1", "policy")
	second := getPolicy(t, r, "ns1", "policy")

	g.Expect(second.ResourceVersion).To(gomega.Equal(first.ResourceVersion))
}

// A rewrite for one field (here the reference count) must not move the
// transition time of a condition whose status did not change.
func TestReconcileKeepsTheTransitionTimeOfAnUnchangedCondition(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{}, mkPolicy("ns1", "policy", 1))
	runReconcile(t, r, "ns1", "policy")

	// Backdate the transition times so a reset is distinguishable from a
	// same-second rewrite.
	got := getPolicy(t, r, "ns1", "policy")
	past := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
	for i := range got.Status.Conditions {
		got.Status.Conditions[i].LastTransitionTime = past
	}
	g.Expect(r.Status().Update(context.Background(), got)).To(gomega.Succeed())

	g.Expect(r.Create(context.Background(), mkConsumer("ns1", "consumer-a", "policy"))).To(gomega.Succeed())
	runReconcile(t, r, "ns1", "policy")
	got = getPolicy(t, r, "ns1", "policy")

	g.Expect(got.Status.AttachedGroups).To(gomega.Equal(int32(1)), "the count moved, so this pass rewrote the status")
	ready := apimeta.FindStatusCondition(got.Status.Conditions, "Ready")
	g.Expect(ready.LastTransitionTime.Time.Equal(past.Time)).To(gomega.BeTrue())
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, "InUse")
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(inUse.LastTransitionTime.Time.After(past.Time)).To(gomega.BeTrue())
}

// The operator configuration sends no watch event, so a pass that read it
// checks again once the cached copy expires, on the unchanged-status path
// as well.
func TestReconcileChecksAgainAfterReadingTheConfiguration(t *testing.T) {
	g := gomega.NewWithT(t)
	const ttl = 30 * time.Second
	r := newReconciler(t, testKind{consulted: true}, mkPolicy("ns1", "policy", 1))
	r.Config.Cache = controllerconfig.NewConfigCache(ttl)

	g.Expect(runReconcile(t, r, "ns1", "policy").RequeueAfter).To(gomega.Equal(ttl))
	g.Expect(runReconcile(t, r, "ns1", "policy").RequeueAfter).To(gomega.Equal(ttl))
}

// No periodic check when the pass read no configuration, and none when the
// cache keeps nothing (every read is live).
func TestReconcileSchedulesNoCheckWithoutACachedConfigurationRead(t *testing.T) {
	g := gomega.NewWithT(t)

	r := newReconciler(t, testKind{consulted: false}, mkPolicy("ns1", "policy", 1))
	r.Config.Cache = controllerconfig.NewConfigCache(30 * time.Second)
	g.Expect(runReconcile(t, r, "ns1", "policy")).To(gomega.Equal(ctrl.Result{}))

	r = newReconciler(t, testKind{consulted: true}, mkPolicy("ns1", "policy", 1))
	g.Expect(runReconcile(t, r, "ns1", "policy")).To(gomega.Equal(ctrl.Result{}))
}

func TestReconcileKeepsTheStatusWhenReadinessFailsTransiently(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{readyErr: errors.New("configuration unreachable")}, mkPolicy("ns1", "policy", 1))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy"}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("configuration unreachable")))

	got := getPolicy(t, r, "ns1", "policy")
	g.Expect(got.Status.Conditions).To(gomega.BeEmpty())
	g.Expect(got.Status.PortableDigest).To(gomega.BeEmpty())
}

func TestReconcileIgnoresAMissingPolicy(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{})
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "gone"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
}

func TestReferencedPoliciesListsEachPolicyOnce(t *testing.T) {
	g := gomega.NewWithT(t)
	g.Expect(ReferencedPolicies[*v1beta1.RolloutPolicy](testKind{}, mkConsumer("ns1", "a", "policy-a", "policy-a", "policy-b"))).
		To(gomega.Equal([]string{"policy-a", "policy-b"}))
	g.Expect(ReferencedPolicies[*v1beta1.RolloutPolicy](testKind{}, mkConsumer("ns1", "b"))).To(gomega.BeNil())
	g.Expect(ReferencedPolicies[*v1beta1.RolloutPolicy](testKind{}, &corev1.Pod{})).To(gomega.BeNil())
}

func TestServiceEventsEnqueueEachReferencedPolicyOnce(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testKind{})

	requests := r.policiesReferencedBy(context.Background(), mkConsumer("ns1", "consumer-a", "policy-a", "policy-a", "policy-b"))
	g.Expect(requests).To(gomega.ConsistOf(
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-a"}},
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-b"}},
	))
	g.Expect(r.policiesReferencedBy(context.Background(), mkConsumer("ns1", "no-refs"))).To(gomega.BeEmpty())
	g.Expect(r.policiesReferencedBy(context.Background(), &corev1.Pod{})).To(gomega.BeNil())
}
