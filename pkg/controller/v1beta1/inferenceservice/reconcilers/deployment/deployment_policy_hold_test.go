package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// TestCheckDeploymentExist_ExternalMarkerPreservesLiveReplicas pins the
// policy-hold replica contract: a hold hands the builder an External-class
// marker (an external writer owns the count), so a live Deployment scaled
// to 8 with a declared minReplicas floor of 1 must NOT trigger an update
// that pins replicas back to the floor — Replicas is excluded from the
// diff and the live value is carried into the target state.
func TestCheckDeploymentExist_ExternalMarkerPreservesLiveReplicas(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, appsv1.AddToScheme(scheme))

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "test-container", Image: "test-image:latest"}},
	}
	componentMeta := metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"}
	minReplicas := 1
	componentExt := &v1beta1.ComponentExtensionSpec{MinReplicas: &minReplicas}

	existing := createRawDeployment(componentMeta, componentExt, podSpec.DeepCopy())
	existing.Spec.Replicas = ptr.Int32(8)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	reconciler := NewDeploymentReconciler(cl, scheme, componentMeta, componentExt, podSpec.DeepCopy(),
		&v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerExternal})

	result, _, err := reconciler.checkDeploymentExist()
	assert.NoError(t, err)
	assert.Equal(t, constants.CheckResultExisted, result,
		"an External-class marker must exclude Replicas from the diff — a replica-only drift is not an update")

	_, err = reconciler.Reconcile()
	assert.NoError(t, err)

	stored := &appsv1.Deployment{}
	assert.NoError(t, cl.Get(context.TODO(), types.NamespacedName{Name: componentMeta.Name, Namespace: componentMeta.Namespace}, stored))
	assert.NotNil(t, stored.Spec.Replicas)
	assert.Equal(t, int32(8), *stored.Spec.Replicas,
		"the live (scaler-written) count must survive a hold pass untouched")
}

// TestCheckDeploymentExist_CarriesLastRenderedAnnotationThroughDrift pins
// the last-known-good record's survival: the rebuilt desired object never
// carries the ome.io/last-rendered-autoscaler annotation, so
// checkDeploymentExist must copy it forward from the live Deployment —
// otherwise any spec-drift update (an image change) would erase the
// freeze/recovery state exactly when a broken policy needs it.
func TestCheckDeploymentExist_CarriesLastRenderedAnnotationThroughDrift(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, appsv1.AddToScheme(scheme))

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "test-container", Image: "test-image:latest"}},
	}
	componentMeta := metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"}
	componentExt := &v1beta1.ComponentExtensionSpec{}

	const lkgRecord = `{"class":"keda","keda":{"triggers":[{"type":"prometheus"}]}}`
	existing := createRawDeployment(componentMeta, componentExt, podSpec.DeepCopy())
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	existing.Annotations[constants.AutoscalerPolicyLastRendered] = lkgRecord
	// Drift: the live image differs from the rebuilt desired spec.
	existing.Spec.Template.Spec.Containers[0].Image = "different-image:latest"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	reconciler := NewDeploymentReconciler(cl, scheme, componentMeta, componentExt, podSpec.DeepCopy(), nil)

	result, _, err := reconciler.checkDeploymentExist()
	assert.NoError(t, err)
	assert.Equal(t, constants.CheckResultUpdate, result)
	assert.Equal(t, lkgRecord, reconciler.Deployment.Annotations[constants.AutoscalerPolicyLastRendered],
		"the desired object must carry the live LKG annotation forward before the update is written")

	_, err = reconciler.Reconcile()
	assert.NoError(t, err)

	stored := &appsv1.Deployment{}
	assert.NoError(t, cl.Get(context.TODO(), types.NamespacedName{Name: componentMeta.Name, Namespace: componentMeta.Namespace}, stored))
	assert.Equal(t, "test-image:latest", stored.Spec.Template.Spec.Containers[0].Image,
		"the drift update itself must land")
	assert.Equal(t, lkgRecord, stored.Annotations[constants.AutoscalerPolicyLastRendered],
		"the LKG record must survive the drift update")
}
