package gangpack

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
)

// TestNewInformerReaderDoesNotHangWithoutPodGroupAPI: the PodGroup CRD is an
// optional dependency, so building the reader must NOT block forever when the
// PodGroup API is unavailable (CRD absent, or apiserver unreachable). The initial
// cache sync is time-bounded; newInformerReader returns after the timeout so the
// scheduler still boots. Simulated with an unreachable apiserver + a short bound.
func TestNewInformerReaderDoesNotHangWithoutPodGroupAPI(t *testing.T) {
	cfg := &rest.Config{Host: "https://127.0.0.1:1"} // nothing listening -> never syncs

	done := make(chan error, 1)
	go func() {
		_, err := newInformerReader(context.Background(), cfg, topologyKeyAnnotation, placementGroupLabel, time.Minute, 200*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("newInformerReader returned an error, want nil (boot regardless): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("newInformerReader hung when the PodGroup API was unavailable; it must time-bound the initial sync")
	}
}

func TestFactsFromPodGroup(t *testing.T) {
	pg := &schedv1alpha1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{topologyKeyAnnotation: "nvidia.com/gpu.clique"}},
		Spec:       schedv1alpha1.PodGroupSpec{MinMember: 18, ScheduleTimeoutSeconds: ptr.To[int32](120)},
	}
	const fallback = 10 * time.Minute
	if mm, tk, to := factsFromPodGroup(pg, topologyKeyAnnotation, fallback); mm != 18 || tk != "nvidia.com/gpu.clique" || to != 120*time.Second {
		t.Fatalf("factsFromPodGroup = %d,%q,%v want 18,nvidia.com/gpu.clique,2m", mm, tk, to)
	}
	// No annotation -> empty key; no ScheduleTimeoutSeconds -> default timeout.
	if mm, tk, to := factsFromPodGroup(&schedv1alpha1.PodGroup{Spec: schedv1alpha1.PodGroupSpec{MinMember: 4}}, topologyKeyAnnotation, fallback); mm != 4 || tk != "" || to != fallback {
		t.Fatalf("factsFromPodGroup(no annotation) = %d,%q,%v want 4,'',default", mm, tk, to)
	}
	if mm, tk, to := factsFromPodGroup(nil, topologyKeyAnnotation, fallback); mm != 0 || tk != "" || to != fallback {
		t.Fatalf("factsFromPodGroup(nil) = %d,%q,%v want 0,'',default", mm, tk, to)
	}
}

func TestInformerReaderGet(t *testing.T) {
	pg := &schedv1alpha1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "pf", UID: types.UID("pg-uid"), Annotations: map[string]string{topologyKeyAnnotation: "clique"}},
		Spec:       schedv1alpha1.PodGroupSpec{MinMember: 8},
	}
	r := &informerReader{topologyKeyAnnotation: topologyKeyAnnotation, getPG: func(ns, name string) (*schedv1alpha1.PodGroup, error) {
		if ns == "team" && name == "pf" {
			return pg, nil
		}
		return nil, errors.New("not found")
	}}

	if mm, tk, _, uid, ok := r.get("team", "pf"); !ok || mm != 8 || tk != "clique" || uid != "pg-uid" {
		t.Fatalf("get(team/pf) = %d,%q,%v want 8,clique,true", mm, tk, ok)
	}
	if _, _, _, _, ok := r.get("team", "missing"); ok {
		t.Fatal("get(missing) should be not-found")
	}

	// A getPG that returns (nil, nil) is treated as not found, not a panic.
	rNil := &informerReader{getPG: func(string, string) (*schedv1alpha1.PodGroup, error) { return nil, nil }}
	if _, _, _, _, ok := rNil.get("x", "y"); ok {
		t.Fatal("nil PodGroup should be not-found")
	}
}
