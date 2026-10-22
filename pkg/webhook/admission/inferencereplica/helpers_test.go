package inferencereplica

import (
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

const (
	// ownerRefDenial is the denial for a controller owner reference only the
	// InferenceService controller may set.
	ownerRefDenial = "only the InferenceService controller may set an InferenceService controller owner reference or change a projected replica's"
	// composerDenial is the denial for a controller write to a user field of
	// a replica it does not project; it lists the fields the controller may
	// write.
	composerDenial = "the InferenceService controller writes only rollout-control fields (pacing.partition, paused, pauseMode, pacing.rollbackToRevision, pairingProtocol) on a replica it does not project"
	// composedFieldsDenial is the denial for a non-controller write to the
	// composed-fields annotation.
	composedFieldsDenial = "the ome.io/composed-fields annotation is written by the InferenceService controller"
)

func newDecoder(t *testing.T) admission.Decoder {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("v1beta1.AddToScheme: %v", err)
	}
	return admission.NewDecoder(s)
}

func encode(t *testing.T, obj *v1beta1.InferenceReplica) []byte {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// baselineIR returns a syntactically-valid projected InferenceReplica
// (InferenceService controller owner reference plus parentRef) suitable
// for cloning into Create / Update fixtures.
func baselineIR(annotations map[string]string) *v1beta1.InferenceReplica {
	return &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "llama-engine",
			Namespace:   "prod-models",
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1beta1.SchemeGroupVersion.String(), Kind: "InferenceService",
				Name: "llama", UID: "isvc-uid", Controller: ptr.To(true),
			}},
		},
		Spec: v1beta1.InferenceReplicaSpec{
			ParentRef: &v1beta1.ParentReference{
				Name: "llama",
			},
			Component: v1beta1.EngineComponent,
			Runners: []v1beta1.Runner{
				{
					Name: v1beta1.RunnerNameDefault,
					Size: 1,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "ome-container",
								Image: "sgl:1.0",
							}},
						},
					},
				},
			},
		},
	}
}

// withControllerWrite stamps the controller-write annotation, matching
// what the ISVC controller's client does on every Create/Update.
func withControllerWrite() map[string]string {
	return map[string]string{
		constants.InferenceReplicaControllerWriteAnnotationKey: constants.InferenceReplicaControllerWriteAnnotationVal,
	}
}

func createReq(t *testing.T, obj *v1beta1.InferenceReplica) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: obj.Namespace,
		Name:      obj.Name,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: encode(t, obj)},
	}}
}

func updateReq(t *testing.T, oldObj, newObj *v1beta1.InferenceReplica) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Namespace: newObj.Namespace,
		Name:      newObj.Name,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		OldObject: runtime.RawExtension{Raw: encode(t, oldObj)},
		Object:    runtime.RawExtension{Raw: encode(t, newObj)},
	}}
}

// withReplicas returns a clone of `ir` with Replicas set; convenience
// for the Update-path table cases that need a spec diff.
func withReplicas(ir *v1beta1.InferenceReplica, n int32) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Spec.Replicas = &n
	return out
}

// withFinalizers returns a clone with metadata.finalizers replaced.
func withFinalizers(ir *v1beta1.InferenceReplica, finalizers ...string) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Finalizers = finalizers
	return out
}

// withLabel returns a clone with one label added.
func withLabel(ir *v1beta1.InferenceReplica, k, v string) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	out.Labels[k] = v
	return out
}

// withPacingPartition returns a clone with spec.pacing.partition set
// (creating the Pacing block if nil).
func withPacingPartition(ir *v1beta1.InferenceReplica, partition int32) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	if out.Spec.Pacing == nil {
		out.Spec.Pacing = &v1beta1.InferenceReplicaPacing{}
	}
	out.Spec.Pacing.Partition = &partition
	return out
}

// withImage returns a clone whose first runner uses a different image.
func withImage(ir *v1beta1.InferenceReplica, image string) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Spec.Runners[0].Template.Spec.Containers[0].Image = image
	return out
}

// standaloneIR is a user-created replica: no owner reference, no parentRef.
func standaloneIR() *v1beta1.InferenceReplica {
	ir := baselineIR(nil)
	ir.Name = "pool-a"
	ir.OwnerReferences = nil
	ir.Spec.ParentRef = nil
	return ir
}

func withAnnotation(ir *v1beta1.InferenceReplica, k, v string) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	if out.Annotations == nil {
		out.Annotations = map[string]string{}
	}
	out.Annotations[k] = v
	return out
}

// withBlockOwnerDeletion returns a copy of ir whose owner references carry
// the given blockOwnerDeletion, the field the garbage collector clears
// during foreground deletion.
func withBlockOwnerDeletion(ir *v1beta1.InferenceReplica, block bool) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	for i := range out.OwnerReferences {
		out.OwnerReferences[i].BlockOwnerDeletion = ptr.To(block)
	}
	return out
}

// withComposedFields returns a copy of ir carrying the composed-fields
// annotation with the given value.
func withComposedFields(ir *v1beta1.InferenceReplica, value string) *v1beta1.InferenceReplica {
	return withAnnotation(ir, constants.InferenceReplicaComposedFieldsAnnotationKey, value)
}

// refsIR is a standalone replica of component c that renders from the
// ClusterServingRuntime runtime-a instead of stored runners.
func refsIR(c v1beta1.ComponentType) *v1beta1.InferenceReplica {
	ir := standaloneIR()
	ir.Spec.Component = c
	ir.Spec.Runners = nil
	ir.Spec.RuntimeRef = &v1beta1.ServingRuntimeRef{Name: "runtime-a", Kind: ptr.To(runtimeselector.KindClusterServingRuntime)}
	return ir
}

func withModelRef(ir *v1beta1.InferenceReplica, name string) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Spec.ModelRef = &v1beta1.ModelRef{Name: name}
	return out
}

func withRuntimeRef(ir *v1beta1.InferenceReplica, ref *v1beta1.ServingRuntimeRef) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Spec.RuntimeRef = ref
	return out
}

func withoutRunners(ir *v1beta1.InferenceReplica) *v1beta1.InferenceReplica {
	out := ir.DeepCopy()
	out.Spec.Runners = nil
	return out
}
