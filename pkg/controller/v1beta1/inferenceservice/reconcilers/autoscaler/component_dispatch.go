package autoscaler

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// defaultDispatchMinReplicas is the compatibility default for omitted bounds.
// Dispatch paths without scale-to-zero support also use it as their floor.
const defaultDispatchMinReplicas int32 = 1

// IRDispatchInput is the per-Component projection of everything
// DispatchAutoscaler needs that's not already on the IR object. The
// ISVC-side component dispatchers (engine.go, decoder.go, router.go)
// build this from their local state and the committed IR returned by
// irprojector.EnsureInferenceReplica.
//
// Alpha API. The shape may change without notice.
type IRDispatchInput struct {
	// Client is the controller-runtime client used to apply the HPA
	// or ScaledObject.
	Client client.Client

	// Scheme is the runtime scheme, threaded for parity with the
	// legacy reconciler signatures. DispatchAutoscaler stamps owner-
	// refs directly via the metav1 path so this is reserved for
	// future ctx-aware refactors.
	Scheme *runtime.Scheme

	// IR is the committed InferenceReplica returned by
	// irprojector.EnsureInferenceReplica. Its UID is captured into
	// the OwnerReference so GC cascades the HPA / SO when
	// the IR is deleted. Required — DispatchForIRComponent returns
	// an error if IR is nil, terminating, or has an empty UID.
	IR *v1beta1.InferenceReplica

	// ResolvedAutoscaler is the resolver output (the output of
	// ResolveComponentAutoscaler). nil is treated as Class=none.
	ResolvedAutoscaler *v1beta1.ComponentAutoscaler

	// ComponentExt is the per-Component extension spec; min / max
	// replicas come from here. The dispatch does NOT consult any other
	// field — the Autoscaler block is carried on ResolvedAutoscaler above.
	ComponentExt *v1beta1.ComponentExtensionSpec
}

// RawDispatchInput carries the resolved autoscaler and Deployment identity for
// one RawDeployment Component.
type RawDispatchInput struct {
	Client client.Client
	Scheme *runtime.Scheme

	ISVC          *v1beta1.InferenceService
	ComponentMeta metav1.ObjectMeta

	ResolvedAutoscaler *v1beta1.ComponentAutoscaler
	ComponentExt       *v1beta1.ComponentExtensionSpec
}

// DispatchForRawComponent targets the Component's Deployment and owner-refs
// the generated autoscaler to the live InferenceService.
func DispatchForRawComponent(ctx context.Context, input RawDispatchInput) error {
	if input.ISVC == nil {
		return fmt.Errorf("DispatchForRawComponent: nil ISVC (refusing to stamp orphaned autoscaler)")
	}
	if input.ISVC.Name == "" {
		return fmt.Errorf("DispatchForRawComponent: empty ISVC name")
	}
	if input.ISVC.Namespace == "" {
		return fmt.Errorf("DispatchForRawComponent: empty ISVC namespace")
	}
	if input.ISVC.UID == "" {
		return fmt.Errorf("DispatchForRawComponent: ISVC %s/%s has empty UID (refusing to stamp orphaned autoscaler)", input.ISVC.Namespace, input.ISVC.Name)
	}
	if input.ComponentMeta.Name == "" {
		return fmt.Errorf("DispatchForRawComponent: empty component name")
	}
	if input.ComponentMeta.Namespace == "" {
		return fmt.Errorf("DispatchForRawComponent: empty component namespace")
	}
	if input.ComponentMeta.Namespace != input.ISVC.Namespace {
		return fmt.Errorf("DispatchForRawComponent: namespace mismatch: component %q, ISVC %q", input.ComponentMeta.Namespace, input.ISVC.Namespace)
	}

	minR, maxR, err := rawMinMaxReplicas(input.ComponentExt, input.ResolvedAutoscaler)
	if err != nil {
		return fmt.Errorf("DispatchForRawComponent: invalid replica bounds: %w", err)
	}
	owner := *metav1.NewControllerRef(input.ISVC, v1beta1.SchemeGroupVersion.WithKind("InferenceService"))
	scaleTargetRef := autoscalingv2.CrossVersionObjectReference{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
		Name:       input.ComponentMeta.Name,
	}

	return DispatchAutoscaler(ctx, DispatchParams{
		Client:         input.Client,
		Scheme:         input.Scheme,
		Owner:          owner,
		Namespace:      input.ComponentMeta.Namespace,
		Name:           input.ComponentMeta.Name,
		Labels:         cloneMetadataMap(input.ComponentMeta.Labels),
		Annotations:    cloneMetadataMap(input.ComponentMeta.Annotations),
		ScaleTargetRef: scaleTargetRef,
		Autoscaler:     input.ResolvedAutoscaler,
		MinReplicas:    minR,
		MaxReplicas:    maxR,
	})
}

func rawMinMaxReplicas(c *v1beta1.ComponentExtensionSpec, autoscaler *v1beta1.ComponentAutoscaler) (int32, int32, error) {
	if c == nil {
		return 0, 0, fmt.Errorf("component replica bounds are required")
	}
	// Stored objects can reach reconciliation without admission defaults, so
	// omitted bounds use the same compatibility floor as the generators.
	minR := int(defaultDispatchMinReplicas)
	if c.MinReplicas != nil {
		minR = *c.MinReplicas
	}
	if minR < 0 {
		return 0, 0, fmt.Errorf("minReplicas must be non-negative, got %d", minR)
	}
	maxR := c.MaxReplicas
	if maxR < 0 {
		return 0, 0, fmt.Errorf("maxReplicas must be non-negative, got %d", maxR)
	}
	if minR == 0 && !validTypedRawKEDA(autoscaler) {
		return 0, 0, fmt.Errorf("minReplicas=0 requires typed KEDA with at least one trigger")
	}
	if maxR == 0 {
		maxR = minR
		if maxR == 0 {
			maxR = int(defaultDispatchMinReplicas)
		}
	}
	if minR > maxR {
		return 0, 0, fmt.Errorf("minReplicas must not exceed maxReplicas: %d > %d", minR, maxR)
	}
	return int32(minR), int32(maxR), nil
}

func validTypedRawKEDA(autoscaler *v1beta1.ComponentAutoscaler) bool {
	return autoscaler != nil &&
		autoscaler.Class == v1beta1.AutoscalerKEDA &&
		autoscaler.Keda != nil &&
		len(autoscaler.Keda.Triggers) > 0
}

// DispatchForIRComponent is the convenience wrapper the ISVC-side
// component dispatchers use to translate the per-Component
// IRDispatchInput into a full DispatchParams call. Centralized here
// (rather than copy-pasted into engine.go / decoder.go / router.go)
// so the owner-ref shape, ScaleTargetRef GVK, and min/max bounds
// stay in sync across components.
//
// The HPA + ScaledObject are stamped with:
//
//   - Owner: the live IR with Controller=true. Deleting the IR cascades
//     to both via GC.
//   - ScaleTargetRef: ome.io/v1beta1/InferenceReplica/<ir-name>. The
//     IR's /scale subresource is the canonical scale target for
//     IR-managed Components.
//   - Min/Max replicas: input.ComponentExt.MinReplicas (default 1) and
//     input.ComponentExt.MaxReplicas (clamped up to MinReplicas by the
//     IR projection).
//
// Errors are wrapped with the IR namespace/name so the operator log
// reveals which Component failed dispatch.
//
// Alpha API. The signature may change without notice.
func DispatchForIRComponent(ctx context.Context, input IRDispatchInput) error {
	if input.IR == nil {
		return fmt.Errorf("DispatchForIRComponent: nil IR (refusing to stamp orphaned autoscaler)")
	}
	if input.IR.UID == "" {
		return fmt.Errorf("DispatchForIRComponent: IR %s/%s has empty UID (refusing to stamp orphaned autoscaler)", input.IR.Namespace, input.IR.Name)
	}
	if input.IR.DeletionTimestamp != nil {
		return fmt.Errorf("DispatchForIRComponent: IR %s/%s is terminating", input.IR.Namespace, input.IR.Name)
	}

	minR, maxR := minMaxReplicas(input.ComponentExt)

	owner := metav1.OwnerReference{
		APIVersion: v1beta1.SchemeGroupVersion.String(),
		Kind:       "InferenceReplica",
		Name:       input.IR.Name,
		UID:        input.IR.UID,
		Controller: ptr.To(true),
	}

	scaleTargetRef := autoscalingv2.CrossVersionObjectReference{
		APIVersion: v1beta1.SchemeGroupVersion.String(),
		Kind:       "InferenceReplica",
		Name:       input.IR.Name,
	}

	return DispatchAutoscaler(ctx, DispatchParams{
		Client:         input.Client,
		Scheme:         input.Scheme,
		Owner:          owner,
		Namespace:      input.IR.Namespace,
		Name:           input.IR.Name,
		Labels:         cloneMetadataMap(input.IR.Labels),
		Annotations:    irRunnerAnnotations(input.IR),
		ScaleTargetRef: scaleTargetRef,
		Autoscaler:     input.ResolvedAutoscaler,
		MinReplicas:    minR,
		MaxReplicas:    maxR,
	})
}

// irRunnerAnnotations reads effective Component annotations from the first
// fully-rendered Runner template, which is the canonical metadata source for
// Component-level scaler controls.
func irRunnerAnnotations(ir *v1beta1.InferenceReplica) map[string]string {
	if ir == nil || len(ir.Spec.Runners) == 0 {
		return nil
	}
	return cloneMetadataMap(ir.Spec.Runners[0].Template.Annotations)
}

func cloneMetadataMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// minMaxReplicas projects ComponentExtensionSpec bounds into the int32
// pair the generators expect. MinReplicas defaults to 1 when nil or
// non-positive (matches hpa.calculateMinReplicas + keda.defaultMinReplicas).
// MaxReplicas defaults to 0 when zero — the HPA / KEDA generators clamp
// MaxReplicas up to MinReplicas when MaxReplicas < MinReplicas, so a
// zero MaxReplicas degenerates to a single-replica HPA/SO that just
// reports current load without changing the desired count.
func minMaxReplicas(c *v1beta1.ComponentExtensionSpec) (int32, int32) {
	minR := defaultDispatchMinReplicas
	if c != nil && c.MinReplicas != nil && *c.MinReplicas > 0 {
		minR = int32(*c.MinReplicas)
	}
	var maxR int32
	if c != nil {
		maxR = int32(c.MaxReplicas)
	}
	if maxR < minR {
		maxR = minR
	}
	return minR, maxR
}
