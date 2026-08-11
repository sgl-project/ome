package placement

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func wcWithReady(status metav1.ConditionStatus, labels map[string]string) *v1beta1.WorkloadCluster {
	return &v1beta1.WorkloadCluster{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Status: v1beta1.WorkloadClusterStatus{
			Conditions: []metav1.Condition{{Type: v1beta1.WorkloadClusterReady, Status: status}},
		},
	}
}

// Routine status heartbeats (Ready unchanged, labels unchanged) must NOT
// re-enqueue ISVCs — this is the fan-out the predicate exists to suppress.
func TestPlacementRelevantClusterChange_DropsHeartbeat(t *testing.T) {
	wc := wcWithReady(metav1.ConditionTrue, map[string]string{"region": "region-a"})
	// Same readiness + labels, a fresh re-stamp of the Ready condition (new
	// LastTransitionTime / resourceVersion the heartbeat would carry).
	updated := wcWithReady(metav1.ConditionTrue, map[string]string{"region": "region-a"})
	assert.False(t, placementRelevantClusterChange.Update(event.UpdateEvent{ObjectOld: wc, ObjectNew: updated}))
}

// A readiness flip changes placement candidacy and MUST re-enqueue.
func TestPlacementRelevantClusterChange_ReadyFlip(t *testing.T) {
	old := wcWithReady(metav1.ConditionTrue, nil)
	updated := wcWithReady(metav1.ConditionFalse, nil)
	assert.True(t, placementRelevantClusterChange.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}))
	assert.True(t, placementRelevantClusterChange.Update(event.UpdateEvent{ObjectOld: updated, ObjectNew: old}))
}

// A label change can change which clusters a selector matches and MUST re-enqueue.
func TestPlacementRelevantClusterChange_LabelChange(t *testing.T) {
	old := wcWithReady(metav1.ConditionTrue, map[string]string{"region": "region-a"})
	updated := wcWithReady(metav1.ConditionTrue, map[string]string{"region": "region-b"})
	assert.True(t, placementRelevantClusterChange.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}))
}

// Create/Delete (capacity appears/disappears) always re-enqueue.
func TestPlacementRelevantClusterChange_CreateDelete(t *testing.T) {
	wc := wcWithReady(metav1.ConditionTrue, nil)
	assert.True(t, placementRelevantClusterChange.Create(event.CreateEvent{Object: wc}))
	assert.True(t, placementRelevantClusterChange.Delete(event.DeleteEvent{Object: wc}))
}
