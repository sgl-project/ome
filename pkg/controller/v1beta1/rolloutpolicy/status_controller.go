// Package rolloutpolicy registers the RolloutPolicy status controller: the
// shared policy status controller with the RolloutPolicy rules for
// references, the digest, readiness and the InUse wording. A status write
// can never move traffic: the InferenceService reconcile applies a policy
// when it opens a rollout.
package rolloutpolicy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/policystatus"
	policycore "sigs.k8s.io/ome/pkg/rolloutpolicy"
	"sigs.k8s.io/ome/pkg/validation"
)

// +kubebuilder:rbac:groups=ome.io,resources=rolloutpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=rolloutpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices,verbs=get;list;watch

// PolicyRefIndexField is the InferenceService field index keyed by the
// distinct RolloutPolicy names the service's rollout groups reference. The
// InferenceService controller registers PolicyRefIndexer under this key;
// this controller only queries it.
const PolicyRefIndexField = "spec.rolloutPolicyRefs"

// PolicyRefIndexer is the extractor for PolicyRefIndexField: the distinct
// policy names across spec.rollout.groups[].policyRef.
func PolicyRefIndexer(obj client.Object) []string {
	return policystatus.ReferencedPolicies[*v1beta1.RolloutPolicy](kind{}, obj)
}

// SetupWithManager registers the status controller. It owns
// RolloutPolicy.status and nothing else; the metric-provider bindings it
// consults come from the shared metricProviders operator configuration.
func SetupWithManager(mgr ctrl.Manager, clientSet kubernetes.Interface, cache *controllerconfig.ConfigCache) error {
	return policystatus.SetupWithManager[*v1beta1.RolloutPolicy](mgr, "rolloutpolicy-status", kind{},
		policystatus.ConfigSource{Clientset: clientSet, Cache: cache})
}

// kind describes the RolloutPolicy kind to the shared status controller.
type kind struct{}

func (kind) NewPolicy() *v1beta1.RolloutPolicy { return &v1beta1.RolloutPolicy{} }

func (kind) ReferenceIndexField() string { return PolicyRefIndexField }

// References lists the policy each rollout group references, one entry per
// group, so a service that names one policy from two groups lists it twice.
func (kind) References(isvc *v1beta1.InferenceService) []string {
	var names []string
	groups := isvc.Spec.GetRolloutGroups()
	for i := range groups {
		ref := groups[i].PolicyRef
		if ref == nil || ref.Name == "" {
			continue
		}
		names = append(names, ref.Name)
	}
	return names
}

func (kind) Digest(policy *v1beta1.RolloutPolicy) (string, error) {
	return policycore.PortableDigest(&policy.Spec)
}

func (kind) Status(policy *v1beta1.RolloutPolicy) policystatus.StatusFields {
	return policystatus.StatusFields{
		ObservedGeneration: &policy.Status.ObservedGeneration,
		PortableDigest:     &policy.Status.PortableDigest,
		Attached:           &policy.Status.AttachedGroups,
		Conditions:         &policy.Status.Conditions,
	}
}

// Ready derives Ready from the body validation an inline progression faces
// plus the policy-only portability rules. The one cluster-local input,
// whether the body's providerRef is bound here, can only soften the reason,
// never the status: clusters in one fleet bind different provider sets, so
// an unbound provider is a warning (Ready stays True, reason
// ProviderUnbound), and a rollout that starts on this cluster waits before
// its first step until the binding exists. The boolean reports whether the
// configuration was read. The error return is transient only (a
// configuration fetch); invalid content is a condition, never an error.
func (kind) Ready(policy *v1beta1.RolloutPolicy, config policystatus.ConfigSource, digestErr error) (metav1.Condition, bool, error) {
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

	bound, err := controllerconfig.NewMetricProvidersConfigCached(config.Cache, config.Clientset)
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

func (kind) InUse(generation int64, attached int32) metav1.Condition {
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
