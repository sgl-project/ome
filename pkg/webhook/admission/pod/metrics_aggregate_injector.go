package pod

import (
	"encoding/json"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils"
)

const (
	defaultKserveContainerPrometheusPort = "8080"
	MetricsAggregatorConfigMapKeyName    = "metricsAggregator"
)

type MetricsAggregator struct {
	EnableMetricAggregation  string `json:"enableMetricAggregation"`
	EnablePrometheusScraping string `json:"enablePrometheusScraping"`
}

func newMetricsAggregator(configMap *v1.ConfigMap) (*MetricsAggregator, error) { //nolint:unparam
	ma := &MetricsAggregator{}

	if maConfigVal, ok := configMap.Data[MetricsAggregatorConfigMapKeyName]; ok {
		err := json.Unmarshal([]byte(maConfigVal), &ma)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall %v json string due to %w ", MetricsAggregatorConfigMapKeyName, err)
		}
	}

	return ma, nil
}

// setMetricAggregationEnvVarsAndPorts tells queue-proxy where to scrape
// the ome-container metrics and which port to expose the aggregated
// metrics on. Scrape port/path are inherited from the ServingRuntime
// YAML; fall back to the kserve container defaults so a transformer
// using the Python SDK still works without explicit annotations.
func setMetricAggregationEnvVarsAndPorts(pod *v1.Pod) {
	for i, container := range pod.Spec.Containers {
		if container.Name != "queue-proxy" {
			continue
		}
		omeContainerPromPort := defaultKserveContainerPrometheusPort
		if port, ok := pod.ObjectMeta.Annotations[constants.ContainerPrometheusPortKey]; ok {
			omeContainerPromPort = port
		}
		omeContainerPromPath := constants.DefaultPrometheusPath
		if path, ok := pod.ObjectMeta.Annotations[constants.ContainerPrometheusPathKey]; ok {
			omeContainerPromPath = path
		}
		// Update-if-exists like the port injection below: the webhook is
		// registered with reinvocationPolicy=IfNeeded, so this may run more
		// than once on the same pod and must not stack duplicates.
		pod.Spec.Containers[i].Env = utils.MergeEnvs(pod.Spec.Containers[i].Env, []v1.EnvVar{
			{Name: constants.ContainerPrometheusMetricsPortEnvVarKey, Value: omeContainerPromPort},
			{Name: constants.ContainerPrometheusMetricsPathEnvVarKey, Value: omeContainerPromPath},
			{Name: constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey, Value: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)},
		})
		pod.Spec.Containers[i].Ports = utils.AppendPortIfNotExists(pod.Spec.Containers[i].Ports, v1.ContainerPort{
			Name:          constants.AggregateMetricsPortName,
			ContainerPort: int32(constants.QueueProxyAggregatePrometheusMetricsPort),
			Protocol:      "TCP",
		})
	}
}

// InjectMetricsAggregator wires queue-proxy aggregate metrics + the
// pod-level prometheus.io scrape annotations. Defaults come from the
// inferenceservice-config ConfigMap; per-pod annotations override.
func (ma *MetricsAggregator) InjectMetricsAggregator(pod *v1.Pod) error {
	enableMetricAggregation, ok := pod.ObjectMeta.Annotations[constants.EnableMetricAggregation]
	if !ok {
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = make(map[string]string)
		}
		pod.ObjectMeta.Annotations[constants.EnableMetricAggregation] = ma.EnableMetricAggregation
		enableMetricAggregation = ma.EnableMetricAggregation
	}
	if enableMetricAggregation == "true" {
		setMetricAggregationEnvVarsAndPorts(pod)
	}

	setPromAnnotation, ok := pod.ObjectMeta.Annotations[constants.SetPrometheusAnnotation]
	if !ok {
		pod.ObjectMeta.Annotations[constants.SetPrometheusAnnotation] = ma.EnablePrometheusScraping
		setPromAnnotation = ma.EnablePrometheusScraping
	}
	if setPromAnnotation == "true" {
		// Aggregation on ⇒ point prometheus at the aggregator port;
		// off ⇒ the default queue-proxy prometheus port.
		podPromPort := constants.DefaultPodPrometheusPort
		if enableMetricAggregation == "true" {
			podPromPort = strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)
		}
		pod.ObjectMeta.Annotations[constants.PrometheusPortAnnotationKey] = podPromPort
		pod.ObjectMeta.Annotations[constants.PrometheusPathAnnotationKey] = constants.DefaultPrometheusPath
	}
	return nil
}
