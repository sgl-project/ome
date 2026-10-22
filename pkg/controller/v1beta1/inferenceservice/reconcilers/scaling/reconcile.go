package scaling

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	"sigs.k8s.io/ome/pkg/placement/protocol"
	"sigs.k8s.io/ome/pkg/validation"
)

// Targets are the follower counts the coordinator resolved for one pass. They
// are read before the projectors run, so each follower's projection writes
// its count and the canary partition sized from it in one write.
type Targets struct {
	policy     *v1beta1.ScalingPolicy
	source     string
	components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec
	modes      map[v1beta1.ComponentType]constants.DeploymentModeType
	replicas   map[v1beta1.ComponentType]int32
}

// ResolveTargets reads the anchor and the followers and resolves each
// follower's count. No count is resolved while the policy cannot be applied
// (an invalid policy, a missing anchor, a follower with a competing scaler),
// and the projector then keeps the count each follower carries.
func ResolveTargets(ctx context.Context, reader client.Reader, isvc *v1beta1.InferenceService,
	policy *v1beta1.ScalingPolicy, source string,
	components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec,
	modes map[v1beta1.ComponentType]constants.DeploymentModeType) (*Targets, error) {
	targets := &Targets{policy: policy, source: source, components: components, modes: modes,
		replicas: map[v1beta1.ComponentType]int32{}}
	if policy == nil {
		return targets, nil
	}
	seen, err := observe(ctx, reader, isvc, policy, components, modes)
	if err != nil {
		return nil, err
	}
	if seen.hold == nil {
		for _, follower := range seen.followers {
			targets.replicas[follower.component] = follower.count.Target
		}
	}
	return targets, nil
}

// Replicas is the count the projector writes on a follower this pass, nil
// when the coordinator resolved none for the Component.
func (t *Targets) Replicas(component v1beta1.ComponentType) *int32 {
	if t == nil {
		return nil
	}
	if n, ok := t.replicas[component]; ok {
		return ptr.To(n)
	}
	return nil
}

// condition is the Ready condition of a pass that resolves no count.
type condition struct {
	ready           bool
	reason, message string
}

// follower is one follower's resolved count beside the count its replica
// carries.
type follower struct {
	component v1beta1.ComponentType
	ratio     resource.Quantity
	count     Count
	// current is nil while the follower's replica is not projected.
	current *int32
}

// observation is the coordinator's reading of the anchor and the followers at
// one moment of a pass.
type observation struct {
	anchor        v1beta1.ComponentType
	anchorDesired int32
	hold          *condition
	// conflict names the follower with a competing scaler; it holds every follower.
	conflict  string
	followers []follower
	skipped   []string
	missing   []string
}

func observe(ctx context.Context, reader client.Reader, isvc *v1beta1.InferenceService,
	policy *v1beta1.ScalingPolicy, components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec,
	modes map[v1beta1.ComponentType]constants.DeploymentModeType) (*observation, error) {
	seen := &observation{}
	hold := func(ready bool, reason, message string) (*observation, error) {
		seen.hold = &condition{ready: ready, reason: reason, message: message}
		return seen, nil
	}
	if policy.Mode == "" || policy.Mode == v1beta1.ScalingIndependent {
		return hold(true, "Independent", "components scale independently")
	}
	if policy.Mode != v1beta1.ScalingProportional || policy.Proportional == nil {
		return hold(false, "ModeNotImplemented", "scaling policy cannot be coordinated")
	}
	if err := validation.ValidateProportionalShape(policy, "spec.scalingPolicy"); err != nil {
		return hold(false, "InvalidPolicy", err.Error())
	}
	seen.anchor = policy.Proportional.Anchor
	if issue := componentIssue(seen.anchor, components, modes); issue != "" {
		return hold(false, "InvalidPolicy", "anchor "+issue)
	}
	keys := make([]v1beta1.ComponentType, 0, len(policy.Proportional.Ratios))
	for component := range policy.Proportional.Ratios {
		keys = append(keys, component)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	eligible := keys[:0]
	for _, component := range keys {
		if issue := componentIssue(component, components, modes); issue != "" {
			seen.skipped = append(seen.skipped, "follower "+issue)
			continue
		}
		eligible = append(eligible, component)
	}
	keys = eligible
	get := func(component v1beta1.ComponentType) (*v1beta1.InferenceReplica, error) {
		ir := &v1beta1.InferenceReplica{}
		key := client.ObjectKey{Namespace: isvc.Namespace, Name: irprojector.InferenceReplicaName(isvc.Name, component)}
		if err := reader.Get(ctx, key, ir); err != nil {
			return nil, err
		}
		if ir.Spec.ParentRef == nil || ir.Spec.ParentRef.Name != isvc.Name || ir.Spec.Component != component {
			return nil, fmt.Errorf("replica %s is not projected by this InferenceService", key)
		}
		return ir, nil
	}
	anchorIR, err := get(seen.anchor)
	if apierrors.IsNotFound(err) || (err == nil && anchorIR.Spec.Replicas == nil) {
		return hold(false, "AnchorUnavailable", "anchor desired count is unavailable; follower counts are held")
	}
	if err != nil {
		return nil, err
	}
	seen.anchorDesired = *anchorIR.Spec.Replicas

	// A competing writer on any follower holds every follower.
	current := make(map[v1beta1.ComponentType]*int32, len(keys))
	for _, component := range keys {
		ir, err := get(component)
		if apierrors.IsNotFound(err) {
			seen.missing = append(seen.missing, string(component))
			continue
		}
		if err != nil {
			return nil, err
		}
		if ir.Spec.Autoscaler != nil && ir.Spec.Autoscaler.Class != v1beta1.AutoscalerNone {
			seen.conflict = followerConflictMessage(isvc, component, ir.Spec.Autoscaler.Class)
			return hold(false, "FollowerAutoscalerConflict", seen.conflict)
		}
		current[component] = ptr.To(ptr.Deref(ir.Spec.Replicas, 0))
	}
	for _, component := range keys {
		ratio := policy.Proportional.Ratios[component]
		if ratio.MilliValue() <= 0 || ratio.MilliValue() > 1_000_000 || ratio.Sign() <= 0 {
			return hold(false, "InvalidPolicy", "follower ratio is outside the supported range")
		}
		count, err := Target(seen.anchorDesired, ratio.MilliValue(), FollowerBounds(components[component]))
		if err != nil {
			return hold(false, "InvalidPolicy", err.Error())
		}
		seen.followers = append(seen.followers, follower{component: component, ratio: ratio,
			count: count, current: current[component]})
	}
	return seen, nil
}

// Reconcile publishes the coordinator's status once the projectors have run:
// the anchor's count, each follower's target beside the count its replica
// carries, the Ready condition and the warning events. It writes no replica:
// a follower's count travels in its own projection, so a follower that does
// not yet carry its target is reported pending and written by the next pass.
// The caller flushes the status alongside the rest of InferenceService status.
func Reconcile(ctx context.Context, reader client.Reader, recorder record.EventRecorder,
	isvc *v1beta1.InferenceService, targets *Targets) error {
	if targets == nil || targets.policy == nil {
		isvc.Status.ScalingPolicy = nil
		return nil
	}
	policy := targets.policy
	previous := isvc.Status.ScalingPolicy
	status := &v1beta1.ScalingPolicyStatus{Mode: policy.Mode, Source: targets.source}
	if previous != nil {
		status.Conditions = append(status.Conditions, previous.Conditions...)
	}
	isvc.Status.ScalingPolicy = status
	setCondition := func(ready bool, reason, message string) {
		state := metav1.ConditionFalse
		if ready {
			state = metav1.ConditionTrue
		}
		apiMeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type: "Ready", Status: state, Reason: reason, Message: message,
			ObservedGeneration: isvc.Generation,
		})
	}
	seen, err := observe(ctx, reader, isvc, policy, targets.components, targets.modes)
	if err != nil {
		return err
	}
	status.Anchor = seen.anchor
	status.AnchorDesired = seen.anchorDesired
	if seen.hold != nil {
		setCondition(seen.hold.ready, seen.hold.reason, seen.hold.message)
		if seen.conflict != "" && recorder != nil && !samePreviousConflict(previous, seen.conflict) {
			recorder.Event(isvc, corev1.EventTypeWarning, "FollowerAutoscalerConflict", seen.conflict)
		}
		return nil
	}
	// notes carry what the Ready condition says beyond the counts: which
	// bound holds a follower, and which follower nothing caps.
	var notes, pending []string
	anchorHasMaximum := targets.components[seen.anchor].MaxReplicas > 0
	placed := placementSetsBounds(isvc)
	for _, f := range seen.followers {
		if f.current == nil {
			continue
		}
		target, clamped := f.count.Target, f.count.Bound != ""
		entry := v1beta1.FollowerScalingStatus{Component: f.component, Ratio: f.ratio.DeepCopy(),
			Target: target, Current: *f.current, Clamped: clamped}
		previouslyClamped := false
		if previous != nil && previous.Mode == status.Mode && previous.Anchor == status.Anchor {
			for _, old := range previous.Followers {
				if old.Component == f.component && old.Clamped && old.Target == target {
					previouslyClamped = true
				}
			}
		}
		if clamped {
			note := fmt.Sprintf("%s target is bounded to %d Instances by its %s (the ratio asks for %d)", f.component, target, f.count.Bound, f.count.Wanted)
			notes = append(notes, note)
			if !previouslyClamped && recorder != nil && !placed {
				recorder.Event(isvc, corev1.EventTypeWarning, "ScalingTargetClamped", note)
			}
		}
		if FollowerBounds(targets.components[f.component]).Max == nil && !anchorHasMaximum {
			note := fmt.Sprintf("%s follows %s with no maxReplicas on either side; set maxReplicas on one of them to bound the %s count", f.component, seen.anchor, f.component)
			notes = append(notes, note)
			if recorder != nil && !previousMessageContains(previous, note) {
				recorder.Event(isvc, corev1.EventTypeWarning, "ScalingFollowerUnbounded", note)
			}
		}
		if *f.current != target {
			pending = append(pending, fmt.Sprintf("%s carries %d Instances, target %d", f.component, *f.current, target))
		}
		status.Followers = append(status.Followers, entry)
	}
	switch {
	case len(seen.skipped) > 0:
		message := strings.Join(seen.skipped, "; ")
		if len(seen.missing) > 0 {
			message += "; follower InferenceReplica not yet projected: " + strings.Join(seen.missing, ", ")
		}
		setCondition(false, "InvalidPolicy", message)
	case len(seen.missing) > 0:
		setCondition(false, "FollowerUnavailable", "follower InferenceReplica not yet projected: "+strings.Join(seen.missing, ", "))
	case len(pending) > 0:
		message := "follower desired counts are written by the next projection: " + strings.Join(pending, "; ")
		setCondition(false, "FollowerPending", strings.Join(append([]string{message}, notes...), "; "))
	default:
		setCondition(true, "Applied", strings.Join(append([]string{"follower desired counts match the resolved policy"}, notes...), "; "))
	}
	return nil
}

// placementSetsBounds reports a member copy of a Split placement home. The hub
// writes every scaled component's bounds there from the plan's shares, so a
// follower clamped to them follows placement, which status records without a
// warning. Single and All homes carry the source's own bounds, like a local
// service, and keep the warning.
func placementSetsBounds(isvc *v1beta1.InferenceService) bool {
	policy, err := protocol.FromDerived(isvc)
	return err == nil && policy != nil && len(policy.ReplicaFloors) == 0
}

func samePreviousConflict(previous *v1beta1.ScalingPolicyStatus, message string) bool {
	if previous == nil {
		return false
	}
	condition := apiMeta.FindStatusCondition(previous.Conditions, "Ready")
	return condition != nil && condition.Status == metav1.ConditionFalse &&
		condition.Reason == "FollowerAutoscalerConflict" && condition.Message == message
}

// previousMessageContains reports whether the last Ready condition already
// carried this note, so a standing note warns once.
func previousMessageContains(previous *v1beta1.ScalingPolicyStatus, note string) bool {
	if previous == nil {
		return false
	}
	condition := apiMeta.FindStatusCondition(previous.Conditions, "Ready")
	return condition != nil && strings.Contains(condition.Message, note)
}

func followerConflictMessage(isvc *v1beta1.InferenceService, component v1beta1.ComponentType, class v1beta1.AutoscalerClass) string {
	origin := "an unresolved source"
	if entry, ok := isvc.Status.Components[component]; ok && entry.Autoscaler != nil {
		switch autoscaler.SpecSource(entry.Autoscaler.SpecSource) {
		case autoscaler.SpecSourceISVC:
			origin = "an inline autoscaler block"
		case autoscaler.SpecSourcePolicy:
			if ref := autoscaler.ComponentPolicyRef(isvc, component); ref != nil {
				origin = fmt.Sprintf("autoscalerPolicyRef %q", ref.Name)
			} else {
				origin = "an AutoscalerPolicy reference"
			}
		case autoscaler.SpecSourceRuntime:
			origin = "the serving runtime"
		default:
			origin = fmt.Sprintf("source %q", entry.Autoscaler.SpecSource)
		}
	} else if ref := autoscaler.ComponentPolicyRef(isvc, component); ref != nil {
		origin = fmt.Sprintf("autoscalerPolicyRef %q", ref.Name)
	}
	return fmt.Sprintf("%s follower has a competing %s autoscaler from %s; follower counts are held", component, class, origin)
}
