package autoscalerpolicy

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
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/autoscalerpolicy/render"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
)

// testPolicyRefIndexer mirrors the extractor the InferenceService controller
// registers for the spec.autoscalerPolicyRefs field index: the distinct
// policy names across all component slots.
func testPolicyRefIndexer(obj client.Object) []string {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, componentType := range policyRefComponents {
		ref := autoscaler.ComponentPolicyRef(isvc, componentType)
		if ref == nil || ref.Name == "" || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		names = append(names, ref.Name)
	}
	return names
}

const testProviderConfig = `{"metricProviders":{"llm-metrics":{"serverAddress":"http://prometheus.example.com"}}}`

func newReconciler(t *testing.T, configJSON string, objs ...client.Object) *StatusReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.AutoscalerPolicy{}).
		WithIndex(&v1beta1.InferenceService{}, autoscalerPolicyRefIndexField, testPolicyRefIndexer).
		WithObjects(objs...).
		Build()

	clientset := k8sfake.NewSimpleClientset()
	if configJSON != "" {
		clientset = k8sfake.NewSimpleClientset(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.InferenceServiceConfigMapName,
				Namespace: constants.OMENamespace,
			},
			Data: map[string]string{controllerconfig.AutoscalerPolicyConfigName: configJSON},
		})
	}
	return &StatusReconciler{
		Client:      c,
		Clientset:   clientset,
		ConfigCache: controllerconfig.NewConfigCache(0),
		Log:         testr.New(t),
		Scheme:      scheme,
	}
}

func runReconcile(t *testing.T, r *StatusReconciler, namespace, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}); err != nil {
		t.Fatalf("Reconcile(%s/%s): %v", namespace, name, err)
	}
}

func getPolicy(t *testing.T, r *StatusReconciler, namespace, name string) *v1beta1.AutoscalerPolicy {
	t.Helper()
	policy := &v1beta1.AutoscalerPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, policy); err != nil {
		t.Fatalf("Get(%s/%s): %v", namespace, name, err)
	}
	return policy
}

func kedaSpec(provider string, metadata map[string]string) v1beta1.AutoscalerPolicySpec {
	if metadata == nil {
		metadata = map[string]string{
			"query":            "sum(rate(http_requests_total[1m]))",
			"threshold":        "10",
			"ignoreNullValues": "false",
		}
	}
	return v1beta1.AutoscalerPolicySpec{
		Class: v1beta1.AutoscalerKEDA,
		Keda: &v1beta1.KedaPolicyTemplate{
			Triggers: []v1beta1.KedaTriggerTemplate{{
				Type:        "prometheus",
				ProviderRef: &v1beta1.MetricProviderRef{Name: provider},
				Metadata:    metadata,
			}},
		},
	}
}

func mkPolicy(namespace, name string, generation int64, spec v1beta1.AutoscalerPolicySpec) *v1beta1.AutoscalerPolicy {
	return &v1beta1.AutoscalerPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Generation: generation},
		Spec:       spec,
	}
}

func mkISVC(namespace, name string, engineRef, decoderRef, routerRef string) *v1beta1.InferenceService {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
	if engineRef != "" {
		isvc.Spec.Engine = &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				AutoscalerPolicyRef: &v1beta1.AutoscalerPolicyRef{Name: engineRef},
			},
		}
	}
	if decoderRef != "" {
		isvc.Spec.Decoder = &v1beta1.DecoderSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				AutoscalerPolicyRef: &v1beta1.AutoscalerPolicyRef{Name: decoderRef},
			},
		}
	}
	if routerRef != "" {
		isvc.Spec.Router = &v1beta1.RouterSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				AutoscalerPolicyRef: &v1beta1.AutoscalerPolicyRef{Name: routerRef},
			},
		}
	}
	return isvc
}

func TestReconcileValidPolicy(t *testing.T) {
	g := gomega.NewWithT(t)
	policy := mkPolicy("ns1", "req-activity", 3, kedaSpec("llm-metrics", nil))
	r := newReconciler(t, testProviderConfig,
		policy,
		// Two components on one consumer plus one on another: the count is
		// per component, not per InferenceService.
		mkISVC("ns1", "consumer-a", "req-activity", "req-activity", ""),
		mkISVC("ns1", "consumer-b", "req-activity", "", ""),
		// Same name in another namespace and a different policy in the same
		// namespace must not count.
		mkISVC("ns2", "other-ns", "req-activity", "", ""),
		mkISVC("ns1", "other-policy", "something-else", "", ""),
	)

	runReconcile(t, r, "ns1", "req-activity")
	got := getPolicy(t, r, "ns1", "req-activity")

	g.Expect(got.Status.ObservedGeneration).To(gomega.Equal(int64(3)))
	g.Expect(got.Status.AttachedComponents).To(gomega.Equal(int32(3)))

	wantDigest, err := render.PortableDigest(&policy.Spec)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(got.Status.PortableDigest).To(gomega.Equal(wantDigest))

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonTemplatesValid))
	g.Expect(ready.ObservedGeneration).To(gomega.Equal(int64(3)))

	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyInUseCondition)
	g.Expect(inUse).NotTo(gomega.BeNil())
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(inUse.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonAttached))
	g.Expect(inUse.ObservedGeneration).To(gomega.Equal(int64(3)))
}

func TestReconcileNoConsumers(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testProviderConfig,
		mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil)))

	runReconcile(t, r, "ns1", "req-activity")
	got := getPolicy(t, r, "ns1", "req-activity")

	g.Expect(got.Status.AttachedComponents).To(gomega.Equal(int32(0)))
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyInUseCondition)
	g.Expect(inUse).NotTo(gomega.BeNil())
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(inUse.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonNoConsumers))
}

func TestReconcileInvalidSpec(t *testing.T) {
	g := gomega.NewWithT(t)
	// Missing explicit ignoreNullValues on a prometheus trigger is the first
	// (and only) validation issue for this spec.
	spec := kedaSpec("llm-metrics", map[string]string{
		"query":     "sum(rate(http_requests_total[1m]))",
		"threshold": "10",
	})
	r := newReconciler(t, testProviderConfig, mkPolicy("ns1", "bad-policy", 2, spec))

	runReconcile(t, r, "ns1", "bad-policy")
	got := getPolicy(t, r, "ns1", "bad-policy")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(ready.Reason).To(gomega.Equal(render.ReasonExplicitNullValues))
	// Both conditions are written even on an invalid policy's first reconcile.
	g.Expect(apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyInUseCondition)).NotTo(gomega.BeNil())
	// The digest is still reported: a digest describes the spec, not its validity.
	g.Expect(got.Status.PortableDigest).NotTo(gomega.BeEmpty())
}

func TestReconcileUnknownProvider(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testProviderConfig,
		mkPolicy("ns1", "req-activity", 1, kedaSpec("unbound-provider", nil)))

	runReconcile(t, r, "ns1", "req-activity")
	got := getPolicy(t, r, "ns1", "req-activity")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonProviderUnknown))
	g.Expect(ready.Message).To(gomega.ContainSubstring("unbound-provider"))
}

func TestReconcileHPAPolicyNeedsNoConfig(t *testing.T) {
	g := gomega.NewWithT(t)
	// No operator ConfigMap at all: a policy that references no provider
	// must still go Ready without touching the config.
	r := newReconciler(t, "",
		mkPolicy("ns1", "cpu-policy", 1, v1beta1.AutoscalerPolicySpec{Class: v1beta1.AutoscalerHPA}))

	runReconcile(t, r, "ns1", "cpu-policy")
	got := getPolicy(t, r, "ns1", "cpu-policy")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyReadyCondition)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(ready.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonTemplatesValid))
}

func TestReconcileConfigFetchErrorIsTransient(t *testing.T) {
	g := gomega.NewWithT(t)
	// A provider-referencing policy with no reachable operator config is a
	// transient fetch failure: the reconcile must error (and requeue), not
	// mark the policy ProviderUnknown off missing data.
	r := newReconciler(t, "",
		mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil)))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "req-activity"}})
	g.Expect(err).To(gomega.HaveOccurred())

	got := getPolicy(t, r, "ns1", "req-activity")
	g.Expect(got.Status.Conditions).To(gomega.BeEmpty())
}

func TestReconcileSkipsUnchangedStatus(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testProviderConfig,
		mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil)),
		mkISVC("ns1", "consumer-a", "req-activity", "", ""))

	runReconcile(t, r, "ns1", "req-activity")
	first := getPolicy(t, r, "ns1", "req-activity")

	runReconcile(t, r, "ns1", "req-activity")
	second := getPolicy(t, r, "ns1", "req-activity")

	g.Expect(second.ResourceVersion).To(gomega.Equal(first.ResourceVersion))
}

// Provider bindings are consulted from the operator config, which emits no
// watch event toward this controller — the reconcile must self-schedule a
// re-check once per config cache TTL, for both binding outcomes, so Ready
// converges after a binding edit without a policy write.
func TestReconcileRequeuesAfterConsultingBindings(t *testing.T) {
	const ttl = 30 * time.Second
	cases := []struct {
		name   string
		policy *v1beta1.AutoscalerPolicy
	}{
		{"unbound provider", mkPolicy("ns1", "req-activity", 1, kedaSpec("unbound-provider", nil))},
		{"bound provider", mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			r := newReconciler(t, testProviderConfig, tc.policy)
			r.ConfigCache = controllerconfig.NewConfigCache(ttl)

			result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "req-activity"}})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result.RequeueAfter).To(gomega.Equal(ttl))

			// The unchanged-status early return must keep the periodic pass.
			result, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "req-activity"}})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result.RequeueAfter).To(gomega.Equal(ttl))
		})
	}
}

// TTL<=0 means live config reads: no periodic requeue is scheduled. A policy
// that references no provider never consults bindings and needs none either.
func TestReconcileSkipsBindingRequeue(t *testing.T) {
	g := gomega.NewWithT(t)

	// Zero TTL, provider-consulting policy.
	r := newReconciler(t, testProviderConfig, mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil)))
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "req-activity"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))

	// Positive TTL, policy with no provider references.
	r = newReconciler(t, testProviderConfig, mkPolicy("ns1", "cpu-policy", 1, v1beta1.AutoscalerPolicySpec{Class: v1beta1.AutoscalerHPA}))
	r.ConfigCache = controllerconfig.NewConfigCache(30 * time.Second)
	result, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cpu-policy"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
}

func TestReconcileMissingPolicyIsNoop(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testProviderConfig)
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "gone"}})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
}

func TestAttachDetachFlipsInUse(t *testing.T) {
	g := gomega.NewWithT(t)
	consumer := mkISVC("ns1", "consumer-a", "req-activity", "", "")
	r := newReconciler(t, testProviderConfig,
		mkPolicy("ns1", "req-activity", 1, kedaSpec("llm-metrics", nil)),
		consumer)

	runReconcile(t, r, "ns1", "req-activity")
	got := getPolicy(t, r, "ns1", "req-activity")
	g.Expect(got.Status.AttachedComponents).To(gomega.Equal(int32(1)))

	g.Expect(r.Delete(context.Background(), consumer)).To(gomega.Succeed())
	runReconcile(t, r, "ns1", "req-activity")
	got = getPolicy(t, r, "ns1", "req-activity")

	g.Expect(got.Status.AttachedComponents).To(gomega.Equal(int32(0)))
	inUse := apimeta.FindStatusCondition(got.Status.Conditions, v1beta1.AutoscalerPolicyInUseCondition)
	g.Expect(inUse.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(inUse.Reason).To(gomega.Equal(v1beta1.AutoscalerPolicyReasonNoConsumers))
}

func TestPoliciesReferencedBy(t *testing.T) {
	g := gomega.NewWithT(t)
	r := newReconciler(t, testProviderConfig)

	// Distinct names only: engine+decoder share a policy, router names another.
	reqs := r.policiesReferencedBy(context.Background(), mkISVC("ns1", "consumer-a", "policy-a", "policy-a", "policy-b"))
	g.Expect(reqs).To(gomega.ConsistOf(
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-a"}},
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "policy-b"}},
	))

	g.Expect(r.policiesReferencedBy(context.Background(), mkISVC("ns1", "no-refs", "", "", ""))).To(gomega.BeEmpty())
	g.Expect(r.policiesReferencedBy(context.Background(), &corev1.Pod{})).To(gomega.BeNil())
}
