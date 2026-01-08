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

	// AuthenticationRef references a TriggerAuthentication or ClusterTriggerAuthentication
	// resource that contains the authentication configuration for the Prometheus server.
	// This is required when the Prometheus server requires authentication (e.g., Grafana Cloud).
	//
	// Example:
	//   authenticationRef:
	//     name: grafana-cloud-auth
	//     kind: TriggerAuthentication
	// +optional
	AuthenticationRef *ScalerAuthenticationRef `json:"authenticationRef,omitempty"`

	// AuthModes specifies the authentication mode(s) for the Prometheus scaler.
	// Common values include: "basic", "tls", "bearer", "custom".
	// Multiple modes can be specified comma-separated (e.g., "tls,basic").
	//
	// Example:
	//   "basic" - Use basic authentication with username/password from TriggerAuthentication
	// +optional
	AuthModes string `json:"authModes,omitempty"`
}

// ScalerAuthenticationRef points to a KEDA TriggerAuthentication or ClusterTriggerAuthentication resource
// that contains the credentials for authenticating with the scaler's target (e.g., Prometheus server).
type ScalerAuthenticationRef struct {
	// Name of the TriggerAuthentication or ClusterTriggerAuthentication resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Kind of the authentication resource being referenced.
	// Valid values are "TriggerAuthentication" (namespace-scoped) or "ClusterTriggerAuthentication" (cluster-scoped).
	// +kubebuilder:default="TriggerAuthentication"
	// +kubebuilder:validation:Enum=TriggerAuthentication;ClusterTriggerAuthentication
	// +optional
	Kind string `json:"kind,omitempty"`
}
