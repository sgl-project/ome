package testfixture_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/revision"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestEngineRevisionKey_NamesTheEngineOfTheService(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	key := fixture.EngineRevisionKey(isvc)
	if key.Namespace != "ns" || key.Name != "svc-engine" || key.Labels[constants.InferenceServicePodLabelKey] != "svc" ||
		key.Labels[constants.OMEComponentLabel] != string(types.ComponentEngine) || key.Labels[query.LabelManagedBy] != query.ManagedByOMENative {
		t.Fatalf("key = %+v, want ns/svc-engine with the engine's labels", key)
	}
}

func TestEnsureTargetRevision_StoresOneRevisionPerSpec(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	c := fixture.FakeClient(t, isvc)
	first := fixture.EnsureTargetRevision(t, c, isvc, fixture.PodSpecWithImage("model:v2"))
	again := fixture.EnsureTargetRevision(t, c, isvc, fixture.PodSpecWithImage("model:v2"))
	other := fixture.EnsureTargetRevision(t, c, isvc, fixture.PodSpecWithImage("model:v3"))
	if first.Name != again.Name || first.Name == other.Name {
		t.Fatalf("revisions %s, %s, %s: the same spec must share a name and another spec must not", first.Name, again.Name, other.Name)
	}
	stored := &appsv1.ControllerRevision{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: first.Name}, stored); err != nil {
		t.Fatalf("the revision is not stored: %v", err)
	}
	meta := &metav1.ObjectMeta{Annotations: map[string]string{"a": "b"}}
	withMeta := fixture.EnsureTargetRevisionWithMeta(t, c, isvc, fixture.PodSpecWithImage("model:v2"), meta)
	if withMeta.Name == first.Name {
		t.Errorf("pod metadata must change the revision: %s", withMeta.Name)
	}
}

func TestSeedRunningRevision_RecordsTheRevisionOnTheRow(t *testing.T) {
	isvc, ir := fixture.ServiceWithReadyInstance("svc", "ns", 1)
	c := fixture.FakeClient(t, isvc, ir)
	fixture.SeedRunningRevision(t, c, isvc, types.ComponentEngine, 0, fixture.PodSpecWithImage("model:v1"))
	rows := fixture.StoredInstanceStatuses(c, isvc, types.ComponentEngine)
	want := fixture.EnsureTargetRevision(t, c, isvc, fixture.PodSpecWithImage("model:v1"))
	if len(rows) != 1 || rows[0].RunningRevision != want.Name {
		t.Fatalf("rows = %+v, want RunningRevision %s", rows, want.Name)
	}
}

func TestStoredRevision_RecordsTheTemplate(t *testing.T) {
	spec := fixture.PodSpecWithImage("model:v1")
	worker := &corev1.PodSpec{Containers: []corev1.Container{{Name: "worker", Image: "model:v1"}}}
	cr := fixture.StoredRevision(t, "ns", "svc-engine-abc", spec, worker)
	if cr.Name != "svc-engine-abc" || cr.Namespace != "ns" {
		t.Fatalf("revision %s/%s, want ns/svc-engine-abc", cr.Namespace, cr.Name)
	}
	var payload revision.DataPayload
	if err := json.Unmarshal(cr.Data.Raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(payload.PodSpec, spec) || !reflect.DeepEqual(payload.WorkerPodSpec, worker) {
		t.Fatalf("payload = %+v, want the two specs", payload)
	}
}
