package inferencereplica

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// updateInferenceReplicaStatus omits ReadyPodCount, ScheduledPodCount, and
// NodesOccupied because workload decisions derive them from current Pods.
func updateInferenceReplicaStatus(ctx context.Context, c client.Client, ir *v1beta1.InferenceReplica) error {
	clearPodDerivedInstanceObservations(ir)
	return c.Status().Update(ctx, ir)
}

func clearPodDerivedInstanceObservations(ir *v1beta1.InferenceReplica) {
	if ir == nil {
		return
	}
	for i := range ir.Status.InstanceStatuses {
		ir.Status.InstanceStatuses[i].ReadyPodCount = 0
		ir.Status.InstanceStatuses[i].ScheduledPodCount = 0
		ir.Status.InstanceStatuses[i].NodesOccupied = nil
	}
}
