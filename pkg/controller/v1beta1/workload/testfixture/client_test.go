package testfixture_test

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
)

func TestFakeClient_StoresEveryKindAnEnginePassTouches(t *testing.T) {
	c := fixture.FakeClient(t)
	ctx := context.Background()
	for _, obj := range []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}},
		&appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: "rev", Namespace: "ns"}},
		&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "slice", Namespace: "ns"}, AddressType: discoveryv1.AddressTypeIPv4},
		&v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "ns"}},
		&v1beta1.InferenceReplica{ObjectMeta: metav1.ObjectMeta{Name: "svc-engine", Namespace: "ns"}},
	} {
		if err := c.Create(ctx, obj); err != nil {
			t.Errorf("create %T: %v", obj, err)
		}
	}
}

func TestFakeClient_ReplicaStatusIsASubresource(t *testing.T) {
	ir := &v1beta1.InferenceReplica{ObjectMeta: metav1.ObjectMeta{Name: "svc-engine", Namespace: "ns"}}
	c := fixture.FakeClient(t, ir)
	ctx := context.Background()
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{Index: 0}}
	if err := c.Update(ctx, ir); err != nil {
		t.Fatalf("update: %v", err)
	}
	stored := &v1beta1.InferenceReplica{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(ir), stored); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(stored.Status.InstanceStatuses) != 0 {
		t.Fatalf("a plain update wrote the status: %+v", stored.Status.InstanceStatuses)
	}
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{Index: 0}}
	if err := c.Status().Update(ctx, ir); err != nil {
		t.Fatalf("status update: %v", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(ir), stored); err != nil || len(stored.Status.InstanceStatuses) != 1 {
		t.Fatalf("after the status update: rows = %+v, err = %v, want one row", stored.Status.InstanceStatuses, err)
	}
}

func TestFakeClientWithInterceptors_RunsTheHookOnEveryCall(t *testing.T) {
	gets := 0
	c := fixture.FakeClientWithInterceptors(t, interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			gets++
			return cl.Get(ctx, key, obj, opts...)
		},
	}, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}})
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "pod"}, &corev1.Pod{}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if gets != 1 {
		t.Fatalf("hook ran %d times, want once", gets)
	}
}
