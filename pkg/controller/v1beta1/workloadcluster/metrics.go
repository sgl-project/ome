package workloadcluster

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Fleet-level membership metrics, keyed by cluster name (cardinality is bounded
// by the WorkloadCluster count).
//
// clusterReady is a plain 0/1 gauge so fleet aggregates are trivial in PromQL;
// clusterStatus adds the condition Reason to answer "why not ready". Reason
// churns, so a cluster's prior series is dropped before the current one is set.
var (
	clusterReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ome_workload_cluster_ready",
		Help: "1 if the WorkloadCluster's Ready condition is True, else 0. Present for every WorkloadCluster known to this control plane, so count() is the fleet size and sum() the ready count.",
	}, []string{"cluster"})

	clusterStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ome_workload_cluster_status",
		Help: "Always 1, labelled with the WorkloadCluster's current Ready condition status and reason. Exactly one series per cluster.",
	}, []string{"cluster", "status", "reason"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(clusterReady, clusterStatus)
}

// recordWorkloadCluster publishes the membership metrics for one cluster from
// its Ready condition. Called on every status write funnel pass, so the gauges
// track the condition the control plane just persisted.
//
// A WorkloadCluster whose condition has not been set yet reports Unknown rather
// than being omitted: an unassessed member is still a member, and dropping it
// would make count() understate the fleet.
func recordWorkloadCluster(wc *v1beta1.WorkloadCluster) {
	if wc == nil || wc.Name == "" {
		return
	}

	status, reason := string(metav1.ConditionUnknown), "NotAssessed"
	if c := apimeta.FindStatusCondition(wc.Status.Conditions, v1beta1.WorkloadClusterReady); c != nil {
		status = string(c.Status)
		if c.Reason != "" {
			reason = c.Reason
		} else {
			reason = "Unset"
		}
	}

	ready := 0.0
	if status == string(metav1.ConditionTrue) {
		ready = 1.0
	}
	clusterReady.WithLabelValues(wc.Name).Set(ready)

	// Reset first so a cluster only ever carries its current (status, reason).
	clusterStatus.DeletePartialMatch(prometheus.Labels{"cluster": wc.Name})
	clusterStatus.WithLabelValues(wc.Name, status, reason).Set(1)
}

// deleteForCluster drops every series for a WorkloadCluster that no longer
// exists, so a removed member stops counting toward the fleet size.
func deleteForCluster(name string) {
	if name == "" {
		return
	}
	match := prometheus.Labels{"cluster": name}
	clusterReady.DeletePartialMatch(match)
	clusterStatus.DeletePartialMatch(match)
}
