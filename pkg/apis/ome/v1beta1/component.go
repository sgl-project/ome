package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComponentExtensionSpec defines the deployment configuration for a given InferenceService component
type ComponentExtensionSpec struct {
	// Minimum number of replicas, defaults to 1.
	// +optional
	MinReplicas *int `json:"minReplicas,omitempty"`
	// Maximum number of replicas for autoscaling.
	// +optional
	MaxReplicas int `json:"maxReplicas,omitempty"`
	// ScaleTarget specifies the integer target value of the metric type the Autoscaler watches for.
	// +optional
	ScaleTarget *int `json:"scaleTarget,omitempty"`
	// ScaleMetric defines the scaling metric type watched by autoscaler.
	// Possible values are cpu, memory for HPA; KEDA additionally supports custom metrics.
	// +optional
	ScaleMetric *ScaleMetric `json:"scaleMetric,omitempty"`
	// TimeoutSeconds specifies the number of seconds to wait before timing out a request to the component.
	// +optional
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`
	// Labels that will be added to the component pod.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations that will be added to the component pod.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// MinAvailable specifies how many component pods must still be available after the eviction
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
	// MaxUnavailable specifies how many component pods can be unavailable
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The deployment strategy to use to replace existing pods with new ones. Only applicable for raw deployment mode.
	// +optional
	DeploymentStrategy *appsv1.DeploymentStrategy `json:"deploymentStrategy,omitempty"`

	KedaConfig *KedaConfig `json:"kedaConfig,omitempty"`
}

// ScaleMetric enum
// +kubebuilder:validation:Enum=cpu;memory
type ScaleMetric string

const (
	MetricCPU    ScaleMetric = "cpu"
	MetricMemory ScaleMetric = "memory"
	MetricTPS    ScaleMetric = "tps"
)
