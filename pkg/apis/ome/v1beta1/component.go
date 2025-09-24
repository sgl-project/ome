package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComponentExtensionSpec defines the deployment configuration for a given InferenceService component
type ComponentExtensionSpec struct {
	// Minimum number of replicas, defaults to 1 but can be set to 0 to enable scale-to-zero.
	// +optional
	MinReplicas *int `json:"minReplicas,omitempty"`
	// Maximum number of replicas for autoscaling.
	// +optional
	MaxReplicas int `json:"maxReplicas,omitempty"`
	// ScaleTarget specifies the integer target value of the metric type the Autoscaler watches for.
	// concurrency and rps targets are supported by Knative Pod Autoscaler
	// (https://knative.dev/docs/serving/autoscaling/autoscaling-targets/).
	// +optional
	ScaleTarget *int `json:"scaleTarget,omitempty"`
	// ScaleMetric defines the scaling metric type watched by autoscaler
	// possible values are concurrency, rps, cpu, memory. concurrency, rps are supported via
	// Knative Pod Autoscaler(https://knative.dev/docs/serving/autoscaling/autoscaling-metrics).
	// +optional
	ScaleMetric *ScaleMetric `json:"scaleMetric,omitempty"`
	// ContainerConcurrency specifies how many requests can be processed concurrently, this sets the hard limit of the container
	// concurrency(https://knative.dev/docs/serving/autoscaling/concurrency).
	// +optional
	ContainerConcurrency *int64 `json:"containerConcurrency,omitempty"`
	// TimeoutSeconds specifies the number of seconds to wait before timing out a request to the component.
	// +optional
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`
	// CanaryTrafficPercent defines the traffic split percentage between the candidate revision and the last ready revision
	// +optional
	CanaryTrafficPercent *int64 `json:"canaryTrafficPercent,omitempty"`
	// Labels that will be added to the component pod.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations that will be added to the component pod.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// MinAvailiable specifies how many component pods must still be aviliable after the eviction
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
// +kubebuilder:validation:Enum=cpu;memory;concurrency;rps
type ScaleMetric string

const (
	MetricCPU         ScaleMetric = "cpu"
	MetricMemory      ScaleMetric = "memory"
	MetricConcurrency ScaleMetric = "concurrency"
	MetricRPS         ScaleMetric = "rps"
	MetricTPS         ScaleMetric = "tps"
)
