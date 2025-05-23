package pod

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/utils"
	v1 "k8s.io/api/core/v1"
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
			panic(fmt.Errorf("Unable to unmarshall %v json string due to %w ", MetricsAggregatorConfigMapKeyName, err))
		}
	}

	return ma, nil
}

func setMetricAggregationEnvVarsAndPorts(pod *v1.Pod) {
	for i, container := range pod.Spec.Containers {
		if container.Name == "queue-proxy" {
			// The ome-container prometheus port/path is inherited from the ClusterServingRuntime YAML.
			// If no port is defined (transformer using python SDK), use the default port/path for the ome-container.
			omeContainerPromPort := defaultKserveContainerPrometheusPort
			if port, ok := pod.ObjectMeta.Annotations[constants.ContainerPrometheusPortKey]; ok {
				omeContainerPromPort = port
			}

			omeContainerPromPath := constants.DefaultPrometheusPath
			if path, ok := pod.ObjectMeta.Annotations[constants.ContainerPrometheusPathKey]; ok {
				omeContainerPromPath = path
			}

			// The ome container port/path is set as an EnvVar in the queue-proxy container
			// so that it knows which port/path to scrape from the ome-container.
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, v1.EnvVar{Name: constants.ContainerPrometheusMetricsPortEnvVarKey, Value: omeContainerPromPort})
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, v1.EnvVar{Name: constants.ContainerPrometheusMetricsPathEnvVarKey, Value: omeContainerPromPath})

			// Set the port that queue-proxy will use to expose the aggregate metrics.
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, v1.EnvVar{Name: constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey, Value: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)})

			pod.Spec.Containers[i].Ports = utils.AppendPortIfNotExists(pod.Spec.Containers[i].Ports, v1.ContainerPort{
				Name:          constants.AggregateMetricsPortName,
				ContainerPort: int32(constants.QueueProxyAggregatePrometheusMetricsPort),
				Protocol:      "TCP",
			})
		}
	}
}

// InjectMetricsAggregator looks for the annotations to enable aggregate ome-container and queue-proxy metrics and
// if specified, sets port-related EnvVars in queue-proxy and the aggregate prometheus annotation.
func (ma *MetricsAggregator) InjectMetricsAggregator(pod *v1.Pod) error {
	// Only set metric configs if the required annotations are set
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

	// Handle setting the pod prometheus annotations
	setPromAnnotation, ok := pod.ObjectMeta.Annotations[constants.SetPrometheusAnnotation]
	if !ok {
		pod.ObjectMeta.Annotations[constants.SetPrometheusAnnotation] = ma.EnablePrometheusScraping
		setPromAnnotation = ma.EnablePrometheusScraping
	}
	if setPromAnnotation == "true" {
		// Set prometheus port to default queue proxy prometheus metrics port.
		// If enableMetricAggregation is true, set it as the queue proxy metrics aggregation port.
		podPromPort := constants.DefaultPodPrometheusPort
		if enableMetricAggregation == "true" {
			podPromPort = strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)
		}
		pod.ObjectMeta.Annotations[constants.PrometheusPortAnnotationKey] = podPromPort
		pod.ObjectMeta.Annotations[constants.PrometheusPathAnnotationKey] = constants.DefaultPrometheusPath
	}

	return nil
}
