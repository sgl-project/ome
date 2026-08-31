package traffic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/traffic/status"
)

var fixedNow = metav1.NewTime(time.Date(2026, time.May, 17, 12, 0, 0, 0, time.UTC))

// stubTranslator records what it was called with and returns a
// pre-programmed response. Avoids a dependency on any real translator
// implementation so this test stays self-contained.
type stubTranslator struct {
	name string

	gotISVC   *v1beta1.InferenceService
	gotRoutes []string
	gotIntent *ResolvedIntent

	respObj          client.Object
	respPassthroughs []string
	respErr          error

	supports         sets.Set[string]
	supportsFields   sets.Set[string]
	supportedPrefixs []string
	watches          client.Object

	// observeFunc lets individual tests script gateway-acceptance
	// observations. When nil, ObserveAcceptance returns Pending.
	observeFunc  func(obj client.Object) AcceptanceObservation
	observedWith client.Object
}

func (s *stubTranslator) Name() string { return s.name }
func (s *stubTranslator) SupportedAnnotations() sets.Set[string] {
	if s.supports == nil {
		return sets.New[string]()
	}
	return s.supports
}
func (s *stubTranslator) Watches() client.Object                 { return s.watches }
func (s *stubTranslator) SupportedPassthroughPrefixes() []string { return s.supportedPrefixs }
func (s *stubTranslator) SupportedTrafficFields() sets.Set[string] {
	if s.supportsFields == nil {
		return sets.New[string]()
	}
	return s.supportsFields
}
func (s *stubTranslator) Translate(
	isvc *v1beta1.InferenceService,
	targetHTTPRoutes []string,
	intent *ResolvedIntent,
) (client.Object, []string, error) {
	s.gotISVC = isvc
	s.gotRoutes = targetHTTPRoutes
	s.gotIntent = intent
	return s.respObj, s.respPassthroughs, s.respErr
}
func (s *stubTranslator) ObserveAcceptance(obj client.Object) AcceptanceObservation {
	s.observedWith = obj
	if s.observeFunc == nil {
		return AcceptanceObservation{State: AcceptancePending}
	}
	return s.observeFunc(obj)
}

func newReconcilerWithStub(t *testing.T, tr *stubTranslator, seed ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add clientgo scheme: %v", err)
	}
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1beta1 scheme: %v", err)
	}
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(seed...).Build()
	r := NewReconciler(c, scheme, tr)
	r.now = func() metav1.Time { return fixedNow }
	return r, c
}

func newISVC(name string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Generation: 4},
	}
}

func TestReconcile_NoIntent_ReturnsNilSkipsTranslator(t *testing.T) {
	tr := &stubTranslator{name: status.NoopTranslatorName}
	r, _ := newReconcilerWithStub(t, tr)

	got, err := r.Reconcile(context.Background(), newISVC("isvc"), []string{"isvc"})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("Reconcile must return nil when no intent declared, got %+v", got)
	}
	if tr.gotISVC != nil {
		t.Fatalf("translator must NOT be invoked when HasIntent=false (called with %+v)", tr.gotISVC)
	}
}

func TestReconcile_Noop_ReturnsNoTranslatorAvailableStatus(t *testing.T) {
	// Intent declared (retry annotation), noop translator active.
	tr := &stubTranslator{name: status.NoopTranslatorName}
	r, _ := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	alg := v1beta1.LoadBalancingTypeRoundRobin
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: &alg}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc", "isvc-engine"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got == nil {
		t.Fatalf("expected TrafficStatus, got nil")
	}
	if got.Conditions[0].Reason != v1beta1.TrafficReasonNoTranslatorAvailable {
		t.Fatalf("Reason = %q, want NoTranslatorAvailable", got.Conditions[0].Reason)
	}
	if got.Conditions[0].ObservedGeneration != isvc.Generation {
		t.Fatalf("ObservedGeneration = %d, want %d", got.Conditions[0].ObservedGeneration, isvc.Generation)
	}
}

func TestReconcile_RealTranslator_AppliesPolicyAndReturnsPendingStatus(t *testing.T) {
	// Use a registered scheme type (ConfigMap) as a stand-in for the
	// emitted backend policy. The reconciler is generic over
	// client.Object so this is sufficient to exercise the apply path
	// without depending on the Envoy Gateway types.
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
		Data:       map[string]string{"mock": "policy"},
	}
	tr := &stubTranslator{
		name:             "envoy-gateway",
		respObj:          desired,
		respPassthroughs: []string{"loadBalancer.slowStart.window"},
	}
	r, c := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	alg := v1beta1.LoadBalancingTypeConsistentHash
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: &alg}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc", "isvc-engine"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	// Status checks.
	if got.Algorithm != string(v1beta1.LoadBalancingTypeConsistentHash) {
		t.Fatalf("LoadBalancingType = %q", got.Algorithm)
	}
	if got.Conditions[0].Reason != v1beta1.TrafficReasonPending {
		t.Fatalf("Reason = %q, want Pending", got.Conditions[0].Reason)
	}

	// Verify the policy was created and the owner reference points at
	// the InferenceService.
	got2 := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, got2); err != nil {
		t.Fatalf("policy not created: %v", err)
	}
	if len(got2.OwnerReferences) != 1 || got2.OwnerReferences[0].Name != isvc.Name {
		t.Fatalf("owner reference not set on policy: %+v", got2.OwnerReferences)
	}
	if got2.Data["mock"] != "policy" {
		t.Fatalf("policy data not preserved: %+v", got2.Data)
	}
}

func TestReconcile_RealTranslator_UpdatesExistingPolicy(t *testing.T) {
	// Seed an existing policy resource — Reconcile must Update
	// instead of Create and the new Data must win. The existing object
	// carries OME's controller-ref so the ConflictingPolicy guard
	// recognizes it as OME-managed.
	isvc := newISVC("isvc")
	isvc.UID = "isvc-uid"
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "isvc-btp",
			Namespace:       "default",
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "ome.io/v1beta1",
				Kind:       "InferenceService",
				Name:       isvc.Name,
				UID:        isvc.UID,
				Controller: ptr.To(true),
			}},
		},
		Data: map[string]string{"mock": "stale"},
	}
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
		Data:       map[string]string{"mock": "fresh"},
	}
	tr := &stubTranslator{name: "envoy-gateway", respObj: desired}
	r, c := newReconcilerWithStub(t, tr, existing)

	isvc.Spec.Traffic = &v1beta1.TrafficSpec{
		Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin),
	}

	if _, err := r.Reconcile(context.Background(), isvc, []string{"isvc"}); err != nil {
		t.Fatalf("err = %v", err)
	}

	updated := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, updated); err != nil {
		t.Fatalf("policy missing: %v", err)
	}
	if updated.Data["mock"] != "fresh" {
		t.Fatalf("policy not updated: %+v", updated.Data)
	}
}

func TestReconcile_ConflictingPolicy_RefusesOverwriteAndSurfacesReason(t *testing.T) {
	// Pre-existing same-name policy with NO controller-ref simulates a
	// hand-authored BTP. Reconcile must refuse to Update and surface
	// BackendPolicyReady=False/ConflictingPolicy (alpha-phase coexistence
	// behavior).
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "isvc-btp",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Data: map[string]string{"mock": "handauthored"},
	}
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
		Data:       map[string]string{"mock": "would-have-been-ome"},
	}
	tr := &stubTranslator{name: "envoy-gateway", respObj: desired}
	r, c := newReconcilerWithStub(t, tr, existing)

	isvc := newISVC("isvc")
	isvc.UID = "isvc-uid"
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{
		Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin),
	}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err == nil {
		t.Fatalf("Reconcile must return the wrapped ErrConflictingPolicy")
	}
	if !errors.Is(err, ErrConflictingPolicy) {
		t.Fatalf("err must wrap ErrConflictingPolicy, got: %v", err)
	}

	if got == nil || len(got.Conditions) == 0 {
		t.Fatalf("status must include the BackendPolicyReady condition; got = %+v", got)
	}
	if got.Conditions[0].Reason != v1beta1.TrafficReasonConflictingPolicy {
		t.Fatalf("Reason = %q, want %q", got.Conditions[0].Reason, v1beta1.TrafficReasonConflictingPolicy)
	}

	// Existing object MUST NOT have been overwritten.
	preserved := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, preserved); err != nil {
		t.Fatalf("existing policy missing: %v", err)
	}
	if preserved.Data["mock"] != "handauthored" {
		t.Fatalf("ConflictingPolicy guard overwrote a hand-authored policy: %+v", preserved.Data)
	}
}

func TestReconcile_TranslatorError_NoApplySurfacedInStatus(t *testing.T) {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	tr := &stubTranslator{
		name:    "envoy-gateway",
		respObj: desired, // non-nil but should be ignored due to err
		respErr: errors.New("invalid hash type"),
	}
	r, c := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err == nil {
		t.Fatalf("Reconcile must propagate translator error")
	}
	if got.Conditions[0].Reason != v1beta1.TrafficReasonTranslationFailed {
		t.Fatalf("Reason = %q, want TranslationFailed", got.Conditions[0].Reason)
	}

	// Apply must NOT have been attempted on translator error.
	cm := &corev1.ConfigMap{}
	getErr := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, cm)
	if getErr == nil {
		t.Fatalf("policy must not be created when translator errors")
	}
}

func TestReconcile_PassesTargetRoutesIntoTranslator(t *testing.T) {
	tr := &stubTranslator{name: "envoy-gateway"}
	r, _ := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}
	routes := []string{"isvc", "isvc-engine", "isvc-decoder"}

	if _, err := r.Reconcile(context.Background(), isvc, routes); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(tr.gotRoutes) != len(routes) {
		t.Fatalf("translator received %v, want %v", tr.gotRoutes, routes)
	}
	for i := range routes {
		if tr.gotRoutes[i] != routes[i] {
			t.Fatalf("route %d mismatch: got %q want %q", i, tr.gotRoutes[i], routes[i])
		}
	}
}

func TestComputeTargetHTTPRoutes(t *testing.T) {
	cases := []struct {
		name       string
		isvcName   string
		hasDecoder bool
		hasRouter  bool
		wantRoutes []string
	}{
		{
			name:       "engine only",
			isvcName:   "foo",
			wantRoutes: []string{"foo", "foo-engine"},
		},
		{
			name:       "engine + decoder",
			isvcName:   "foo",
			hasDecoder: true,
			wantRoutes: []string{"foo", "foo-engine", "foo-decoder"},
		},
		{
			name:       "engine + router",
			isvcName:   "foo",
			hasRouter:  true,
			wantRoutes: []string{"foo", "foo-engine", "foo-router"},
		},
		{
			name:       "engine + decoder + router (PD-disaggregated)",
			isvcName:   "foo",
			hasDecoder: true,
			hasRouter:  true,
			wantRoutes: []string{"foo", "foo-engine", "foo-decoder", "foo-router"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeTargetHTTPRoutes(
				&v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: tc.isvcName}},
				tc.hasDecoder,
				tc.hasRouter,
			)
			if len(got) != len(tc.wantRoutes) {
				t.Fatalf("len = %d, want %d (got %v)", len(got), len(tc.wantRoutes), got)
			}
			for i := range tc.wantRoutes {
				if got[i] != tc.wantRoutes[i] {
					t.Fatalf("route %d: got %q want %q", i, got[i], tc.wantRoutes[i])
				}
			}
		})
	}
}

func TestReconciler_TranslatorNameAndWatchesPassThrough(t *testing.T) {
	wantWatch := &corev1.ConfigMap{}
	tr := &stubTranslator{name: "envoy-gateway", watches: wantWatch}
	r, _ := newReconcilerWithStub(t, tr)
	if r.TranslatorName() != "envoy-gateway" {
		t.Fatalf("TranslatorName = %q", r.TranslatorName())
	}
	if r.Watches() != wantWatch {
		t.Fatalf("Watches = %v, want %v", r.Watches(), wantWatch)
	}
}

func TestReconcile_GatewayAccepted_SurfacesAcceptedByGateway(t *testing.T) {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	tr := &stubTranslator{
		name:    "envoy-gateway",
		respObj: desired,
		observeFunc: func(_ client.Object) AcceptanceObservation {
			return AcceptanceObservation{
				State:   AcceptanceAccepted,
				Reason:  "Accepted",
				Message: "Policy is valid and accepted",
			}
		},
	}
	r, _ := newReconcilerWithStub(t, tr)
	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	c := got.Conditions[0]
	if c.Status != metav1.ConditionTrue {
		t.Fatalf("Status = %q, want True", c.Status)
	}
	if c.Reason != v1beta1.TrafficReasonAcceptedByGateway {
		t.Fatalf("Reason = %q, want AcceptedByGateway", c.Reason)
	}
	if c.Message != "Policy is valid and accepted" {
		t.Fatalf("Message = %q, want pass-through from observation", c.Message)
	}
	if tr.observedWith == nil {
		t.Fatalf("ObserveAcceptance was not called")
	}
}

func TestReconcile_GatewayRejected_SurfacesGatewayRejected(t *testing.T) {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	tr := &stubTranslator{
		name:    "envoy-gateway",
		respObj: desired,
		observeFunc: func(_ client.Object) AcceptanceObservation {
			return AcceptanceObservation{
				State:   AcceptanceRejected,
				Reason:  "Invalid",
				Message: "unsupported field foo.bar",
			}
		},
	}
	r, _ := newReconcilerWithStub(t, tr)
	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	c := got.Conditions[0]
	if c.Status != metav1.ConditionFalse {
		t.Fatalf("Status = %q, want False", c.Status)
	}
	if c.Reason != v1beta1.TrafficReasonGatewayRejected {
		t.Fatalf("Reason = %q, want GatewayRejected", c.Reason)
	}
	if c.Message != "unsupported field foo.bar" {
		t.Fatalf("Message = %q, want gateway controller message", c.Message)
	}
}

func TestReconcile_GatewayPending_DefaultsCondition(t *testing.T) {
	// observeFunc nil -> stub returns Pending. The reconciler must
	// then keep the BackendPolicyReady condition at Unknown/Pending.
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	tr := &stubTranslator{name: "envoy-gateway", respObj: desired}
	r, _ := newReconcilerWithStub(t, tr)
	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	c := got.Conditions[0]
	if c.Status != metav1.ConditionUnknown {
		t.Fatalf("Status = %q, want Unknown", c.Status)
	}
	if c.Reason != v1beta1.TrafficReasonPending {
		t.Fatalf("Reason = %q, want Pending", c.Reason)
	}
}

func TestReconcile_NoIntent_NoPriorStatus_NoDeleteAttempted(t *testing.T) {
	// Fresh ISVC, no intent, no prior status: cleanup must skip the
	// Delete API call entirely so the common "ISVC never had traffic
	// intent" case is free.
	tr := &stubTranslator{name: "envoy-gateway", watches: &corev1.ConfigMap{}}
	r, c := newReconcilerWithStub(t, tr)
	// Seed a phantom policy with the ISVC's name to prove no Delete
	// touches it. If the reconciler called Delete, this ConfigMap
	// would disappear.
	phantom := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc", Namespace: "default"},
	}
	if err := c.Create(context.Background(), phantom); err != nil {
		t.Fatalf("seed: %v", err)
	}

	isvc := newISVC("isvc")
	// isvc.Status.Traffic stays nil.

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != nil {
		t.Fatalf("status must be nil when no intent, got %+v", got)
	}
	// Phantom must still exist — proves no Delete was attempted.
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc"}, cm); err != nil {
		t.Fatalf("phantom was unexpectedly deleted: %v", err)
	}
}

func TestReconcile_NoIntent_PriorPolicyExists_DeletesIt(t *testing.T) {
	tr := &stubTranslator{name: "envoy-gateway", watches: &corev1.ConfigMap{}}
	// Seed a BTP-stand-in that a prior reconcile would have emitted.
	prior := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	r, c := newReconcilerWithStub(t, tr, prior)

	isvc := newISVC("isvc")
	// Status reflects a previous successful emit.
	isvc.Status.Traffic = &v1beta1.TrafficStatus{
		BackendPolicyResource: &v1beta1.BackendPolicyRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "isvc-btp",
		},
	}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != nil {
		t.Fatalf("status must be nil when no intent, got %+v", got)
	}
	// Prior policy must be gone.
	cm := &corev1.ConfigMap{}
	getErr := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, cm)
	if getErr == nil {
		t.Fatalf("prior policy still exists after cleanup")
	}
}

func TestReconcile_NoIntent_PriorPolicyAlreadyGone_NotFoundIsOK(t *testing.T) {
	// Status says we emitted a policy but it's already gone (manual
	// delete, GC race). Cleanup must treat NotFound as a no-op.
	tr := &stubTranslator{name: "envoy-gateway", watches: &corev1.ConfigMap{}}
	r, _ := newReconcilerWithStub(t, tr) // no seed

	isvc := newISVC("isvc")
	isvc.Status.Traffic = &v1beta1.TrafficStatus{
		BackendPolicyResource: &v1beta1.BackendPolicyRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "isvc-btp",
		},
	}

	if _, err := r.Reconcile(context.Background(), isvc, []string{"isvc"}); err != nil {
		t.Fatalf("NotFound must not propagate, got: %v", err)
	}
}

func TestReconcile_NoIntent_NoopTranslator_NeverDeletes(t *testing.T) {
	// Noop's Watches() returns nil; cleanup must skip even if status
	// somehow has a BackendPolicyResource (it shouldn't, but defense
	// in depth).
	tr := &stubTranslator{name: status.NoopTranslatorName, watches: nil}
	phantom := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	r, c := newReconcilerWithStub(t, tr, phantom)

	isvc := newISVC("isvc")
	isvc.Status.Traffic = &v1beta1.TrafficStatus{
		BackendPolicyResource: &v1beta1.BackendPolicyRef{Name: "isvc-btp"},
	}

	if _, err := r.Reconcile(context.Background(), isvc, []string{"isvc"}); err != nil {
		t.Fatalf("err = %v", err)
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "isvc-btp"}, cm); err != nil {
		t.Fatalf("noop translator must not delete: %v", err)
	}
}

func TestReconcile_TranslateError_SkipsAcceptanceObservation(t *testing.T) {
	// Apply path is skipped on translator error, and so must
	// ObserveAcceptance — otherwise we'd report a stale acceptance
	// from a previous reconcile that does not reflect the current
	// (failed) attempt.
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	observed := false
	tr := &stubTranslator{
		name:    "envoy-gateway",
		respObj: desired,
		respErr: errors.New("boom"),
		observeFunc: func(_ client.Object) AcceptanceObservation {
			observed = true
			return AcceptanceObservation{State: AcceptanceAccepted}
		},
	}
	r, _ := newReconcilerWithStub(t, tr)
	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}

	_, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err == nil {
		t.Fatalf("expected translator error to propagate")
	}
	if observed {
		t.Fatalf("ObserveAcceptance must NOT be called when translator errors")
	}
}

func TestReconcile_SurfacesUnsupportedAnnotationsFromIsvc(t *testing.T) {
	// EG-style translator: supports only retry-attempts and the EG
	// pass-through prefix. The ISVC declares an Istio pass-through and
	// an unsupported per-key annotation — both should appear in the
	// UnsupportedFields condition's message.
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-btp", Namespace: "default"},
	}
	tr := &stubTranslator{
		name:             "envoy-gateway",
		respObj:          desired,
		watches:          &corev1.ConfigMap{},
		supports:         sets.New("ome.io/retry-attempts"),
		supportedPrefixs: []string{"ome.io/btp."},
	}
	r, _ := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin)}
	isvc.Annotations = map[string]string{
		"ome.io/retry-attempts":                  "3",
		"ome.io/circuit-breaker-max-connections": "1024", // unsupported per-key
		"ome.io/dr.trafficPolicy.connectionPool": "30s",  // unsupported prefix
	}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d: %+v", len(got.Conditions), got.Conditions)
	}
	var uc *metav1.Condition
	for i := range got.Conditions {
		if got.Conditions[i].Type == v1beta1.TrafficConditionBackendPolicyUnsupportedFields {
			uc = &got.Conditions[i]
		}
	}
	if uc == nil {
		t.Fatalf("UnsupportedFields condition missing: %+v", got.Conditions)
	}
	for _, want := range []string{
		"ome.io/circuit-breaker-max-connections",
		"ome.io/dr.trafficPolicy.connectionPool",
	} {
		if !strings.Contains(uc.Message, want) {
			t.Errorf("Message should list %q, got: %q", want, uc.Message)
		}
	}
}

func TestReconcile_SurfacesUnsupportedTypedTrafficFields(t *testing.T) {
	// DestinationRule-style translator: declares algorithm +
	// single-header hashing, but not endpoint override. The ISVC's
	// typed endpointOverride must surface in the UnsupportedFields
	// condition message along with the translator name.
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "isvc-dr", Namespace: "default"},
	}
	tr := &stubTranslator{
		name:    "istio",
		respObj: desired,
		watches: &corev1.ConfigMap{},
		supportsFields: sets.New(
			"spec.traffic.algorithm",
			"spec.traffic.consistentHash.type=Header",
		),
	}
	r, _ := newReconcilerWithStub(t, tr)

	isvc := newISVC("isvc")
	isvc.Spec.Traffic = &v1beta1.TrafficSpec{
		Algorithm: ptrAlg(v1beta1.LoadBalancingTypeRoundRobin),
		EndpointOverride: &v1beta1.EndpointOverrideSpec{
			Type:    v1beta1.EndpointOverrideTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-endpoint"}},
		},
	}

	got, err := r.Reconcile(context.Background(), isvc, []string{"isvc"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var uc *metav1.Condition
	for i := range got.Conditions {
		if got.Conditions[i].Type == v1beta1.TrafficConditionBackendPolicyUnsupportedFields {
			uc = &got.Conditions[i]
		}
	}
	if uc == nil {
		t.Fatalf("UnsupportedFields condition missing: %+v", got.Conditions)
	}
	if !strings.Contains(uc.Message, "spec.traffic.endpointOverride.type=Header") {
		t.Errorf("Message should name the dropped typed field, got: %q", uc.Message)
	}
	if !strings.Contains(uc.Message, "istio") {
		t.Errorf("Message should name the active translator, got: %q", uc.Message)
	}
	// The declared algorithm IS honored and must not be listed.
	if strings.Contains(uc.Message, "spec.traffic.algorithm") {
		t.Errorf("Message must not list honored fields, got: %q", uc.Message)
	}
}

func ptrAlg(a v1beta1.LoadBalancingType) *v1beta1.LoadBalancingType { return &a }
