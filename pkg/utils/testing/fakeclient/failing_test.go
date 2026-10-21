package fakeclient_test

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

var refused = errors.New("refused")

func TestFailingVerb_EveryCall_RefusesEachCall(t *testing.T) {
	c := fakeclient.New(t, fakeclient.FailingVerb(fakeclient.List, fakeclient.EveryCall, refused))
	for i := 0; i < 2; i++ {
		if err := c.List(context.Background(), &corev1.PodList{}); !errors.Is(err, refused) {
			t.Errorf("list %d = %v, want refused", i+1, err)
		}
	}
}

func TestFailingVerb_NthCall_RefusesOnlyThatCall(t *testing.T) {
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm), fakeclient.FailingVerb(fakeclient.Get, 2, refused))
	want := []error{nil, refused, nil}
	for i, w := range want {
		err := c.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
		if !errors.Is(err, w) {
			t.Errorf("get %d = %v, want %v", i+1, err, w)
		}
	}
}

func TestFailingVerb_OnKind_LeavesOtherKindsAlone(t *testing.T) {
	p, cm := pod("a", ""), configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(p, cm),
		fakeclient.FailingVerb(fakeclient.Get, fakeclient.EveryCall, refused).OnKind(&corev1.Pod{}))
	ctx := context.Background()
	if err := c.Get(ctx, client.ObjectKeyFromObject(p), &corev1.Pod{}); !errors.Is(err, refused) {
		t.Errorf("get pod = %v, want refused", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Errorf("get configmap = %v, want it answered", err)
	}
}

func TestFailingVerb_StatusUpdate_LeavesUpdateAlone(t *testing.T) {
	p := pod("a", "")
	c := fakeclient.New(t, fakeclient.WithObjects(p), fakeclient.WithStatusSubresource(&corev1.Pod{}),
		fakeclient.FailingVerb(fakeclient.StatusUpdate, fakeclient.EveryCall, refused))
	ctx := context.Background()
	if err := c.Update(ctx, p); err != nil {
		t.Errorf("update = %v, want it answered", err)
	}
	if err := c.Status().Update(ctx, p); !errors.Is(err, refused) {
		t.Errorf("status update = %v, want refused", err)
	}
}

func TestFailingVerb_RefusedWrite_NeverReachesTheStore(t *testing.T) {
	c := fakeclient.New(t, fakeclient.FailingVerb(fakeclient.Create, fakeclient.EveryCall, refused))
	cm := configMap("a", "v")
	ctx := context.Background()
	if err := c.Create(ctx, cm); !errors.Is(err, refused) {
		t.Fatalf("create = %v, want refused", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
		t.Errorf("get after a refused create = %v, want NotFound", err)
	}
}

func TestFailingVerb_TwoRules_EachCountsItsOwnCalls(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm),
		fakeclient.FailingVerb(fakeclient.Get, 1, first),
		fakeclient.FailingVerb(fakeclient.Get, 2, second))
	want := []error{first, second, nil}
	for i, w := range want {
		err := c.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
		if !errors.Is(err, w) {
			t.Errorf("get %d = %v, want %v", i+1, err, w)
		}
	}
}

func TestFailingWrites_EveryCall_RefusesWritesAndAnswersReads(t *testing.T) {
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm), fakeclient.FailingWrites(fakeclient.EveryCall, refused))
	ctx := context.Background()
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("get = %v, want it answered", err)
	}
	if err := c.Update(ctx, cm); !errors.Is(err, refused) {
		t.Errorf("update = %v, want refused", err)
	}
	if err := c.Delete(ctx, cm); !errors.Is(err, refused) {
		t.Errorf("delete = %v, want refused", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Errorf("get after refused delete = %v, want the object still there", err)
	}
}

func TestFailingWrites_NthCall_CountsAcrossTheWriteVerbs(t *testing.T) {
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm), fakeclient.FailingWrites(2, refused))
	ctx := context.Background()
	if err := c.Update(ctx, cm); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if err := c.Create(ctx, pod("a", "")); !errors.Is(err, refused) {
		t.Errorf("create as write 2 = %v, want refused", err)
	}
	if err := c.Delete(ctx, cm); err != nil {
		t.Errorf("delete as write 3 = %v, want it answered", err)
	}
}
