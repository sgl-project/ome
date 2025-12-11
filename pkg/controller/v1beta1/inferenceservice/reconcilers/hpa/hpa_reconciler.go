package hpa

import (
	"context"
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

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

var log = logf.Log.WithName("HPAReconciler")

// HPAReconciler reconciles the HorizontalPodAutoscaler resource
type HPAReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	HPA          *autoscalingv2.HorizontalPodAutoscaler
	componentExt *v1beta1.ComponentExtensionSpec
}

func NewHPAReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec) *HPAReconciler {

	return &HPAReconciler{
		client:       client,
		scheme:       scheme,
		HPA:          createHPA(componentMeta, componentExt),
		componentExt: componentExt,
	}
}

func createHPA(componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec) *autoscalingv2.HorizontalPodAutoscaler {

	minReplicas := calculateMinReplicas(componentExt)
	maxReplicas := calculateMaxReplicas(componentExt, minReplicas)
	metrics := getHPAMetrics(componentMeta, componentExt)

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: componentMeta,
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       componentMeta.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior:    &autoscalingv2.HorizontalPodAutoscalerBehavior{},
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

func getDeploymentName(metadata metav1.ObjectMeta) string {
	return metadata.Name
}

func getHPAMetrics(metadata metav1.ObjectMeta, componentExt *v1beta1.ComponentExtensionSpec) []autoscalingv2.MetricSpec {
	utilization := getTargetUtilization(metadata, componentExt)
	resourceName := getResourceName(componentExt)

	metricTarget := autoscalingv2.MetricTarget{
		Type:               "Utilization",
		AverageUtilization: &utilization,
	}

	return []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name:   resourceName,
				Target: metricTarget,
			},
		},
	}
}

func getTargetUtilization(metadata metav1.ObjectMeta, componentExt *v1beta1.ComponentExtensionSpec) int32 {
	if value, ok := metadata.Annotations[constants.TargetUtilizationPercentage]; ok {
		utilization, _ := strconv.Atoi(value)
		return int32(utilization) // #nosec G109
	}
	if componentExt.ScaleTarget != nil {
		return int32(*componentExt.ScaleTarget)
	}
	return constants.DefaultCPUUtilization
}

func getResourceName(componentExt *v1beta1.ComponentExtensionSpec) corev1.ResourceName {
	if componentExt.ScaleMetric != nil {
		return corev1.ResourceName(*componentExt.ScaleMetric)
	}
	return corev1.ResourceCPU
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

	if semanticHPAEquals(r.HPA, existingHPA) {
		return constants.CheckResultExisted, existingHPA, nil
	}
	if shouldDeleteHPA(r.HPA) {
		return constants.CheckResultDelete, existingHPA, nil
	}
	return constants.CheckResultUpdate, existingHPA, nil
}

func semanticHPAEquals(desired, existing *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	existingAutoscalerClass := existing.Annotations[constants.AutoscalerClass]

	autoscalerClassChanged := desiredAutoscalerClass != existingAutoscalerClass
	return equality.Semantic.DeepEqual(desired.Spec, existing.Spec) && !autoscalerClassChanged
}

func shouldDeleteHPA(desired *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassExternal
}

func shouldCreateHPA(desired *autoscalingv2.HorizontalPodAutoscaler) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return desiredAutoscalerClass == "" || constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassHPA
}

func (r *HPAReconciler) Reconcile() (runtime.Object, error) {
	checkResult, existingHPA, err := r.checkHPAExist()
	log.Info("Reconciling HPA", "namespace", r.HPA.Namespace, "name", r.HPA.Name, "checkResult", checkResult.String())
	if err != nil {
		return nil, err
	}

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.HPA)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.HPA)
	case constants.CheckResultDelete:
		opErr = r.client.Delete(context.TODO(), r.HPA)
	default:
		return existingHPA, nil
	}

	if opErr != nil {
		log.Error(opErr, "Failed to reconcile HPA", "namespace", r.HPA.Namespace, "name", r.HPA.Name)
		return nil, opErr
	}

	return r.HPA, nil
}

func (r *HPAReconciler) SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(owner, r.HPA, scheme)
}
