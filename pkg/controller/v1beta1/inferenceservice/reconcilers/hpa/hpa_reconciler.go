package hpa

import (
	"context"
	"fmt"
	"strconv"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/scalermetadata"
)

var log = logf.Log.WithName("HPAReconciler")

// HPAReconciler reconciles the HorizontalPodAutoscaler resource
type HPAReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	HPA    *autoscalingv2.HorizontalPodAutoscaler
}

// NewHPAReconciler builds an HPAReconciler whose generated HPA targets the
// caller-supplied Deployment. Used by the Raw Deployment dispatch path. The
// HPA bounds (MinReplicas / MaxReplicas) and metric block come from the
// ComponentExtensionSpec:
//
//   - Bounds: ComponentExtensionSpec.MinReplicas / MaxReplicas (the
//     canonical replica bound location).
//   - Metric: Autoscaler.HPA.Metrics when set, else CPU=80% default.
//     ome.io/targetUtilizationPercentage annotation overrides the
//     default CPU target.
//
// See NewHPAReconcilerForTarget for the IR-managed dispatch path.
func NewHPAReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec) *HPAReconciler {

	return &HPAReconciler{
		client: client,
		scheme: scheme,
		HPA:    createHPAFromComponentExt(componentMeta, componentExt),
	}
}

// NewHPAReconcilerForTarget builds an HPAReconciler whose generated HPA
// targets the caller-supplied scaleTargetRef (typically an InferenceReplica
// for the IR-managed path). hpaSpec carries the resolved HPAAutoscaler
// block (resolver output); nil / empty Metrics expands to the default of a
// single CPU=80% Resource metric.
//
// It is the entry point for the IR-managed dispatch and stays free of
// any ComponentExtensionSpec concerns so the two callers are completely
// decoupled.
func NewHPAReconcilerForTarget(
	client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	scaleTargetRef autoscalingv2.CrossVersionObjectReference,
	hpaSpec *v1beta1.HPAAutoscaler,
	minReplicas int32,
	maxReplicas int32,
) *HPAReconciler {
	return &HPAReconciler{
		client: client,
		scheme: scheme,
		HPA:    createHPA(componentMeta, scaleTargetRef, hpaSpec, minReplicas, maxReplicas),
	}
}

// createHPA builds the desired HorizontalPodAutoscaler from explicit inputs.
//
// scaleTargetRef is forwarded verbatim to the HPA spec — callers are expected
// to supply `{apps/v1, Deployment, <name>}` for Raw Deployment dispatch and
// `{ome.io/v1beta1, InferenceReplica, <ir-name>}` for the IR-managed path.
//
// hpaSpec / minReplicas / maxReplicas come from the resolver or the Raw
// Deployment shim (createHPAFromComponentExt). When hpaSpec is nil or
// hpaSpec.Metrics is empty the generator emits a single CPU=80% Resource
// metric (the default).
func createHPA(
	componentMeta metav1.ObjectMeta,
	scaleTargetRef autoscalingv2.CrossVersionObjectReference,
	hpaSpec *v1beta1.HPAAutoscaler,
	minReplicas int32,
	maxReplicas int32,
) *autoscalingv2.HorizontalPodAutoscaler {
	metrics := defaultHPAMetrics()
	var behavior *autoscalingv2.HorizontalPodAutoscalerBehavior
	if hpaSpec != nil {
		if len(hpaSpec.Metrics) > 0 {
			metrics = hpaSpec.Metrics
		}
		if hpaSpec.Behavior != nil {
			behavior = hpaSpec.Behavior
		}
	}
	if behavior == nil {
		behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{}
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: componentMeta,
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: scaleTargetRef,
			MinReplicas:    &minReplicas,
			MaxReplicas:    maxReplicas,
			Metrics:        metrics,
			Behavior:       behavior,
		},
	}
}

// createHPAFromComponentExt is the bridge from the Raw Deployment
// (componentMeta, componentExt) caller to the parameterized createHPA.
// Reads Autoscaler.HPA when present (the canonical authoring location)
// and falls back to the CPU=80% default (overridden by the
// ome.io/targetUtilizationPercentage annotation if set) for ISVCs that
// don't declare a per-Component autoscaler block at all.
func createHPAFromComponentExt(
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
) *autoscalingv2.HorizontalPodAutoscaler {
	scaleTargetRef := autoscalingv2.CrossVersionObjectReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       componentMeta.Name,
	}
	minReplicas := calculateMinReplicas(componentExt)
	maxReplicas := calculateMaxReplicas(componentExt, minReplicas)

	var hpaSpec *v1beta1.HPAAutoscaler
	if componentExt.Autoscaler != nil && componentExt.Autoscaler.HPA != nil {
		hpaSpec = componentExt.Autoscaler.HPA
	} else if utilization, ok := utilizationFromAnnotation(componentMeta); ok {
		// Preserve the ome.io/targetUtilizationPercentage annotation
		// override for ISVCs that don't declare an Autoscaler.HPA block.
		// The annotation is the only knob left for the CPU target now
		// that the legacy ScaleTarget field is gone.
		hpaSpec = &v1beta1.HPAAutoscaler{
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               "Utilization",
							AverageUtilization: &utilization,
						},
					},
				},
			},
		}
	}

	return createHPA(componentMeta, scaleTargetRef, hpaSpec, minReplicas, maxReplicas)
}

// defaultHPAMetrics returns the default metric list: a single
// Resource{cpu, 80%} entry. Used when the resolver / shim hands back an
// HPAAutoscaler with no explicit Metrics.
func defaultHPAMetrics() []autoscalingv2.MetricSpec {
	utilization := constants.DefaultCPUUtilization
	return []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               "Utilization",
					AverageUtilization: &utilization,
				},
			},
		},
	}
}

func calculateMinReplicas(componentExt *v1beta1.ComponentExtensionSpec) int32 {
	if componentExt.MinReplicas == nil || *componentExt.MinReplicas < constants.DefaultMinReplicas {
		return int32(constants.DefaultMinReplicas)
	}
	return int32(*componentExt.MinReplicas)
}

func calculateMaxReplicas(componentExt *v1beta1.ComponentExtensionSpec, minReplicas int32) int32 {
	maxReplicas := int32(componentExt.MaxReplicas)
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	return maxReplicas
}

// utilizationFromAnnotation reads the legacy
// ome.io/targetUtilizationPercentage annotation as an int32; returns
// (utilization, true) when present and parseable, else (0, false).
func utilizationFromAnnotation(metadata metav1.ObjectMeta) (int32, bool) {
	value, ok := metadata.Annotations[constants.TargetUtilizationPercentage]
	if !ok {
		return 0, false
	}
	utilization, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(utilization), true
}

func (r *HPAReconciler) checkHPAExist() (constants.CheckResultType, *autoscalingv2.HorizontalPodAutoscaler, error) {
	existingHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.HPA.Namespace,
		Name:      r.HPA.Name,
	}, existingHPA)

	if err != nil {
		if apierr.IsNotFound(err) {
			if shouldCreateHPA(r.HPA) {
				return constants.CheckResultCreate, nil, nil
			}
			return constants.CheckResultSkipped, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	if err := validateHPAControllerOwnership(r.HPA, existingHPA); err != nil {
		return constants.CheckResultUnknown, existingHPA, err
	}
	if existingHPA.DeletionTimestamp != nil {
		return constants.CheckResultUnknown, existingHPA, fmt.Errorf("HorizontalPodAutoscaler %s/%s is terminating", existingHPA.Namespace, existingHPA.Name)
	}

	if semanticHPAEquals(r.HPA, existingHPA) {
		return constants.CheckResultExisted, existingHPA, nil
	}
	if shouldDeleteHPA(r.HPA) {
		return constants.CheckResultDelete, existingHPA, nil
	}
	return constants.CheckResultUpdate, existingHPA, nil
}

func validateHPAControllerOwnership(desired, existing *autoscalingv2.HorizontalPodAutoscaler) error {
	expected := metav1.GetControllerOf(desired)
	if expected == nil {
		return nil
	}
	actual := metav1.GetControllerOf(existing)
	if actual != nil && actual.UID == expected.UID {
		return nil
	}
	if actual == nil {
		return fmt.Errorf("HorizontalPodAutoscaler %s/%s is not controlled by expected owner %s %s (UID %q): object has no controller owner", existing.Namespace, existing.Name, expected.Kind, expected.Name, expected.UID)
	}
	return fmt.Errorf("HorizontalPodAutoscaler %s/%s is not controlled by expected owner %s %s (UID %q): controller is %s %s (UID %q)", existing.Namespace, existing.Name, expected.Kind, expected.Name, expected.UID, actual.Kind, actual.Name, actual.UID)
}

// semanticHPAEquals reports whether the live HPA already matches the desired
// metadata, controller owner, and spec fields OME manages. The apiserver
// defaults spec.behavior (scaleUp/scaleDown policies) whenever the desired HPA
// omits it, so the live behavior is always populated while the generated one
// is nil or an empty stub.
// Comparing the full spec would therefore diff on every reconcile and trigger a
// perpetual no-op Update. When OME does not manage a behavior block (desired
// behavior is nil or zero-valued) we treat it as "don't care" and exclude it
// from the comparison rather than re-introducing the server defaults in code.
func semanticHPAEquals(desired, existing *autoscalingv2.HorizontalPodAutoscaler) bool {
	if desired.Annotations[constants.AutoscalerClass] != existing.Annotations[constants.AutoscalerClass] {
		return false
	}
	if !scalermetadata.Contains(existing.Labels, existing.Annotations, desired.Labels, desired.Annotations) {
		return false
	}

	desiredSpec := desired.Spec
	existingSpec := existing.Spec
	if behaviorUnmanaged(desiredSpec.Behavior) {
		desiredSpec.Behavior = nil
		existingSpec.Behavior = nil
	}

	return equality.Semantic.DeepEqual(desiredSpec, existingSpec) &&
		equality.Semantic.DeepEqual(metav1.GetControllerOf(desired), metav1.GetControllerOf(existing))
}

func hpaForUpdate(desired, existing *autoscalingv2.HorizontalPodAutoscaler) *autoscalingv2.HorizontalPodAutoscaler {
	updated := existing.DeepCopy()
	updated.Spec = desired.Spec
	updated.Labels, updated.Annotations = scalermetadata.Merge(
		existing.Labels,
		existing.Annotations,
		desired.Labels,
		desired.Annotations,
	)
	if _, present := desired.Annotations[constants.AutoscalerClass]; !present {
		delete(updated.Annotations, constants.AutoscalerClass)
	}
	updated.OwnerReferences = mergeHPAOwnerReferences(existing.OwnerReferences, desired.OwnerReferences)
	return updated
}

func mergeHPAOwnerReferences(existing, desired []metav1.OwnerReference) []metav1.OwnerReference {
	merged := make([]metav1.OwnerReference, 0, len(existing)+len(desired))
	for _, ref := range existing {
		if ref.Controller == nil || !*ref.Controller {
			merged = append(merged, ref)
		}
	}
	for _, ref := range desired {
		if ref.Controller != nil && *ref.Controller {
			merged = append(merged, ref)
		}
	}
	return merged
}

// behaviorUnmanaged reports whether OME is not asserting any HPA behavior:
// either no block at all, or an empty stub (the legacy default emitted by
// createHPA when the caller supplies none). In both cases the live,
// server-defaulted behavior must be ignored in the equality check.
func behaviorUnmanaged(b *autoscalingv2.HorizontalPodAutoscalerBehavior) bool {
	if b == nil {
		return true
	}
	return equality.Semantic.DeepEqual(b, &autoscalingv2.HorizontalPodAutoscalerBehavior{})
}

func shouldDeleteHPA(desired *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassExternal
}

func shouldCreateHPA(desired *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return desiredAutoscalerClass == "" || constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassHPA
}

func (r *HPAReconciler) Reconcile() error {
	checkResult, existingHPA, err := r.checkHPAExist()
	log.V(1).Info("Reconciling HPA", "namespace", r.HPA.Namespace, "name", r.HPA.Name, "checkResult", checkResult.String())
	if err != nil {
		return err
	}

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.HPA)
	case constants.CheckResultUpdate:
		r.HPA = hpaForUpdate(r.HPA, existingHPA)
		opErr = r.client.Update(context.TODO(), r.HPA)
	case constants.CheckResultDelete:
		opErr = r.client.Delete(context.TODO(), r.HPA)
	default:
		return nil
	}

	if opErr != nil {
		log.Error(opErr, "Failed to reconcile HPA", "namespace", r.HPA.Namespace, "name", r.HPA.Name)
		return opErr
	}

	return nil
}

func (r *HPAReconciler) SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(owner, r.HPA, scheme)
}
