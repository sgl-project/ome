package v1beta1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadClusterRoundTrip(t *testing.T) {
	wc := &WorkloadCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: "ome.io/v1beta1", Kind: "WorkloadCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-a", Labels: map[string]string{"gpu": "gb300"}},
		Spec: WorkloadClusterSpec{
			ClusterSource: ClusterConnectionSource{
				KubeConfig: &KubeConfigSource{
					SecretRef: corev1.SecretReference{Name: "cluster-a-kubeconfig", Namespace: "ome-system"},
					Key:       "kubeconfig",
				},
			},
		},
		Status: WorkloadClusterStatus{
			Conditions: []metav1.Condition{{
				Type:   WorkloadClusterReady,
				Status: metav1.ConditionTrue,
				Reason: "Reachable",
				// Realistic Condition (the API server requires these). Use a
				// fixed, second-precision time so JSON (RFC3339, no sub-second)
				// round-trips exactly — metav1.Now() would lose nanoseconds and
				// fail the cmp.Diff.
				LastTransitionTime: metav1.NewTime(time.Unix(1700000000, 0).UTC()),
				Message:            "Connection verified",
			}},
		},
	}
	data, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got WorkloadCluster
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if diff := cmp.Diff(wc, &got); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wc, wc.DeepCopy()); diff != "" {
		t.Errorf("DeepCopy mismatch (-want +got):\n%s", diff)
	}
}
