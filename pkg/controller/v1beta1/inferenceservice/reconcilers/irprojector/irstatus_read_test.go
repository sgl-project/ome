package irprojector

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestComponentIRStatus_ReturnsAuthoritativeStatus(t *testing.T) {
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: InferenceReplicaName("svc", v1beta1.EngineComponent)},
		Status: v1beta1.InferenceReplicaStatus{
			Replicas: 3, ReadyReplicas: 2,
			InstanceStatuses: []v1beta1.OMENativeInstanceStatus{{Index: 0, Phase: v1beta1.OMENativeInstanceReady}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(ir).Build()

	got, err := ComponentIRStatus(context.Background(), c, "ns", "svc", v1beta1.EngineComponent)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil || got.Replicas != 3 || len(got.InstanceStatuses) != 1 {
		t.Fatalf("want authoritative IR status (3 replicas, 1 instance); got %+v", got)
	}
}

func TestComponentIRStatus_MissingIRReturnsNil(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	got, err := ComponentIRStatus(context.Background(), c, "ns", "svc", v1beta1.EngineComponent)
	if err != nil {
		t.Fatalf("missing IR must not error; got %v", err)
	}
	if got != nil {
		t.Fatalf("missing IR must return nil status; got %+v", got)
	}
}

func TestComponentIRStatus_NilReaderErrors(t *testing.T) {
	// A nil reader is a wiring bug: it must surface as an error (so gate
	// callers fail closed) rather than being mistaken for a missing IR —
	// and must not panic the reconcile.
	got, err := ComponentIRStatus(context.Background(), nil, "ns", "svc", v1beta1.EngineComponent)
	if err == nil {
		t.Fatalf("nil reader must return an error; got (%+v, nil)", got)
	}
	if got != nil {
		t.Fatalf("nil reader must return nil status; got %+v", got)
	}
	if !strings.Contains(err.Error(), "nil reader") {
		t.Errorf("error should name the nil-reader cause: %v", err)
	}
}

// erroringReader fails every Get with a non-NotFound error.
type erroringReader struct {
	client.Reader
}

func (erroringReader) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return errors.New("simulated apiserver failure")
}

func TestComponentIRStatus_ReadErrorPropagatesWrapped(t *testing.T) {
	// Only NotFound maps to (nil, nil). Any other read failure must
	// surface as a wrapped error so gate callers can fail closed instead
	// of mistaking a flaky read for "no observation yet".
	got, err := ComponentIRStatus(context.Background(), erroringReader{}, "ns", "svc", v1beta1.EngineComponent)
	if err == nil || got != nil {
		t.Fatalf("read error must propagate with nil status; got (%+v, %v)", got, err)
	}
	if !strings.Contains(err.Error(), "get InferenceReplica ns/") || !strings.Contains(err.Error(), "simulated apiserver failure") {
		t.Errorf("error should wrap the IR key and cause: %v", err)
	}
}
