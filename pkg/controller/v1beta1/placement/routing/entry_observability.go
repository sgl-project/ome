package routing

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

var (
	trafficMapEntryWeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ome_trafficmap_entry_weight",
		Help: "Relative routing weight saved for a member in the TrafficMap spec. This does not report the weight applied by the gateway.",
	}, []string{"namespace", "isvc", "cluster"})
	trafficMapEntryHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ome_trafficmap_entry_healthy",
		Help: "Member health saved in the TrafficMap spec (1 healthy, 0 unhealthy). This does not report gateway health.",
	}, []string{"namespace", "isvc", "cluster"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(trafficMapEntryWeight, trafficMapEntryHealthy)
}

// Record only a saved spec so failed writes cannot advertise an unsaved routing
// plan. Replacing this service's series also removes entries absent from the map.
func recordTrafficMapEntryMetrics(namespace, isvc string, spec v1beta1.TrafficMapSpec) {
	deleteTrafficMapEntryMetrics(namespace, isvc)
	for _, entry := range spec.Entries {
		trafficMapEntryWeight.WithLabelValues(namespace, isvc, entry.Cluster).Set(float64(entry.Weight))
		healthy := float64(0)
		if entry.Healthy {
			healthy = 1
		}
		trafficMapEntryHealthy.WithLabelValues(namespace, isvc, entry.Cluster).Set(healthy)
	}
}

func deleteTrafficMapEntryMetrics(namespace, isvc string) {
	labels := prometheus.Labels{"namespace": namespace, "isvc": isvc}
	trafficMapEntryWeight.DeletePartialMatch(labels)
	trafficMapEntryHealthy.DeletePartialMatch(labels)
}
