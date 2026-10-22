package testfixture

import (
	"context"
	"encoding/json"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/revision"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// EngineRevisionKey is the revision key of the service's engine component,
// the one the service adapter hands the engine.
func EngineRevisionKey(isvc *v1beta1.InferenceService) revision.Key {
	return revision.Key{
		Namespace: isvc.Namespace,
		Name:      isvc.Name + "-" + string(types.ComponentEngine),
		Labels: map[string]string{
			constants.InferenceServicePodLabelKey: isvc.Name,
			constants.OMEComponentLabel:           string(types.ComponentEngine),
			query.LabelManagedBy:                  query.ManagedByOMENative,
		},
	}
}

// EnsureTargetRevision stores the ControllerRevision of the given pod spec
// for the service's engine, the revision an update then rolls toward.
func EnsureTargetRevision(t *testing.T, c client.Client, isvc *v1beta1.InferenceService, spec *corev1.PodSpec) *appsv1.ControllerRevision {
	t.Helper()
	return EnsureTargetRevisionWithMeta(t, c, isvc, spec, nil)
}

// EnsureTargetRevisionWithMeta is EnsureTargetRevision with the pod
// metadata the revision records beside the spec, so an in-place roll has
// the annotations it must carry onto the live pod. nil meta records none.
func EnsureTargetRevisionWithMeta(t *testing.T, c client.Client, isvc *v1beta1.InferenceService, spec *corev1.PodSpec, meta *metav1.ObjectMeta) *appsv1.ControllerRevision {
	t.Helper()
	cr, _, err := revision.EnsureControllerRevision(
		context.Background(), c, c, isvc,
		v1beta1.SchemeGroupVersion.WithKind("InferenceService"),
		EngineRevisionKey(isvc),
		spec, meta, nil, isvc.UID,
	)
	if err != nil {
		t.Fatalf("revision.EnsureControllerRevision: %v", err)
	}
	return cr
}

// StoredRevision is a ControllerRevision of the given name whose payload
// records spec and workerSpec as its template, for a row that names a
// revision by a literal name instead of minting one.
func StoredRevision(t *testing.T, namespace, name string, spec, workerSpec *corev1.PodSpec) *appsv1.ControllerRevision {
	t.Helper()
	raw, err := json.Marshal(revision.DataPayload{PodSpec: spec, WorkerPodSpec: workerSpec})
	if err != nil {
		t.Fatalf("marshal revision payload: %v", err)
	}
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       runtime.RawExtension{Raw: raw},
	}
}

// SeedRunningRevision stores the ControllerRevision of spec and records it
// as the running revision of the Instance's row on the replica. The engine
// compares an update against the recorded running template, not against
// the live pod, when it decides whether the change can be made in place.
func SeedRunningRevision(t *testing.T, c client.Client, isvc *v1beta1.InferenceService, component types.ComponentType, idx int32, spec *corev1.PodSpec) {
	t.Helper()
	SeedRunningRevisionWithMeta(t, c, isvc, component, idx, spec, nil)
}

// SeedRunningRevisionWithMeta is SeedRunningRevision with the pod metadata
// the running revision records, the baseline an in-place annotation diff
// subtracts from.
func SeedRunningRevisionWithMeta(t *testing.T, c client.Client, isvc *v1beta1.InferenceService, component types.ComponentType, idx int32, spec *corev1.PodSpec, meta *metav1.ObjectMeta) {
	t.Helper()
	cr, _, err := revision.EnsureControllerRevision(
		context.Background(), c, c, isvc,
		v1beta1.SchemeGroupVersion.WithKind("InferenceService"),
		EngineRevisionKey(isvc),
		spec, meta, nil, isvc.UID,
	)
	if err != nil {
		t.Fatalf("revision.EnsureControllerRevision: %v", err)
	}
	ir := &v1beta1.InferenceReplica{}
	key := k8stypes.NamespacedName{Namespace: isvc.Namespace, Name: InferenceReplicaName(isvc, component)}
	if err := c.Get(context.Background(), key, ir); err != nil {
		t.Fatalf("re-read IR: %v", err)
	}
	for i := range ir.Status.InstanceStatuses {
		if ir.Status.InstanceStatuses[i].Index == idx {
			ir.Status.InstanceStatuses[i].RunningRevision = cr.Name
			if err := c.Status().Update(context.Background(), ir); err != nil {
				t.Fatalf("set RunningRevision: %v", err)
			}
			return
		}
	}
	t.Fatalf("no InstanceStatus for idx=%d to attach RunningRevision", idx)
}
