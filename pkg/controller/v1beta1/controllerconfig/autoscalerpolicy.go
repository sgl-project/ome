package controllerconfig

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// AutoscalerPolicyConfigName is the inferenceservice-config ConfigMap key
// holding the AutoscalerPolicy operator configuration.
const AutoscalerPolicyConfigName = "autoscalerPolicy"

// Operational fallbacks for the preflight tunables. Provider addresses have
// deliberately NO in-code default — the chart/GitOps supplies them; an
// unbound provider name is a render error, never a baked-in endpoint.
const (
	DefaultPolicyMemberGetTimeoutSeconds int32 = 5
	DefaultPolicySkewDeadlineSeconds     int32 = 900
)

// +kubebuilder:object:generate=false
// AutoscalerPolicyMetricProvider is one cluster-local binding of a logical
// metric-provider name (referenced by AutoscalerPolicy triggers via
// providerRef) to an endpoint and optional credentials.
type AutoscalerPolicyMetricProvider struct {
	// ServerAddress is injected as the rendered trigger's serverAddress.
	ServerAddress string `json:"serverAddress"`

	// AuthSecretRef optionally names a Secret key holding a bearer token.
	// The controller materializes a KEDA TriggerAuthentication in each
	// consumer namespace and wires it on rendered triggers; the token itself
	// never appears in policies, rendered blocks, status, or digests.
	// The Secret is read from the consumer namespace.
	// +optional
	AuthSecretRef *v1.SecretKeySelector `json:"authSecretRef,omitempty"`
}

// +kubebuilder:object:generate=false
// AutoscalerPolicyPreflightConfig tunes the control-plane placement
// preflight for policy-referencing InferenceServices.
type AutoscalerPolicyPreflightConfig struct {
	// MemberGetTimeoutSeconds bounds each per-candidate live policy GET; a
	// timed-out member is ineligible for that placement, so one hung
	// apiserver cannot stall other candidates.
	// +optional
	MemberGetTimeoutSeconds int32 `json:"memberGetTimeoutSeconds,omitempty"`

	// SkewDeadlineSeconds bounds how long a home may hold a derived
	// policy-referencing ISVC without reporting a resolved digest before the
	// control plane flags it (AutoscalerPolicyReady=False, ResolveTimeout).
	// +optional
	SkewDeadlineSeconds int32 `json:"skewDeadlineSeconds,omitempty"`
}

// +kubebuilder:object:generate=false
// AutoscalerPolicyConfig is the operator-level configuration for the
// AutoscalerPolicy feature, loaded from the inferenceservice-config
// ConfigMap (key "autoscalerPolicy").
type AutoscalerPolicyConfig struct {
	// MetricProviders maps logical provider names to cluster-local bindings.
	// +optional
	MetricProviders map[string]AutoscalerPolicyMetricProvider `json:"metricProviders,omitempty"`

	// +optional
	Preflight AutoscalerPolicyPreflightConfig `json:"preflight,omitempty"`
}

// NewAutoscalerPolicyConfig loads the block once (startup paths).
func NewAutoscalerPolicyConfig(clientset kubernetes.Interface) (*AutoscalerPolicyConfig, error) {
	configMap, err := getInferenceServiceConfigMap(clientset)
	if err != nil {
		return nil, err
	}
	return parseAutoscalerPolicyConfig(configMap)
}

// NewAutoscalerPolicyConfigCached loads the block through the TTL config
// cache, so provider-binding edits take effect without a controller restart.
func NewAutoscalerPolicyConfigCached(cache *ConfigCache, clientset kubernetes.Interface) (*AutoscalerPolicyConfig, error) {
	configMap, err := cache.get(clientset)
	if err != nil {
		return nil, err
	}
	return parseAutoscalerPolicyConfig(configMap)
}

func parseAutoscalerPolicyConfig(configMap *v1.ConfigMap) (*AutoscalerPolicyConfig, error) {
	cfg := &AutoscalerPolicyConfig{}
	if err := getComponentConfig(AutoscalerPolicyConfigName, configMap, cfg); err != nil {
		return nil, fmt.Errorf("unable to parse autoscalerPolicy config json: %w", err)
	}
	for name, provider := range cfg.MetricProviders {
		if provider.ServerAddress == "" {
			return nil, fmt.Errorf("autoscalerPolicy config: metric provider %q has no serverAddress", name)
		}
	}
	if cfg.Preflight.MemberGetTimeoutSeconds <= 0 {
		cfg.Preflight.MemberGetTimeoutSeconds = DefaultPolicyMemberGetTimeoutSeconds
	}
	if cfg.Preflight.SkewDeadlineSeconds <= 0 {
		cfg.Preflight.SkewDeadlineSeconds = DefaultPolicySkewDeadlineSeconds
	}
	return cfg, nil
}
