package hpa

import (
	"strconv"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ownedscaler"
)

// HPAReconciler brings a Component's HorizontalPodAutoscaler to the desired
// shape.
type HPAReconciler = ownedscaler.Reconciler[*autoscalingv2.HorizontalPodAutoscaler]

// kind is the HorizontalPodAutoscaler as an owned scaler. Its one rule of its
// own: a behavior block OME does not set is left out of the comparison,
// because the apiserver fills one in on every live HPA.
var kind = ownedscaler.Kind[*autoscalingv2.HorizontalPodAutoscaler]{
	Name:      "HorizontalPodAutoscaler",
	Class:     constants.AutoscalerClassHPA,
	Log:       logf.Log.WithName("HPAReconciler").V(1),
	New:       func() *autoscalingv2.HorizontalPodAutoscaler { return &autoscalingv2.HorizontalPodAutoscaler{} },
	SpecEqual: specEqual,
	CopySpec:  func(from, to *autoscalingv2.HorizontalPodAutoscaler) { to.Spec = from.Spec },
}

// NewHPAReconcilerForTarget builds an HPAReconciler whose HPA targets
// scaleTargetRef, a Deployment or an InferenceReplica. hpaSpec is the
// resolved HPAAutoscaler block; nil, or empty Metrics, means one CPU metric at
// the default utilization.
func NewHPAReconcilerForTarget(
	client client.Client,
	componentMeta metav1.ObjectMeta,
	scaleTargetRef autoscalingv2.CrossVersionObjectReference,
	hpaSpec *v1beta1.HPAAutoscaler,
	minReplicas int32,
	maxReplicas int32,
) *HPAReconciler {
	return ownedscaler.New(client, kind, createHPA(componentMeta, scaleTargetRef, hpaSpec, minReplicas, maxReplicas))
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

// specEqual compares the HPA specs, leaving the behavior block out when OME
// does not set one, because the apiserver fills it in on the live object and
// a full comparison would update the HPA on every pass.
func specEqual(desired, existing *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredSpec := desired.Spec
	existingSpec := existing.Spec
	if behaviorUnmanaged(desiredSpec.Behavior) {
		desiredSpec.Behavior = nil
		existingSpec.Behavior = nil
	}
	return equality.Semantic.DeepEqual(desiredSpec, existingSpec)
}

// behaviorUnmanaged reports whether OME sets no HPA behavior: no block, or
// the empty block createHPA emits when the caller supplies none.
func behaviorUnmanaged(b *autoscalingv2.HorizontalPodAutoscalerBehavior) bool {
	if b == nil {
		return true
	}
	return equality.Semantic.DeepEqual(b, &autoscalingv2.HorizontalPodAutoscalerBehavior{})
}
