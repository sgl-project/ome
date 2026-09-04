package workloadcluster

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// resetClusterMetrics clears both vectors so whole-vector counts are
// deterministic. Required because other tests in this package reconcile
// WorkloadClusters into these same globals.
func resetClusterMetrics() {
	clusterReady.Reset()
	clusterStatus.Reset()
}

func wcWithReady(name string, status metav1.ConditionStatus, reason string) *v1beta1.WorkloadCluster {
	wc := &v1beta1.WorkloadCluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if status != "" {
		wc.Status.Conditions = []metav1.Condition{{
			Type:   v1beta1.WorkloadClusterReady,
			Status: status,
			Reason: reason,
		}}
	}
	return wc
}

// TestRecordWorkloadCluster_ReadyAndNotReady: the 0/1 gauge is what makes
// count() the fleet size and sum() the ready count, so both states must be
// present rather than the unready cluster being omitted.
func TestRecordWorkloadCluster_ReadyAndNotReady(t *testing.T) {
	resetClusterMetrics()

	recordWorkloadCluster(wcWithReady("a", metav1.ConditionTrue, "Connected"))
	recordWorkloadCluster(wcWithReady("b", metav1.ConditionFalse, "ConnectionFailed"))

	if got := testutil.ToFloat64(clusterReady.WithLabelValues("a")); got != 1 {
		t.Errorf("ready{a} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(clusterReady.WithLabelValues("b")); got != 0 {
		t.Errorf("ready{b} = %v, want 0", got)
	}
	if got := testutil.CollectAndCount(clusterReady); got != 2 {
		t.Errorf("ready series = %d, want 2 (fleet size)", got)
	}
	if got := testutil.ToFloat64(clusterStatus.WithLabelValues("b", "False", "ConnectionFailed")); got != 1 {
		t.Errorf("status{b,False,ConnectionFailed} = %v, want 1", got)
	}
}

// TestRecordWorkloadCluster_ReasonChurnDoesNotAccumulate: Reason is a churning
// label, so a cluster must carry exactly one status series at any time.
func TestRecordWorkloadCluster_ReasonChurnDoesNotAccumulate(t *testing.T) {
	resetClusterMetrics()

	recordWorkloadCluster(wcWithReady("a", metav1.ConditionFalse, "ConnectionFailed"))
	recordWorkloadCluster(wcWithReady("a", metav1.ConditionTrue, "ProbeFailedRetrying"))
	recordWorkloadCluster(wcWithReady("a", metav1.ConditionTrue, "Connected"))

	if got := testutil.CollectAndCount(clusterStatus); got != 1 {
		t.Fatalf("status series = %d, want 1 (prior reasons must not linger)", got)
	}
	if got := testutil.ToFloat64(clusterStatus.WithLabelValues("a", "True", "Connected")); got != 1 {
		t.Errorf("status{a,True,Connected} = %v, want 1", got)
	}
}

// TestRecordWorkloadCluster_UnassessedCounts: a member whose condition has not
// been written yet is still a member; dropping it would understate the fleet.
func TestRecordWorkloadCluster_UnassessedCounts(t *testing.T) {
	resetClusterMetrics()

	recordWorkloadCluster(wcWithReady("fresh", "", ""))

	if got := testutil.ToFloat64(clusterReady.WithLabelValues("fresh")); got != 0 {
		t.Errorf("ready{fresh} = %v, want 0", got)
	}
	if got := testutil.ToFloat64(clusterStatus.WithLabelValues("fresh", "Unknown", "NotAssessed")); got != 1 {
		t.Errorf("status{fresh,Unknown,NotAssessed} = %v, want 1", got)
	}
}

// TestDeleteForCluster_DropsEverything: a removed member must stop counting
// toward the fleet size.
func TestDeleteForCluster_DropsEverything(t *testing.T) {
	resetClusterMetrics()

	recordWorkloadCluster(wcWithReady("gone", metav1.ConditionTrue, "Connected"))
	deleteForCluster("gone")

	if got := testutil.CollectAndCount(clusterReady); got != 0 {
		t.Errorf("ready series = %d, want 0", got)
	}
	if got := testutil.CollectAndCount(clusterStatus); got != 0 {
		t.Errorf("status series = %d, want 0", got)
	}
}
