package v1beta1

// KedaConfig stores the configuration settings for KEDA autoscaling within the InferenceService.
// It includes fields like the Prometheus server address, custom query, scaling threshold, and operator.
type KedaConfig struct {
	// EnableKeda determines whether KEDA autoscaling is enabled for the InferenceService.
	// - true: KEDA will manage the autoscaling based on the provided configuration.
	// - false: KEDA will not be used, and autoscaling will rely on other mechanisms (e.g., HPA).
	EnableKeda bool `json:"enableKeda,omitempty"`

	// PromServerAddress specifies the address of the Prometheus server that KEDA will query
	// to retrieve metrics for autoscaling decisions. This should be a fully qualified URL,
	// including the protocol and port number.
	//
	// Example:
	//   http://prometheus-operated.monitoring.svc.cluster.local:9090
	PromServerAddress string `json:"promServerAddress,omitempty"`

	// CustomPromQuery defines a custom Prometheus query that KEDA will execute to evaluate
	// the desired metric for scaling. This query should return a single numerical value that
	// represents the metric to be monitored.
	//
	// Example:
	//   avg_over_time(http_requests_total{service="llama"}[5m])
	CustomPromQuery string `json:"customPromQuery,omitempty"`

	// ScalingThreshold sets the numerical threshold against which the result of the Prometheus
	// query will be compared. Depending on the ScalingOperator, this threshold determines
	// when to scale the number of replicas up or down.
	//
	// Example:
	//   "10" - The Autoscaler will compare the metric value to 10.
	ScalingThreshold string `json:"scalingThreshold,omitempty"`

	// ScalingOperator specifies the comparison operator used by KEDA to decide whether to scale
	// the Deployment. Common operators include:
	// - "GreaterThanOrEqual": Scale up when the metric is >= ScalingThreshold.
	// - "LessThanOrEqual": Scale down when the metric is <= ScalingThreshold.
	//
	// This operator defines the condition under which scaling actions are triggered based on
	// the evaluated metric.
	//
	// Example:
	//   "GreaterThanOrEqual"
	ScalingOperator string `json:"scalingOperator,omitempty"`
}
