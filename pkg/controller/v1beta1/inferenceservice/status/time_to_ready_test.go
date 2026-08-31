package status

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knapis "knative.dev/pkg/apis"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/obsmetrics"
)

// timeToReadyFor returns the (count, sum) of the ome_isvc_time_to_ready_seconds
// sample carrying the given labels, or (0, 0) when there is no such sample.
func timeToReadyFor(t *testing.T, labels prometheus.Labels) (uint64, float64) {
	t.Helper()
	gathered, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range gathered {
		if mf.GetName() != "ome_isvc_time_to_ready_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !matches(m.GetLabel(), labels) {
				continue
			}
			return m.Histogram.GetSampleCount(), m.Histogram.GetSampleSum()
		}
	}
	return 0, 0
}

func matches(pairs []*dto.LabelPair, labels prometheus.Labels) bool {
	for _, p := range pairs {
		if want, ok := labels[p.GetName()]; ok && want != p.GetValue() {
			return false
		}
	}
	return true
}

func isvcCreatedAgo(name string, d time.Duration) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "prod",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-d)),
		},
	}
}

func readyCond(status v1.ConditionStatus, ago time.Duration) *knapis.Condition {
	return &knapis.Condition{
		Type:               knapis.ConditionReady,
		Status:             status,
		LastTransitionTime: knapis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-ago))},
	}
}

// With no prior Ready condition the ISVC has never converged, so creation is
// the only defensible anchor and the deploy is a create.
func TestObserveTimeToReady_NoPriorConditionAnchorsOnCreation(t *testing.T) {
	labels := prometheus.Labels{"namespace": "prod", "isvc": "anchor-create", "deploy_kind": obsmetrics.DeployKindCreate}
	startCount, startSum := timeToReadyFor(t, labels)

	ObserveTimeToReady(isvcCreatedAgo("anchor-create", 5*time.Minute), nil)

	cnt, sum := timeToReadyFor(t, labels)
	if cnt-startCount != 1 {
		t.Fatalf("count delta: got %v want 1", cnt-startCount)
	}
	if d := sum - startSum; d < 299 || d > 301 {
		t.Errorf("expected ~300s measured from creation, got %v", d)
	}
}

// A still-initializing ISVC reports Ready=Unknown; the anchor moves to that
// condition's transition time but the deploy is still a create.
func TestObserveTimeToReady_UnknownIsCreateAnchoredOnCondition(t *testing.T) {
	labels := prometheus.Labels{"namespace": "prod", "isvc": "anchor-unknown", "deploy_kind": obsmetrics.DeployKindCreate}
	startCount, startSum := timeToReadyFor(t, labels)

	isvc := isvcCreatedAgo("anchor-unknown", 30*time.Minute)
	ObserveTimeToReady(isvc, readyCond(v1.ConditionUnknown, 2*time.Minute))

	cnt, sum := timeToReadyFor(t, labels)
	if cnt-startCount != 1 {
		t.Fatalf("count delta: got %v want 1", cnt-startCount)
	}
	if d := sum - startSum; d < 119 || d > 121 {
		t.Errorf("expected ~120s measured from the condition, not creation, got %v", d)
	}
}

// A previously-serving ISVC reports Ready=False while rolling out, which is
// the update case and anchors on when it lost readiness.
func TestObserveTimeToReady_FalseIsUpdate(t *testing.T) {
	labels := prometheus.Labels{"namespace": "prod", "isvc": "anchor-update", "deploy_kind": obsmetrics.DeployKindUpdate}
	startCount, startSum := timeToReadyFor(t, labels)

	isvc := isvcCreatedAgo("anchor-update", 48*time.Hour)
	ObserveTimeToReady(isvc, readyCond(v1.ConditionFalse, 90*time.Second))

	cnt, sum := timeToReadyFor(t, labels)
	if cnt-startCount != 1 {
		t.Fatalf("count delta: got %v want 1", cnt-startCount)
	}
	if d := sum - startSum; d < 89 || d > 91 {
		t.Errorf("expected ~90s, got %v", d)
	}
}

// A zero LastTransitionTime is not a usable anchor; fall back to creation
// rather than measuring from the epoch.
func TestObserveTimeToReady_ZeroTransitionTimeFallsBackToCreation(t *testing.T) {
	labels := prometheus.Labels{"namespace": "prod", "isvc": "anchor-zero", "deploy_kind": obsmetrics.DeployKindCreate}
	startCount, startSum := timeToReadyFor(t, labels)

	cond := &knapis.Condition{Type: knapis.ConditionReady, Status: v1.ConditionFalse}
	ObserveTimeToReady(isvcCreatedAgo("anchor-zero", time.Minute), cond)

	cnt, sum := timeToReadyFor(t, labels)
	if cnt-startCount != 1 {
		t.Fatalf("count delta: got %v want 1", cnt-startCount)
	}
	if d := sum - startSum; d < 59 || d > 61 {
		t.Errorf("expected ~60s from creation, got %v", d)
	}
}

func TestObserveTimeToReady_NilISVCIsNoOp(t *testing.T) {
	before, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	ObserveTimeToReady(nil, nil)
	after, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(before) != len(after) {
		t.Errorf("nil ISVC changed the registry shape: %d -> %d families", len(before), len(after))
	}
}

func TestIsReadyTrue(t *testing.T) {
	cases := []struct {
		name string
		cond *knapis.Condition
		want bool
	}{
		{"no conditions", nil, false},
		{"unknown", readyCond(v1.ConditionUnknown, 0), false},
		{"false", readyCond(v1.ConditionFalse, 0), false},
		{"true", readyCond(v1.ConditionTrue, 0), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var st v1beta1.InferenceServiceStatus
			if tc.cond != nil {
				st.Conditions = []knapis.Condition{*tc.cond}
			}
			if got := IsReadyTrue(st); got != tc.want {
				t.Errorf("IsReadyTrue = %v, want %v", got, tc.want)
			}
		})
	}
}
