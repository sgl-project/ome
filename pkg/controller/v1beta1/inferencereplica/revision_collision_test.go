package inferencereplica

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

func TestBumpCollisionCount_SameNameReplacementUntouched(t *testing.T) {
	stale := baselineIR("llama-engine", "prod", 1)
	replacement := stale.DeepCopy()
	replacement.UID = "replacement-uid"
	count := int32(7)
	replacement.Status.CollisionCount = &count
	r, c := newReconciler(t, replacement)

	bumped, err := r.bumpCollisionCount(context.Background(), stale)
	if !errors.Is(err, workload.ErrStatusOwnerGone) {
		t.Fatalf("bump error: got %v want ErrStatusOwnerGone", err)
	}
	if bumped != nil {
		t.Fatalf("bumped count: got %d want nil", *bumped)
	}

	fresh := &v1beta1.InferenceReplica{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(replacement), fresh); err != nil {
		t.Fatalf("re-read replacement: %v", err)
	}
	if fresh.Status.CollisionCount == nil || *fresh.Status.CollisionCount != count {
		t.Fatalf("replacement CollisionCount: got %v want %d", fresh.Status.CollisionCount, count)
	}
	if stale.Status.CollisionCount != nil {
		t.Fatalf("stale in-memory CollisionCount: got %d want nil", *stale.Status.CollisionCount)
	}
}
