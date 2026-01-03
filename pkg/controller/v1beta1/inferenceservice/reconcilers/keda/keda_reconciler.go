package keda

import (
	"context"
	"fmt"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
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
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

var log = logf.Log.WithName("KEDAReconciler")

// KEDAReconciler reconciles the ScaledObject resource
type KEDAReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	ScaledObject *kedav1.ScaledObject
	componentExt *v1beta1.ComponentExtensionSpec
}

// NewKEDAReconciler creates a new KEDAReconciler
func NewKEDAReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	inferenceServiceSpec *v1beta1.InferenceServiceSpec,
) (*KEDAReconciler, error) {

	scaledObject := createScaledObject(componentMeta, *inferenceServiceSpec)

	return &KEDAReconciler{
		client:       client,
		scheme:       scheme,
		ScaledObject: scaledObject,
		componentExt: &inferenceServiceSpec.Predictor.ComponentExtensionSpec,
	}, nil
}

// createScaledObject creates the ScaledObject resource
func createScaledObject(
	componentMeta metav1.ObjectMeta,
	inferenceServiceSpec v1beta1.InferenceServiceSpec,
) *kedav1.ScaledObject {
	filteredLabels := make(map[string]string)
	for key, value := range componentMeta.Labels {
		// Exclude the label that could prevent opening the edit window through lens
		if key != "k8slens-edit-resource-version" {
			filteredLabels[key] = value
		}
	}
	componentExt := &inferenceServiceSpec.Predictor.ComponentExtensionSpec
	minReplicas := calculateMinReplicas(componentExt)
	maxReplicas := calculateMaxReplicas(componentExt, minReplicas)
	triggers := getScaledObjectTriggers(componentMeta, inferenceServiceSpec)

	return &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.GetScaledObjectName(componentMeta.Name),
			Namespace:   componentMeta.Namespace,
			Labels:      filteredLabels,
			Annotations: componentMeta.Annotations,
		},
		Spec: kedav1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1.ScaleTarget{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       componentMeta.Name,
			},
			MinReplicaCount: &minReplicas,
			MaxReplicaCount: &maxReplicas,
			Triggers:        triggers,
		},
	}
}

// calculateMinReplicas calculates the minimum replicas
func calculateMinReplicas(componentExt *v1beta1.ComponentExtensionSpec) int32 {
	if componentExt.MinReplicas != nil && *componentExt.MinReplicas > 0 {
		return int32(*componentExt.MinReplicas)
	}
	return int32(constants.KedaDefaultMinReplicas)
}

// calculateMaxReplicas calculates the maximum replicas
func calculateMaxReplicas(componentExt *v1beta1.ComponentExtensionSpec, minReplicas int32) int32 {
	if componentExt.MaxReplicas > int(minReplicas) {
		return int32(componentExt.MaxReplicas)
	}
	return minReplicas
}

// getDeploymentName constructs the deployment name based on the componentMeta
func getDeploymentName(metadata metav1.ObjectMeta) string {
	if enabledKueue, ok := metadata.Annotations["kueue-enabled"]; ok && enabledKueue == "true" {
		return fmt.Sprintf("%s-%s", metadata.Name, "new")
	}
	return metadata.Name
}

// getScaledObjectTriggers constructs the triggers for the ScaledObject
func getScaledObjectTriggers(metadata metav1.ObjectMeta, inferenceServiceSpec v1beta1.InferenceServiceSpec) []kedav1.ScaleTriggers {
	kedaConfig := inferenceServiceSpec.KedaConfig
	threshold := getScalingThreshold(metadata, kedaConfig)
	operator := getScalingOperator(metadata, kedaConfig)
	prometheusServerAddress := getPrometheusServerAddress(metadata, kedaConfig)
	prometheusQuery := getPrometheusQuery(metadata, kedaConfig)
	scaleMetric := getScaleMetric(inferenceServiceSpec)

	triggerMetadata := map[string]string{
		"serverAddress": prometheusServerAddress,
		"metricName":    scaleMetric,
		"query":         prometheusQuery,
		"threshold":     threshold,
		"operator":      operator,
	}

	trigger := kedav1.ScaleTriggers{
		Type:     "prometheus",
		Metadata: triggerMetadata,
	}

	// Add authenticationRef if configured
	if kedaConfig != nil && kedaConfig.AuthenticationRef != nil {
		kind := kedaConfig.AuthenticationRef.Kind
		if kind == "" {
			kind = "TriggerAuthentication" // Default kind
		}
		trigger.AuthenticationRef = &kedav1.AuthenticationRef{
			Name: kedaConfig.AuthenticationRef.Name,
			Kind: kind,
		}
		// Add authModes to metadata only when authenticationRef is present
		// as KEDA requires authenticationRef for authModes to be effective
		if kedaConfig.AuthModes != "" {
			trigger.Metadata["authModes"] = kedaConfig.AuthModes
		}
	}

	return []kedav1.ScaleTriggers{trigger}
}

// getScalingThreshold retrieves the scaling threshold
func getScalingThreshold(metadata metav1.ObjectMeta, kedaConfig *v1beta1.KedaConfig) string {
	if value, ok := metadata.Annotations[constants.KedaScalingThreshold]; ok {
		return value
	}
	if kedaConfig != nil && kedaConfig.ScalingThreshold != "" {
		return kedaConfig.ScalingThreshold
	}
	return "10" // Default threshold
}

// getScaleMetric retrieves the scaling metric name
func getScaleMetric(inferenceServiceSpec v1beta1.InferenceServiceSpec) string {
	// Use ScaleMetric from inferenceServiceSpec if available
	if inferenceServiceSpec.Predictor.ScaleMetric != nil && *inferenceServiceSpec.Predictor.ScaleMetric != "" {
		return string(*inferenceServiceSpec.Predictor.ScaleMetric)
	}
	// Default metric
	return string(v1beta1.MetricTPS)
}

// getScalingOperator retrieves the scaling operator
func getScalingOperator(metadata metav1.ObjectMeta, kedaConfig *v1beta1.KedaConfig) string {
	if value, ok := metadata.Annotations[constants.KedaScalingOperator]; ok {
		return value
	}
	if kedaConfig != nil && kedaConfig.ScalingOperator != "" {
		return kedaConfig.ScalingOperator
	}
	return "LessThanOrEqual" // Default operator
}

// getPrometheusServerAddress retrieves the Prometheus server address
func getPrometheusServerAddress(metadata metav1.ObjectMeta, kedaConfig *v1beta1.KedaConfig) string {
	if value, ok := metadata.Annotations[constants.KedaPrometheusServerAddress]; ok {
		return value
	}
	if kedaConfig != nil && kedaConfig.PromServerAddress != "" {
		return kedaConfig.PromServerAddress
	}
	return "http://prometheus-operated.monitoring.svc.cluster.local:9090" // Default address
}

// getPrometheusQuery constructs the Prometheus query
func getPrometheusQuery(metadata metav1.ObjectMeta, kedaConfig *v1beta1.KedaConfig) string {
	if value, ok := metadata.Annotations[constants.KedaPrometheusQuery]; ok {
		return value
	}
	if kedaConfig != nil && kedaConfig.CustomPromQuery != "" {
		return fmt.Sprintf(kedaConfig.CustomPromQuery, metadata.Name)
	}
	// Default VLLM Prometheus query
	// Scale up condition: Low token throughput during high request load
	throughputThreshold := 10   // Token throughput in TPS
	requestRateThreshold := 0.5 // Request throughput in RPM
	return fmt.Sprintf(
		`sum(
            avg_over_time(vllm:avg_generation_throughput_toks_per_s{ome_io_inferenceservice="%s"}[5m]) < bool %d
        )
        *
        sum(
            rate(vllm:request_success_total{ome_io_inferenceservice="%s"}[1m]) > bool %.2f
        )`,
		metadata.Name,
		throughputThreshold,
		metadata.Name,
		requestRateThreshold,
	)
}

// checkScaledObjectExist checks if the ScaledObject exists and determines the action
func (r *KEDAReconciler) checkScaledObjectExist() (constants.CheckResultType, *kedav1.ScaledObject, error) {
	existingScaledObject := &kedav1.ScaledObject{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.ScaledObject.Namespace,
		Name:      r.ScaledObject.Name,
	}, existingScaledObject)

	if err != nil {
		if apierr.IsNotFound(err) {
			if shouldCreateScaledObject(r.ScaledObject) {
				return constants.CheckResultCreate, nil, nil
			}
			return constants.CheckResultSkipped, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	if semanticScaledObjectEquals(r.ScaledObject, existingScaledObject) {
		return constants.CheckResultExisted, existingScaledObject, nil
	}
	if shouldDeleteScaledObject(r.ScaledObject) {
		return constants.CheckResultDelete, existingScaledObject, nil
	}
	return constants.CheckResultUpdate, existingScaledObject, nil
}

// semanticScaledObjectEquals checks if the desired and existing ScaledObjects are equal
func semanticScaledObjectEquals(desired, existing *kedav1.ScaledObject) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	existingAutoscalerClass := existing.Annotations[constants.AutoscalerClass]

	autoscalerClassChanged := desiredAutoscalerClass != existingAutoscalerClass
	return equality.Semantic.DeepEqual(desired.Spec, existing.Spec) && !autoscalerClassChanged
}

// shouldDeleteScaledObject determines if the ScaledObject should be deleted
func shouldDeleteScaledObject(desired *kedav1.ScaledObject) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassExternal
}

// shouldCreateScaledObject determines if the ScaledObject should be created
func shouldCreateScaledObject(desired *kedav1.ScaledObject) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return desiredAutoscalerClass == "" || constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassKEDA
}

// Reconcile reconciles the ScaledObject resource
func (r *KEDAReconciler) Reconcile() (runtime.Object, error) {
	checkResult, existingScaledObject, err := r.checkScaledObjectExist()
	log.Info("Reconciling ScaledObject", "namespace", r.ScaledObject.Namespace, "name", r.ScaledObject.Name, "checkResult", checkResult.String())
	if err != nil {
		return nil, err
	}

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.ScaledObject)
	case constants.CheckResultUpdate:
		// Use the resourceVersion from the existing ScaledObject
		r.ScaledObject.ResourceVersion = existingScaledObject.ResourceVersion

		opErr = r.client.Update(context.TODO(), r.ScaledObject)
	case constants.CheckResultDelete:
		opErr = r.client.Delete(context.TODO(), r.ScaledObject)
	default:
		return existingScaledObject, nil
	}

	if opErr != nil {
		log.Error(opErr, "Failed to reconcile ScaledObject", "namespace", r.ScaledObject.Namespace, "name", r.ScaledObject.Name)
		return nil, opErr
	}

	return r.ScaledObject, nil
}

// SetControllerReferences sets the owner reference for the ScaledObject
func (r *KEDAReconciler) SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(owner, r.ScaledObject, scheme)
}
