// Package policystatus is the status controller the policy kinds share.
// It publishes what a policy's status reports: the generation it observed,
// the portable digest of the spec, how many components or rollout groups
// reference the policy, and the Ready and InUse conditions. It never changes
// a workload. The InferenceService reconcile applies a policy, so a status
// write cannot start a scaler or move traffic.
package policystatus

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// Kind is what the controller needs to know about one policy kind. Each
// kind's package implements it once and registers the controller through
// SetupWithManager.
type Kind[P client.Object] interface {
	// NewPolicy returns an empty policy to read into.
	NewPolicy() P

	// ReferenceIndexField names the InferenceService field index whose
	// values are the names of this kind's policies that a service
	// references. The InferenceService controller registers the index;
	// this controller only queries it.
	ReferenceIndexField() string

	// References lists the names of this kind's policies that one
	// InferenceService references, one entry per component or rollout
	// group that carries a reference, so a name repeats when two of them
	// share it. A reference without a name is left out.
	References(isvc *v1beta1.InferenceService) []string

	// Digest computes the portable digest of the policy's spec.
	Digest(policy P) (string, error)

	// Ready derives the Ready condition. The boolean reports whether the
	// rule read the operator configuration. An error is a transient read
	// failure and retries the pass; invalid content is a condition, never
	// an error.
	Ready(policy P, config ConfigSource, digestErr error) (condition metav1.Condition, consulted bool, err error)

	// InUse derives the InUse condition from the number of components or
	// rollout groups that reference the policy.
	InUse(generation int64, attached int32) metav1.Condition

	// Status returns the fields of the policy's status that the controller
	// writes.
	Status(policy P) StatusFields
}

// StatusFields points at the fields of a policy's status that the controller
// writes, so one write serves every kind's status type.
type StatusFields struct {
	ObservedGeneration *int64
	PortableDigest     *string
	// Attached counts the components or rollout groups that reference the
	// policy.
	Attached   *int32
	Conditions *[]metav1.Condition
}

// ConfigSource is where a readiness rule reads the operator configuration:
// the clientset fetches the ConfigMap and the cache keeps it for its
// lifetime.
type ConfigSource struct {
	Clientset kubernetes.Interface
	Cache     *controllerconfig.ConfigCache
}

// Reconciler publishes the status of one policy kind.
type Reconciler[P client.Object] struct {
	client.Client
	Kind   Kind[P]
	Config ConfigSource
}

// SetupWithManager registers the status controller of one kind under the
// given name. A policy event reconciles that policy. An InferenceService
// event reconciles every policy of the kind that the service references, on
// the old object as well as the new one, so a reference added or removed
// updates the count without polling.
func SetupWithManager[P client.Object](mgr ctrl.Manager, name string, kind Kind[P], config ConfigSource) error {
	r := &Reconciler[P]{Client: mgr.GetClient(), Kind: kind, Config: config}
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(kind.NewPolicy(), builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1beta1.InferenceService{}, handler.EnqueueRequestsFromMapFunc(r.policiesReferencedBy)).
		Complete(r)
}

func (r *Reconciler[P]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := r.Kind.NewPolicy()
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	attached, err := r.attachedReferences(ctx, policy)
	if err != nil {
		return ctrl.Result{}, err
	}

	digest, digestErr := r.Kind.Digest(policy)
	ready, consultedConfig, err := r.Kind.Ready(policy, r.Config, digestErr)
	if err != nil {
		return ctrl.Result{}, err
	}

	// The operator configuration sends this controller no watch event, and
	// the generation predicate drops resyncs, so a pass that read it checks
	// again when the cached copy expires. Without that, a binding edit would
	// not change Ready until the policy itself is rewritten. A cache
	// lifetime of zero means live reads and no periodic check.
	result := ctrl.Result{}
	if consultedConfig {
		if ttl := r.Config.Cache.TTL(); ttl > 0 {
			result.RequeueAfter = ttl
		}
	}

	before := policy.DeepCopyObject()
	status := r.Kind.Status(policy)
	*status.ObservedGeneration = policy.GetGeneration()
	*status.PortableDigest = digest
	*status.Attached = attached
	apimeta.SetStatusCondition(status.Conditions, ready)
	apimeta.SetStatusCondition(status.Conditions, r.Kind.InUse(policy.GetGeneration(), attached))

	if equality.Semantic.DeepEqual(before, policy) {
		return result, nil
	}
	if updateErr := r.Status().Update(ctx, policy); updateErr != nil {
		if apierrors.IsConflict(updateErr) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, updateErr
	}
	return result, nil
}

// attachedReferences counts the components or rollout groups in the policy's
// namespace that reference it, not the InferenceServices. One service that
// references the policy from two places counts twice, so the count matches
// what removing one reference releases.
func (r *Reconciler[P]) attachedReferences(ctx context.Context, policy P) (int32, error) {
	consumers := &v1beta1.InferenceServiceList{}
	if err := r.List(ctx, consumers,
		client.InNamespace(policy.GetNamespace()),
		client.MatchingFields{r.Kind.ReferenceIndexField(): policy.GetName()}); err != nil {
		return 0, fmt.Errorf("list the InferenceServices that reference policy %s/%s: %w", policy.GetNamespace(), policy.GetName(), err)
	}
	var count int32
	for i := range consumers.Items {
		for _, name := range r.Kind.References(&consumers.Items[i]) {
			if name == policy.GetName() {
				count++
			}
		}
	}
	return count, nil
}

// policiesReferencedBy maps an InferenceService event to the same-namespace
// policies the service references, each once.
func (r *Reconciler[P]) policiesReferencedBy(_ context.Context, obj client.Object) []reconcile.Request {
	var requests []reconcile.Request
	for _, name := range ReferencedPolicies(r.Kind, obj) {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: name},
		})
	}
	return requests
}

// ReferencedPolicies returns the distinct names of the kind's policies that
// obj references, in first-reference order, or nil when obj is not an
// InferenceService. It is the extractor for the kind's field index.
func ReferencedPolicies[P client.Object](kind Kind[P], obj client.Object) []string {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, name := range kind.References(isvc) {
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
