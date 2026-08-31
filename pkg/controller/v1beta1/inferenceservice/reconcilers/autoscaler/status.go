package autoscaler

import (
	"context"
	"fmt"
	"regexp"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

// WriteAutoscalerStatus computes the per-Component ComponentAutoscalerStatus
// + ScaleTargetRef for the ISVC status writer. Inputs:
//
//   - resolvedAutoscaler: the output of the Component's deployment-mode
//     resolver. nil is treated as Class=none — surfaces ManagedBy=none.
//   - specSource: the SpecSource returned alongside resolvedAutoscaler so the
//     status mirror echoes the inheritance-chain decision back to the operator.
//   - scaleTargetRef: the canonical scale target for this Component. Always
//     published, even when Class=none, so operators have a stable address
//     they can scale manually (kubectl scale ome.io/v1beta1/InferenceReplica
//     name --replicas=N).
//
// Returns the (autoscalerStatus, scaleTargetRef) pair the caller writes onto
// status.components.<comp>.{autoscaler,scaleTargetRef}. The scaleTargetRef
// return value is a deep copy of the input so the caller can mutate freely.
//
// Three-way ManagedBy mapping (NOTE: none and external are status-field
// twins):
//
//   - resolvedAutoscaler == nil           → ManagedBy=none
//   - resolvedAutoscaler.Class == hpa     → ManagedBy=ome (try mirror HPA)
//   - resolvedAutoscaler.Class == keda    → ManagedBy=ome (try mirror SO)
//   - resolvedAutoscaler.Class == external→ ManagedBy=external (no mirror)
//   - resolvedAutoscaler.Class == none    → ManagedBy=none (no mirror)
//   - unrecognized Class                  → ManagedBy=none (defensive)
//
// When ManagedBy=ome the writer Gets the live HPA / SO by name
// (= <objectName>) and mirrors Conditions, CurrentReplicas, DesiredReplicas,
// LastScaleTime verbatim. NotFound is silently tolerated — the scaler may
// have been created in the same reconcile pass and not yet reported, so
// counters stay at zero and Conditions stays empty.
//
// objectName is the (Namespace, Name) the dispatch uses for the HPA /
// ScaledObject. For OMENative-managed Components this is the IR name
// (=<isvc>-<component>); for RawDeployment Components it's the rendered
// component metadata Name. The KEDA generator translates this name through
// utils.GetScaledObjectName under the hood so the lookup applies the same
// prefix.
//
// cli may be nil — in that case mirroring is skipped and the result contains
// the static fields (Class / ManagedBy / SpecSource) only. Useful for unit
// tests that exercise the mapping logic without a live client.
//
// prev is the previously-written ComponentAutoscalerStatus (nil on first
// reconcile). It is used only to preserve KEDA condition LastTransitionTime
// across reconciles (KEDA conditions carry no timestamp), keeping the
// mirrored status byte-stable so the writer's DeepEqual diff doesn't storm
// status updates.
//
// Alpha API. The signature may change without notice.
func WriteAutoscalerStatus(
	ctx context.Context,
	cli client.Client,
	namespace string,
	objectName string,
	resolvedAutoscaler *v1beta1.ComponentAutoscaler,
	specSource SpecSource,
	scaleTargetRef v1beta1.ScaleTargetRef,
	prev *v1beta1.ComponentAutoscalerStatus,
) (*v1beta1.ComponentAutoscalerStatus, *v1beta1.ScaleTargetRef, error) {
	class := resolvedClass(resolvedAutoscaler)
	managedBy := managedByForClass(class)

	status := &v1beta1.ComponentAutoscalerStatus{
		Class:      class,
		ManagedBy:  managedBy,
		SpecSource: string(specSource),
	}

	if managedBy == v1beta1.AutoscalerManagedByOME && cli != nil {
		switch class {
		case v1beta1.AutoscalerHPA:
			if err := mirrorHPAStatus(ctx, cli, namespace, objectName, status); err != nil {
				return nil, nil, fmt.Errorf("mirror HPA status (ns=%s, name=%s): %w", namespace, objectName, err)
			}
		case v1beta1.AutoscalerKEDA:
			if err := mirrorScaledObjectStatus(ctx, cli, namespace, objectName, status, prevConditions(prev)); err != nil {
				return nil, nil, fmt.Errorf("mirror ScaledObject status (ns=%s, name=%s): %w", namespace, objectName, err)
			}
		}
	}

	// Always return a copy of scaleTargetRef so the caller can mutate freely
	// without affecting the input. Empty targets are returned as nil so
	// downstream marshalling doesn't emit `{"apiVersion":"","kind":"","name":""}`
	// onto the status — operators reading status.components.<c>.scaleTargetRef
	// expect "field absent" when no target is published.
	var stRef *v1beta1.ScaleTargetRef
	if scaleTargetRef.APIVersion != "" || scaleTargetRef.Kind != "" || scaleTargetRef.Name != "" {
		ref := scaleTargetRef
		stRef = &ref
	}

	return status, stRef, nil
}

// prevConditions returns the condition slice from a previously-written
// ComponentAutoscalerStatus, nil-safe (nil status → nil conditions).
func prevConditions(prev *v1beta1.ComponentAutoscalerStatus) []metav1.Condition {
	if prev == nil {
		return nil
	}
	return prev.Conditions
}

// resolvedClass extracts Class from the resolved ComponentAutoscaler. A nil
// input is treated as Class=none so the caller-side decision "do nothing,
// no autoscaler" surfaces as a stable ManagedBy=none on status.
func resolvedClass(a *v1beta1.ComponentAutoscaler) v1beta1.AutoscalerClass {
	if a == nil {
		return v1beta1.AutoscalerNone
	}
	return a.Class
}

// managedByForClass maps the resolved AutoscalerClass to the operator-visible
// ManagedBy field: hpa + keda => ome (OME owns + reconciles); external =>
// external (operator owns); none / unknown => none.
func managedByForClass(class v1beta1.AutoscalerClass) v1beta1.AutoscalerManagedBy {
	switch class {
	case v1beta1.AutoscalerHPA, v1beta1.AutoscalerKEDA:
		return v1beta1.AutoscalerManagedByOME
	case v1beta1.AutoscalerExternal:
		return v1beta1.AutoscalerManagedByExternal
	default:
		// Defensive fallback: AutoscalerNone, "", and any unrecognized
		// value all surface as ManagedBy=none. We refuse to claim ownership
		// for a Class value we don't know how to dispatch.
		return v1beta1.AutoscalerManagedByNone
	}
}

// mirrorHPAStatus reads the live HorizontalPodAutoscaler at (namespace, name)
// and stamps its Conditions / CurrentReplicas / DesiredReplicas /
// LastScaleTime onto status. NotFound is swallowed — the dispatch may have
// created the HPA in the same reconcile and the cache may not have caught
// up, or no dispatch has wired this Component. Counters stay zero in those
// cases and operators can observe "scaler just spun up" by reading status
// again on the next reconcile pass.
func mirrorHPAStatus(ctx context.Context, cli client.Client, namespace, name string, status *v1beta1.ComponentAutoscalerStatus) error {
	obj := &autoscalingv2.HorizontalPodAutoscaler{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	status.CurrentReplicas = obj.Status.CurrentReplicas
	status.DesiredReplicas = obj.Status.DesiredReplicas
	if obj.Status.LastScaleTime != nil {
		t := *obj.Status.LastScaleTime
		status.LastScaleTime = &t
	}
	status.Conditions = hpaConditionsToMetav1(obj.Status.Conditions)
	return nil
}

// mirrorScaledObjectStatus reads the live KEDA ScaledObject (looked up by
// utils.GetScaledObjectName(name)) and stamps Conditions / LastScaleTime
// onto status. CurrentReplicas / DesiredReplicas come from the HPA the
// ScaledObject derives — KEDA writes them onto its own embedded HPA, not
// the ScaledObject status — so we additionally Get that HPA (named by
// SO.Status.HpaName, falling back to the SO name) and read counters from
// there. Both lookups tolerate NotFound (transient or unwired state).
//
// NotRegistered errors on the SO Get are swallowed too: if the cluster
// hasn't installed KEDA but somebody resolved class=keda upstream
// (shouldn't happen at steady state because dispatch validates earlier),
// we still want the parent reconcile to make forward progress.
func mirrorScaledObjectStatus(ctx context.Context, cli client.Client, namespace, name string, status *v1beta1.ComponentAutoscalerStatus, prevConds []metav1.Condition) error {
	soName := utils.GetScaledObjectName(name)
	obj := &kedav1.ScaledObject{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: soName}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		// KEDA is registered unconditionally at manager startup; this
		// branch is defensive against a rolling-uninstall race.
		if runtime.IsNotRegisteredError(err) {
			return nil
		}
		return err
	}

	status.Conditions = kedaConditionsToMetav1(obj.Status.Conditions, prevConds)
	if obj.Status.LastActiveTime != nil {
		t := *obj.Status.LastActiveTime
		status.LastScaleTime = &t
	}

	// CurrentReplicas / DesiredReplicas live on the derived HPA. SO.Status
	// .HpaName is the canonical handle for the derived HPA; if empty (older
	// KEDA versions stamp it lazily) fall back to a name-based lookup.
	hpaName := obj.Status.HpaName
	if hpaName == "" {
		hpaName = soName
	}
	hpaObj := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: hpaName}, hpaObj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	status.CurrentReplicas = hpaObj.Status.CurrentReplicas
	status.DesiredReplicas = hpaObj.Status.DesiredReplicas
	return nil
}

// podNameRefRe matches the volatile "of Pod <name>" tail the HPA metrics
// controller appends to FailedGetResourceMetric / FailedGetPodsMetric
// condition messages, e.g.
//
//	"...missing request for cpu in container ome-container of Pod sglang-decoder-7d9f8-abcde"
//
// The trailing pod name rotates every time the metrics client samples a
// different replica, so mirroring the raw message onto the ISVC status makes
// the status genuinely differ on every reconcile pass — which re-triggers the
// ISVC's own For() watch and drives a status-write storm. We collapse the pod
// reference to a stable placeholder so the mirrored message carries the same
// signal (which metric / container failed) without the rotating identifier.
var podNameRefRe = regexp.MustCompile(`(?i) of pod \S+`)

// normalizeAutoscalerMessage strips volatile per-pod runtime detail from a
// scaler condition message so the mirrored ISVC status stays byte-stable
// across reconciles when nothing meaningful changed. Currently this collapses
// the rotating "of Pod <name>" tail to "of Pod <pod>"; the stable prefix
// (which metric/container failed, the reason) is preserved verbatim.
func normalizeAutoscalerMessage(msg string) string {
	return podNameRefRe.ReplaceAllString(msg, " of Pod <pod>")
}

// hpaConditionsToMetav1 translates the HPA condition type (which uses
// autoscalingv2.HorizontalPodAutoscalerCondition) into metav1.Condition so
// the ISVC status surface can stay agnostic of the source scaler. Type +
// Status + Reason + LastTransitionTime pass through 1:1; Message is run
// through normalizeAutoscalerMessage to drop the rotating per-pod tail that
// would otherwise churn the ISVC status every reconcile.
func hpaConditionsToMetav1(in []autoscalingv2.HorizontalPodAutoscalerCondition) []metav1.Condition {
	if len(in) == 0 {
		return nil
	}
	out := make([]metav1.Condition, 0, len(in))
	for i := range in {
		c := in[i]
		out = append(out, metav1.Condition{
			Type:               string(c.Type),
			Status:             metav1.ConditionStatus(c.Status),
			Reason:             c.Reason,
			Message:            normalizeAutoscalerMessage(c.Message),
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return out
}

// kedaConditionsToMetav1 translates kedav1.Conditions (a slice of
// kedav1.Condition) into metav1.Condition for the ISVC status surface.
//
// KEDA's Condition struct carries neither a LastTransitionTime nor,
// for some conditions (e.g. Paused), a Reason. Both are REQUIRED by the
// ISVC CRD's condition schema (lastTransitionTime required, reason min 1
// char), so copying KEDA conditions verbatim makes every status update
// fail apiserver validation — which wedges the ISVC reconcile in a
// permanent error loop (the status never persists). We therefore
// synthesize both:
//
//   - Reason: fall back to the condition Type when KEDA leaves it empty
//     (Type is always a valid single-word reason: Ready/Active/Fallback/
//     Paused), or "Unknown" as a last resort.
//   - LastTransitionTime: preserve the timestamp from the prior status
//     when the observed Status is unchanged, else stamp now. Preserving
//     it keeps the mirrored status byte-stable across reconciles (the
//     status writer diffs with equality.Semantic.DeepEqual), so a fresh
//     Now() every pass doesn't storm status updates.
//
// prev is the previously-written condition set (nil on first observation).
func kedaConditionsToMetav1(in kedav1.Conditions, prev []metav1.Condition) []metav1.Condition {
	if len(in) == 0 {
		return nil
	}
	out := make([]metav1.Condition, 0, len(in))
	for i := range in {
		c := in[i]
		cond := metav1.Condition{
			Type:    string(c.Type),
			Status:  c.Status,
			Reason:  kedaConditionReason(c),
			Message: normalizeAutoscalerMessage(c.Message),
		}
		cond.LastTransitionTime = kedaConditionTransitionTime(cond, prev)
		out = append(out, cond)
	}
	return out
}

// kedaConditionReason returns a CRD-valid (non-empty) reason for a KEDA
// condition. KEDA leaves Reason empty for some conditions (notably
// Paused), but the ISVC CRD requires at least one character; we fall back
// to the condition Type (always a valid single word) and finally to a
// generic constant.
func kedaConditionReason(c kedav1.Condition) string {
	if c.Reason != "" {
		return c.Reason
	}
	if t := string(c.Type); t != "" {
		return t
	}
	return "Unknown"
}

// kedaConditionTransitionTime returns a stable LastTransitionTime for a
// synthesized KEDA condition: the prior recorded time when a same-typed
// condition existed with the same Status (no real transition), otherwise
// now (first observation or an actual Status flip). This keeps the
// mirrored status byte-stable across reconciles.
func kedaConditionTransitionTime(cond metav1.Condition, prev []metav1.Condition) metav1.Time {
	for i := range prev {
		p := prev[i]
		if p.Type != cond.Type {
			continue
		}
		if p.Status == cond.Status && !p.LastTransitionTime.IsZero() {
			return p.LastTransitionTime
		}
		break
	}
	return metav1.Now()
}
