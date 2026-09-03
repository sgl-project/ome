package autoscalerpolicy

import (
	"context"
	"fmt"

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
	"sigs.k8s.io/ome/pkg/autoscalerpolicy/render"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
)

// +kubebuilder:rbac:groups=ome.io,resources=autoscalerpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=autoscalerpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices,verbs=get;list;watch

// autoscalerPolicyRefIndexField is the InferenceService field index keyed by
// the distinct policy names an ISVC's components reference. The
// InferenceService controller registers the extractor; this controller only
// queries it.
const autoscalerPolicyRefIndexField = "spec.autoscalerPolicyRefs"

// policyRefComponents is every component slot that can carry a policy ref;
// attachment accounting and watch fan-out iterate the same set the
// InferenceService controller's index extractor does.
var policyRefComponents = []v1beta1.ComponentType{
	v1beta1.EngineComponent,
	v1beta1.DecoderComponent,
	v1beta1.RouterComponent,
}

// StatusReconciler owns AutoscalerPolicy.status and nothing else: observed
// generation, the portable spec digest, the attached-component count, and
// the Ready/InUse conditions. It never renders into workloads — dispatch is
// the consuming InferenceService reconcile's job — so a policy's status is
// pure observation and a status write can never actuate a scaler.
type StatusReconciler struct {
	client.Client
	Clientset   kubernetes.Interface
	ConfigCache *controllerconfig.ConfigCache
	Log         logr.Logger
	Scheme      *runtime.Scheme
}

// SetupWithManager wires the status controller: policy events reconcile the
// policy itself; InferenceService events fan out to the policies their
// components reference, so attach/detach flips InUse and the count without
// polling.
func SetupWithManager(mgr ctrl.Manager, clientset kubernetes.Interface, cache *controllerconfig.ConfigCache) error {
	r := &StatusReconciler{
		Client:      mgr.GetClient(),
		Clientset:   clientset,
		ConfigCache: cache,
		Log:         ctrl.Log.WithName("controllers").WithName("AutoscalerPolicyStatus"),
		Scheme:      mgr.GetScheme(),
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("autoscalerpolicy-status").
		For(&v1beta1.AutoscalerPolicy{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1beta1.InferenceService{},
			handler.EnqueueRequestsFromMapFunc(r.policiesReferencedBy)).
		Complete(r)
}

func (r *StatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &v1beta1.AutoscalerPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	attached, err := r.attachedComponents(ctx, policy)
	if err != nil {
		return ctrl.Result{}, err
	}

	digest, digestErr := render.PortableDigest(&policy.Spec)
	readyCond, consultedBindings, err := r.readyCondition(policy, digestErr)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Provider bindings live in the operator config, which emits no watch
	// event toward this controller (and GenerationChangedPredicate filters
	// resyncs), so a Ready outcome that consulted them — bound or unbound —
	// is re-checked once per config cache TTL. Without this, a binding edit
	// would never flip Ready until the policy itself is rewritten. TTL<=0
	// means live config reads; the periodic pass is skipped then.
	result := ctrl.Result{}
	if consultedBindings {
		if ttl := r.ConfigCache.TTL(); ttl > 0 {
			result.RequeueAfter = ttl
		}
	}

	newStatus := *policy.Status.DeepCopy()
	newStatus.ObservedGeneration = policy.Generation
	newStatus.PortableDigest = digest
	newStatus.AttachedComponents = attached
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

// attachedComponents counts component slots — not InferenceServices — in the
// policy's namespace whose ref names it. One ISVC referencing the policy
// from both engine and decoder counts twice: a component is the unit of
// attachment, so the count matches what a detach actually releases.
func (r *StatusReconciler) attachedComponents(ctx context.Context, policy *v1beta1.AutoscalerPolicy) (int32, error) {
	consumers := &v1beta1.InferenceServiceList{}
	if err := r.List(ctx, consumers,
		client.InNamespace(policy.Namespace),
		client.MatchingFields{autoscalerPolicyRefIndexField: policy.Name}); err != nil {
		return 0, fmt.Errorf("list consumers of AutoscalerPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
	}
	var count int32
	for i := range consumers.Items {
		for _, componentType := range policyRefComponents {
			ref := autoscaler.ComponentPolicyRef(&consumers.Items[i], componentType)
			if ref != nil && ref.Name == policy.Name {
				count++
			}
		}
	}
	return count, nil
}

// readyCondition derives Ready from cluster-independent validation plus the
// one cluster-local input: every referenced provider name must be bound in
// the operator config. The boolean reports whether provider bindings were
// consulted — the caller sizes a periodic re-check off it, since binding
// edits emit no watch event. The error return is transient only (config
// fetch); invalid content is a condition, never a reconcile error.
func (r *StatusReconciler) readyCondition(policy *v1beta1.AutoscalerPolicy, digestErr error) (metav1.Condition, bool, error) {
	issues := render.ValidateSpec(&policy.Spec)
	if digestErr != nil {
		issues = append([]render.Issue{{
			Reason: v1beta1.AutoscalerPolicyReasonParseError,
			Detail: fmt.Sprintf("compute portable digest: %v", digestErr),
		}}, issues...)
	}
	if len(issues) > 0 {
		return metav1.Condition{
			Type:               v1beta1.AutoscalerPolicyReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: policy.Generation,
			Reason:             issues[0].Reason,
			Message:            issues[0].Detail,
		}, false, nil
	}

	unbound, consulted, err := r.firstUnboundProvider(&policy.Spec)
	if err != nil {
		return metav1.Condition{}, consulted, err
	}
	if unbound != "" {
		return metav1.Condition{
			Type:               v1beta1.AutoscalerPolicyReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: policy.Generation,
			Reason:             v1beta1.AutoscalerPolicyReasonProviderUnknown,
			Message:            fmt.Sprintf("metric provider %q is not bound in the operator configuration", unbound),
		}, consulted, nil
	}

	return metav1.Condition{
		Type:               v1beta1.AutoscalerPolicyReadyCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		Reason:             v1beta1.AutoscalerPolicyReasonTemplatesValid,
		Message:            "templates validate and every referenced metric provider is bound",
	}, consulted, nil
}

// firstUnboundProvider returns the first trigger-referenced provider name
// with no binding in the operator config, or "" when all are bound. The
// boolean reports whether the config was consulted at all: it is fetched
// only when the spec references a provider, so a policy with no endpoint
// triggers needs no operator config to go Ready.
func (r *StatusReconciler) firstUnboundProvider(spec *v1beta1.AutoscalerPolicySpec) (string, bool, error) {
	referenced := referencedProviders(spec)
	if len(referenced) == 0 {
		return "", false, nil
	}
	cfg, err := controllerconfig.NewAutoscalerPolicyConfigCached(r.ConfigCache, r.Clientset)
	if err != nil {
		return "", true, fmt.Errorf("load autoscalerPolicy config: %w", err)
	}
	for _, name := range referenced {
		if _, ok := cfg.MetricProviders[name]; !ok {
			return name, true, nil
		}
	}
	return "", true, nil
}

// referencedProviders collects the distinct provider names the spec's
// triggers bind, in first-reference order.
func referencedProviders(spec *v1beta1.AutoscalerPolicySpec) []string {
	if spec.Keda == nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for i := range spec.Keda.Triggers {
		ref := spec.Keda.Triggers[i].ProviderRef
		if ref == nil || ref.Name == "" || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		names = append(names, ref.Name)
	}
	return names
}

func inUseCondition(generation int64, attached int32) metav1.Condition {
	if attached > 0 {
		return metav1.Condition{
			Type:               v1beta1.AutoscalerPolicyInUseCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             v1beta1.AutoscalerPolicyReasonAttached,
			Message:            fmt.Sprintf("%d component(s) reference this policy", attached),
		}
	}
	return metav1.Condition{
		Type:               v1beta1.AutoscalerPolicyInUseCondition,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             v1beta1.AutoscalerPolicyReasonNoConsumers,
		Message:            "no component references this policy",
	}
}

// policiesReferencedBy maps an InferenceService event to the same-namespace
// policies its components reference. Update events map both the old and new
// object, so a detach enqueues the policy losing the ref as well as the one
// gaining it.
func (r *StatusReconciler) policiesReferencedBy(_ context.Context, obj client.Object) []reconcile.Request {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var requests []reconcile.Request
	for _, componentType := range policyRefComponents {
		ref := autoscaler.ComponentPolicyRef(isvc, componentType)
		if ref == nil || ref.Name == "" || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Namespace: isvc.Namespace, Name: ref.Name},
		})
	}
	return requests
}
