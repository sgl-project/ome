package rbg

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	rbgv1alpha2 "sigs.k8s.io/rbgs/api/workloads/v1alpha2"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/components"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbgv1alpha2.AddToScheme(scheme))
	return scheme
}

func newTestISVC() *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-isvc",
			Namespace: "ns",
			UID:       "uid-1",
		},
	}
}

func enginePodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "engine", Image: "engine:v1"},
		},
	}
}

func workerPodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "engine", Image: "engine-worker:v1"},
		},
	}
}

func TestBuildRole_RawDeployment(t *testing.T) {
	min := 2
	cfg := &components.RoleConfig{
		ComponentType:  v1beta1.EngineComponent,
		DeploymentMode: constants.RawDeployment,
		PodSpec:        enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{
			MinReplicas: &min,
			MaxReplicas: 5,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-isvc-engine",
			Namespace:   "ns",
			Labels:      map[string]string{"app": "engine"},
			Annotations: map[string]string{"foo": "bar"},
		},
	}

	role, err := buildRole(cfg)
	require.NoError(t, err)

	assert.Equal(t, "engine", role.Name)
	require.NotNil(t, role.Replicas)
	assert.Equal(t, int32(2), *role.Replicas)
	assert.Equal(t, "bar", role.Annotations["foo"])
	assert.Equal(t, "engine", role.Labels[constants.OMEComponentLabel])

	require.NotNil(t, role.ScalingAdapter, "RawDeployment role must enable ScalingAdapter")
	assert.True(t, role.ScalingAdapter.Enable)

	require.NotNil(t, role.StandalonePattern)
	require.Nil(t, role.LeaderWorkerPattern)
	require.NotNil(t, role.StandalonePattern.Template)
	assert.Equal(t, "engine:v1", role.StandalonePattern.Template.Spec.Containers[0].Image)
	assert.Equal(t, "engine", role.StandalonePattern.Template.Labels[constants.OMEComponentLabel])
}

func TestBuildRole_MultiNode(t *testing.T) {
	cfg := &components.RoleConfig{
		ComponentType:          v1beta1.DecoderComponent,
		DeploymentMode:         constants.MultiNode,
		PodSpec:                enginePodSpec(),
		LeaderPodSpec:          enginePodSpec(),
		WorkerPodSpec:          workerPodSpec(),
		WorkerSize:             3,
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-isvc-decoder",
			Namespace: "ns",
		},
	}

	role, err := buildRole(cfg)
	require.NoError(t, err)

	assert.Equal(t, "decoder", role.Name)
	assert.Nil(t, role.ScalingAdapter, "MultiNode role should not enable ScalingAdapter")

	require.Nil(t, role.StandalonePattern)
	require.NotNil(t, role.LeaderWorkerPattern)
	require.NotNil(t, role.LeaderWorkerPattern.Size)
	assert.Equal(t, int32(4), *role.LeaderWorkerPattern.Size, "size must be 1 leader + N workers")

	require.NotNil(t, role.LeaderWorkerPattern.Template)
	assert.Equal(t, "engine:v1", role.LeaderWorkerPattern.Template.Spec.Containers[0].Image)

	require.NotNil(t, role.LeaderWorkerPattern.WorkerTemplatePatch)
	var patch corev1.PodTemplateSpec
	require.NoError(t, json.Unmarshal(role.LeaderWorkerPattern.WorkerTemplatePatch.Raw, &patch))
	assert.Equal(t, "engine-worker:v1", patch.Spec.Containers[0].Image)
}

func TestBuildRole_DefaultsReplicasWhenMinReplicasMissing(t *testing.T) {
	cfg := &components.RoleConfig{
		ComponentType:          v1beta1.RouterComponent,
		DeploymentMode:         constants.RawDeployment,
		PodSpec:                enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{},
		ObjectMeta:             metav1.ObjectMeta{Name: "my-isvc-router", Namespace: "ns"},
	}
	role, err := buildRole(cfg)
	require.NoError(t, err)
	require.NotNil(t, role.Replicas)
	assert.Equal(t, int32(1), *role.Replicas)
}

func TestBuildRole_RejectsUnsupportedDeploymentMode(t *testing.T) {
	cfg := &components.RoleConfig{
		ComponentType:          v1beta1.EngineComponent,
		DeploymentMode:         constants.Serverless,
		PodSpec:                enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{},
		ObjectMeta:             metav1.ObjectMeta{Name: "x", Namespace: "ns"},
	}
	_, err := buildRole(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported deployment mode")
}

func TestReconcile_CreatesNewRBG(t *testing.T) {
	scheme := newTestScheme(t)
	isvc := newTestISVC()
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc).Build()
	r := NewRBGReconciler(c, scheme, logr.Discard())

	cfg := &components.RoleConfig{
		ComponentType:          v1beta1.EngineComponent,
		DeploymentMode:         constants.RawDeployment,
		PodSpec:                enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{},
		ObjectMeta:             metav1.ObjectMeta{Name: "my-isvc-engine", Namespace: "ns"},
	}

	_, err := r.Reconcile(context.Background(), isvc, []*components.RoleConfig{cfg})
	require.NoError(t, err)

	var got rbgv1alpha2.RoleBasedGroup
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc", Namespace: "ns"}, &got))
	require.Len(t, got.Spec.Roles, 1)
	assert.Equal(t, "engine", got.Spec.Roles[0].Name)

	require.Len(t, got.OwnerReferences, 1)
	assert.Equal(t, "my-isvc", got.OwnerReferences[0].Name)
}

func TestReconcile_PreservesReplicasOnUpdate(t *testing.T) {
	scheme := newTestScheme(t)
	isvc := newTestISVC()

	existingReplicas := int32(7)
	existing := &rbgv1alpha2.RoleBasedGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc", Namespace: "ns"},
		Spec: rbgv1alpha2.RoleBasedGroupSpec{
			Roles: []rbgv1alpha2.RoleSpec{
				{Name: "engine", Replicas: &existingReplicas},
			},
		},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc, existing).Build()
	r := NewRBGReconciler(c, scheme, logr.Discard())

	min := 2
	cfg := &components.RoleConfig{
		ComponentType:  v1beta1.EngineComponent,
		DeploymentMode: constants.RawDeployment,
		PodSpec:        enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{
			MinReplicas: &min,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc-engine", Namespace: "ns"},
	}

	_, err := r.Reconcile(context.Background(), isvc, []*components.RoleConfig{cfg})
	require.NoError(t, err)

	var got rbgv1alpha2.RoleBasedGroup
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc", Namespace: "ns"}, &got))
	require.Len(t, got.Spec.Roles, 1)
	require.NotNil(t, got.Spec.Roles[0].Replicas)
	assert.Equal(t, int32(7), *got.Spec.Roles[0].Replicas, "replicas from existing RBG must be preserved on update")
}

func TestReconcile_ErrorWhenNoConfigs(t *testing.T) {
	scheme := newTestScheme(t)
	isvc := newTestISVC()
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc).Build()
	r := NewRBGReconciler(c, scheme, logr.Discard())

	_, err := r.Reconcile(context.Background(), isvc, nil)
	require.Error(t, err)
}

func TestReconcile_MergesLabelsOnUpdate(t *testing.T) {
	scheme := newTestScheme(t)
	isvc := newTestISVC()

	existing := &rbgv1alpha2.RoleBasedGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-isvc",
			Namespace: "ns",
			Labels: map[string]string{
				"controller-managed":              "true",
				"serving.ome.io/inferenceservice": "my-isvc",
			},
		},
		Spec: rbgv1alpha2.RoleBasedGroupSpec{
			Roles: []rbgv1alpha2.RoleSpec{{Name: "engine"}},
		},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc, existing).Build()
	r := NewRBGReconciler(c, scheme, logr.Discard())

	cfg := &components.RoleConfig{
		ComponentType:          v1beta1.EngineComponent,
		DeploymentMode:         constants.RawDeployment,
		PodSpec:                enginePodSpec(),
		ComponentExtensionSpec: &v1beta1.ComponentExtensionSpec{},
		ObjectMeta:             metav1.ObjectMeta{Name: "my-isvc-engine", Namespace: "ns"},
	}

	_, err := r.Reconcile(context.Background(), isvc, []*components.RoleConfig{cfg})
	require.NoError(t, err)

	var got rbgv1alpha2.RoleBasedGroup
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc", Namespace: "ns"}, &got))

	assert.Equal(t, "my-isvc", got.Labels["serving.ome.io/inferenceservice"],
		"desired label must be present")
	assert.Equal(t, "true", got.Labels["controller-managed"],
		"pre-existing label not in desired set must be preserved")
}
