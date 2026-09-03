// Package rolloutpolicy contains the RolloutPolicy status controller. Status
// is pure observation — validity, portable digest, attachment count — and a
// status write can never move traffic: run-open composition happens in the
// consuming InferenceService reconcile, never here.
package rolloutpolicy

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	policycore "sigs.k8s.io/ome/pkg/rolloutpolicy"
	"sigs.k8s.io/ome/pkg/validation"
)

// +kubebuilder:rbac:groups=ome.io,resources=rolloutpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=rolloutpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices,verbs=get;list;watch

// PolicyRefIndexField is the InferenceService field index keyed by the
// distinct RolloutPolicy names the ISVC's rollout groups reference. The
// manager wiring registers PolicyRefIndexer under this key; this controller
// only queries it.
const PolicyRefIndexField = "spec.rolloutPolicyRefs"

// PolicyRefIndexer is the extractor for PolicyRefIndexField: the distinct
// policy names across spec.rollout.groups[].policyRef.
func PolicyRefIndexer(obj client.Object) []string {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	groups := isvc.Spec.GetRolloutGroups()
	for i := range groups {
		ref := groups[i].PolicyRef
		if ref == nil || ref.Name == "" || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		names = append(names, ref.Name)
	}
	return names
}

// ProviderBindings is the seam through which this controller reads the
// cluster-local metric-provider bindings that resolve a canary body's
// prometheus.providerRef. Injected at Setup: the bindings are operator
// configuration and clusters in one fleet legitimately bind different sets,
// so the controller depends on the lookup, not on a config source.
type ProviderBindings interface {
	// Providers returns the set of logical provider names bound on this
	// cluster. A returned error is transient (config fetch) and retries the
	// reconcile; an unbound name is a status condition, never an error.
	Providers() (map[string]struct{}, error)

	// RecheckInterval sizes the periodic requeue scheduled after a reconcile
	// consults bindings — binding edits emit no watch event toward this
	// controller, so without the periodic pass a binding change would never
	// move the Ready reason until the policy itself is rewritten. Zero or
	// negative disables it (live reads).
	RecheckInterval() time.Duration
}

// StatusReconciler owns RolloutPolicy.status and nothing else: observed
// generation, the portable spec digest, the attached-group count, and the
// Ready/InUse conditions. It never composes a plan into a consumer — that is
// the InferenceService reconcile's run-open job — so a policy's status is
// pure observation.
type StatusReconciler struct {
	client.Client
	Providers ProviderBindings
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

// configProviderBindings reads the shared metricProviders operator config —
// the same loader every other consumer uses — with the config-cache TTL as
// the recheck cadence (binding edits emit no policy event, so the periodic
// requeue is how a policy's condition heals unattended).
type configProviderBindings struct {
	clientSet kubernetes.Interface
	cache     *controllerconfig.ConfigCache
}

func (b *configProviderBindings) Providers() (map[string]struct{}, error) {
	cfg, err := controllerconfig.NewMetricProvidersConfigCached(b.cache, b.clientSet)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(cfg))
	for name := range cfg {
		out[name] = struct{}{}
	}
	return out, nil
}

func (b *configProviderBindings) RecheckInterval() time.Duration {
	return b.cache.TTL()
}

// SetupWithManager wires the status controller, mirroring its
// AutoscalerPolicy sibling's signature: policy events reconcile the policy
// itself; InferenceService events fan out to the policies their rollout
// groups reference, so attach/detach flips InUse and the count without
// polling. Provider bindings come from the shared metricProviders operator
// config; the ProviderBindings seam stays injectable for tests.
func SetupWithManager(mgr ctrl.Manager, clientSet kubernetes.Interface, cache *controllerconfig.ConfigCache) error {
	return SetupWithBindings(mgr, &configProviderBindings{clientSet: clientSet, cache: cache})
}

// SetupWithBindings is SetupWithManager with the provider seam injected —
// the test entry point (suites bind a static provider set instead of the
// operator ConfigMap).
func SetupWithBindings(mgr ctrl.Manager, providers ProviderBindings) error {
	r := &StatusReconciler{
		Client:    mgr.GetClient(),
		Providers: providers,
		Log:       ctrl.Log.WithName("controllers").WithName("RolloutPolicyStatus"),
		Scheme:    mgr.GetScheme(),
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("rolloutpolicy-status").
		For(&v1beta1.RolloutPolicy{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1beta1.InferenceService{},
			handler.EnqueueRequestsFromMapFunc(r.policiesReferencedBy)).
		Complete(r)
}

func (r *StatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &v1beta1.RolloutPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	attached, err := r.attachedGroups(ctx, policy)
	if err != nil {
		return ctrl.Result{}, err
	}

	digest, digestErr := policycore.PortableDigest(&policy.Spec)
	readyCond, consultedBindings, err := r.readyCondition(policy, digestErr)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Provider bindings live in operator configuration, which emits no watch
	// event toward this controller (and GenerationChangedPredicate filters
	// resyncs), so an outcome that consulted them — bound or unbound — is
	// re-checked on the bindings' own cadence. Without this, a binding edit
	// would never move the Ready reason until the policy itself is rewritten.
	result := ctrl.Result{}
	if consultedBindings && r.Providers != nil {
		if interval := r.Providers.RecheckInterval(); interval > 0 {
			result.RequeueAfter = interval
		}
	}

	newStatus := *policy.Status.DeepCopy()
	newStatus.ObservedGeneration = policy.Generation
	newStatus.PortableDigest = digest
	newStatus.AttachedGroups = attached
	apimeta.SetStatusCondition(&newStatus.Conditions, readyCond)
	apimeta.SetStatusCondition(&newStatus.Conditions, inUseCondition(policy.Generation, attached))

	if equality.Semantic.DeepEqual(policy.Status, newStatus) {
		return result, nil
	}
	policy.Status = newStatus
	if updateErr := r.Status().Update(ctx, policy); updateErr != nil {
		if apierrors.IsConflict(updateErr) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, updateErr
	}
	return result, nil
}

// attachedGroups counts rollout groups — not InferenceServices — in the
// policy's namespace whose policyRef names it. One ISVC referencing the
// policy from two groups counts twice: the group is the unit of attachment,
// so the count matches what a detach actually releases.
func (r *StatusReconciler) attachedGroups(ctx context.Context, policy *v1beta1.RolloutPolicy) (int32, error) {
	consumers := &v1beta1.InferenceServiceList{}
	if err := r.List(ctx, consumers,
		client.InNamespace(policy.Namespace),
		client.MatchingFields{PolicyRefIndexField: policy.Name}); err != nil {
		return 0, fmt.Errorf("list consumers of RolloutPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
	}
	var count int32
	for i := range consumers.Items {
		groups := consumers.Items[i].Spec.GetRolloutGroups()
		for j := range groups {
			ref := groups[j].PolicyRef
			if ref != nil && ref.Name == policy.Name {
				count++
			}
		}
	}
	return count, nil
}

// readyCondition derives Ready from the body validation an inline progression
// faces plus the policy-only portability rules. The one cluster-local input —
// whether the body's providerRef is bound here — can only soften the reason,
// never the status: members of one fleet legitimately bind different provider
// sets, so unbound-on-this-cluster is a warning (Ready stays True, reason
// ProviderUnbound), while a run opened on this cluster parks at run open. The
// boolean reports whether bindings were consulted — the caller sizes the
// periodic re-check off it. The error return is transient only (binding
// fetch); invalid content is a condition, never a reconcile error.
func (r *StatusReconciler) readyCondition(policy *v1beta1.RolloutPolicy, digestErr error) (metav1.Condition, bool, error) {
	if err := validation.ValidateRolloutPolicySpec(&policy.Spec); err != nil {
		return metav1.Condition{
			Type:               v1beta1.RolloutPolicyReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: policy.Generation,
			Reason:             v1beta1.RolloutPolicyReasonBodyInvalid,
			Message:            err.Error(),
		}, false, nil
	}
	if digestErr != nil {
		return metav1.Condition{
			Type:               v1beta1.RolloutPolicyReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: policy.Generation,
			Reason:             v1beta1.RolloutPolicyReasonBodyInvalid,
			Message:            fmt.Sprintf("compute portable digest: %v", digestErr),
		}, false, nil
	}

	provider := referencedProvider(&policy.Spec)
	if provider == "" {
		return metav1.Condition{
			Type:               v1beta1.RolloutPolicyReadyCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: policy.Generation,
			Reason:             v1beta1.RolloutPolicyReasonBodyValid,
			Message:            "progression body passes plan validation",
		}, false, nil
	}

	bound, err := r.boundProviders()
	if err != nil {
		return metav1.Condition{}, true, err
	}
	if _, ok := bound[provider]; !ok {
		return metav1.Condition{
			Type:               v1beta1.RolloutPolicyReadyCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: policy.Generation,
			Reason:             v1beta1.RolloutPolicyReasonProviderUnbound,
			Message:            fmt.Sprintf("progression body is valid, but metric provider %q is not bound on this cluster — a rollout opened here parks at run open until the binding exists", provider),
		}, true, nil
	}
	return metav1.Condition{
		Type:               v1beta1.RolloutPolicyReadyCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		Reason:             v1beta1.RolloutPolicyReasonBodyValid,
		Message:            "progression body passes plan validation and its metric provider is bound",
	}, true, nil
}

// boundProviders reads the binding set through the injected seam. A nil seam
// means the wiring supplies no binding source, which is indistinguishable
// from an empty binding set: every referenced name reports unbound.
func (r *StatusReconciler) boundProviders() (map[string]struct{}, error) {
	if r.Providers == nil {
		return nil, nil
	}
	return r.Providers.Providers()
}

// referencedProvider returns the logical provider name the spec's canary
// metrics source declares, or "". Only canary bodies carry a metrics source,
// and policy admission rejects the raw serverAddress spelling, so providerRef
// is the only form that can appear on a valid body.
func referencedProvider(spec *v1beta1.RolloutPolicySpec) string {
	if spec.Canary == nil || spec.Canary.Prometheus == nil || spec.Canary.Prometheus.ProviderRef == nil {
		return ""
	}
	return spec.Canary.Prometheus.ProviderRef.Name
}

func inUseCondition(generation int64, attached int32) metav1.Condition {
	if attached > 0 {
		return metav1.Condition{
			Type:               v1beta1.RolloutPolicyInUseCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             v1beta1.RolloutPolicyReasonAttached,
			Message:            fmt.Sprintf("%d rollout group(s) reference this policy", attached),
		}
	}
	return metav1.Condition{
		Type:               v1beta1.RolloutPolicyInUseCondition,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             v1beta1.RolloutPolicyReasonNoConsumers,
		Message:            "no rollout group references this policy",
	}
}

// policiesReferencedBy maps an InferenceService event to the same-namespace
// policies its rollout groups reference. Update events map both the old and
// new object, so a detach enqueues the policy losing the ref as well as the
// one gaining it.
func (r *StatusReconciler) policiesReferencedBy(_ context.Context, obj client.Object) []reconcile.Request {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	var requests []reconcile.Request
	for _, name := range PolicyRefIndexer(isvc) {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Namespace: isvc.Namespace, Name: name},
		})
	}
	return requests
}
