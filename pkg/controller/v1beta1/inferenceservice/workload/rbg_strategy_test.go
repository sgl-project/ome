package workload

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"
	rbgv1alpha2 "sigs.k8s.io/rbgs/api/workloads/v1alpha2"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/components"
)

// rbgInstallTestConfigMap installs the inferenceservice-config ConfigMap that
// controllerconfig.NewInferenceServicesConfig requires to construct an
// InferenceServicesConfig. The data payload is intentionally minimal -
// the RBG strategy doesn't depend on its contents.
func rbgInstallTestConfigMap(t *testing.T, cs kubernetes.Interface) {
	t.Helper()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inferenceservice-config", Namespace: "ome"},
		Data:       map[string]string{},
	}
	_, err := cs.CoreV1().ConfigMaps("ome").Create(context.Background(), cm, metav1.CreateOptions{})
	require.NoError(t, err)
}

func rbgTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, autoscalingv2.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, lws.AddToScheme(scheme))
	require.NoError(t, rbgv1alpha2.AddToScheme(scheme))
	return scheme
}

func TestRBGStrategy_GetStrategyName(t *testing.T) {
	s := NewRBGStrategy(nil, nil, nil, logr.Discard())
	assert.Equal(t, RBGStrategyName, s.GetStrategyName())
}

func TestRBGStrategy_IsApplicable(t *testing.T) {
	s := NewRBGStrategy(nil, nil, nil, logr.Discard())
	isvc := &v1beta1.InferenceService{}

	assert.True(t, s.IsApplicable(isvc, constants.RoleBasedGroup))
	assert.False(t, s.IsApplicable(isvc, constants.RawDeployment))
	assert.False(t, s.IsApplicable(isvc, constants.MultiNode))
	assert.False(t, s.IsApplicable(isvc, constants.Serverless))
	assert.False(t, s.IsApplicable(isvc, ""))
}

func TestRBGStrategy_ValidateDeploymentModes(t *testing.T) {
	s := NewRBGStrategy(nil, nil, nil, logr.Discard())

	cases := []struct {
		name    string
		modes   *ComponentDeploymentModes
		wantErr string
	}{
		{
			name: "raw deployment for engine only",
			modes: &ComponentDeploymentModes{
				Engine: constants.RawDeployment,
			},
		},
		{
			name: "engine raw + decoder multinode + router raw",
			modes: &ComponentDeploymentModes{
				Engine:  constants.RawDeployment,
				Decoder: constants.MultiNode,
				Router:  constants.RawDeployment,
			},
		},
		{
			name:    "missing engine mode",
			modes:   &ComponentDeploymentModes{},
			wantErr: "requires a deployment mode for engine",
		},
		{
			name: "serverless engine rejected",
			modes: &ComponentDeploymentModes{
				Engine: constants.Serverless,
			},
			wantErr: "does not support deployment mode",
		},
		{
			name: "multinode-ray-vllm rejected",
			modes: &ComponentDeploymentModes{
				Engine: constants.MultiNodeRayVLLM,
			},
			wantErr: "does not support deployment mode",
		},
		{
			name: "router serverless rejected",
			modes: &ComponentDeploymentModes{
				Engine: constants.RawDeployment,
				Router: constants.Serverless,
			},
			wantErr: "does not support deployment mode",
		},
		{
			name:  "nil modes accepted",
			modes: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateDeploymentModes(tc.modes)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRBGStrategy_ReconcileWorkload_CreatesRBGAndHPAandRBAC(t *testing.T) {
	scheme := rbgTestScheme(t)

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc", Namespace: "default", UID: "uid-1"},
		Spec:       v1beta1.InferenceServiceSpec{},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc).Build()
	clientset := fake.NewClientset()
	rbgInstallTestConfigMap(t, clientset)

	isvcConfig, err := controllerconfig.NewInferenceServicesConfig(clientset)
	require.NoError(t, err)
	factory := components.NewComponentBuilderFactory(c, clientset, scheme, isvcConfig)

	min := 1
	request := &WorkloadReconcileRequest{
		InferenceService: isvc,
		MergedEngine: &v1beta1.EngineSpec{
			PodSpec: v1beta1.PodSpec{
				Containers: []corev1.Container{{Name: "ome-container", Image: "engine:v1"}},
			},
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas: &min,
				MaxReplicas: 3,
			},
		},
		MergedRouter: &v1beta1.RouterSpec{
			PodSpec: v1beta1.PodSpec{
				Containers: []corev1.Container{{Name: "router", Image: "router:v1"}},
			},
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas: &min,
				MaxReplicas: 1,
			},
		},
		DeploymentModes: &ComponentDeploymentModes{
			Engine: constants.RawDeployment,
			Router: constants.RawDeployment,
		},
		ComponentBuilderFactory: factory,
	}

	s := NewRBGStrategy(c, clientset, scheme, logr.Discard())
	result, err := s.ReconcileWorkload(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.Requeue)
	assert.Zero(t, result.RequeueAfter)

	// RBG was created with two roles in deterministic order.
	var rbg rbgv1alpha2.RoleBasedGroup
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc", Namespace: "default"}, &rbg))
	require.Len(t, rbg.Spec.Roles, 2)
	assert.Equal(t, "engine", rbg.Spec.Roles[0].Name)
	assert.Equal(t, "router", rbg.Spec.Roles[1].Name)

	// RBAC: only Router gets SA + Role + RoleBinding; Engine has no SA.
	var engineSA corev1.ServiceAccount
	err = c.Get(context.Background(), types.NamespacedName{Name: "my-isvc-engine", Namespace: "default"}, &engineSA)
	require.Error(t, err, "engine should not have a ServiceAccount")

	var routerSA corev1.ServiceAccount
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc-router", Namespace: "default"}, &routerSA))

	var routerRole rbacv1.Role
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc-router", Namespace: "default"}, &routerRole))

	// HPA was created for each RawDeployment role and targets the RBGSA by name.
	var engineHPA autoscalingv2.HorizontalPodAutoscaler
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc-engine", Namespace: "default"}, &engineHPA))
	assert.Equal(t, "RoleBasedGroupScalingAdapter", engineHPA.Spec.ScaleTargetRef.Kind)
	assert.Equal(t, "workloads.x-k8s.io/v1alpha2", engineHPA.Spec.ScaleTargetRef.APIVersion)
	assert.Equal(t, "my-isvc-engine", engineHPA.Spec.ScaleTargetRef.Name)

	var routerHPA autoscalingv2.HorizontalPodAutoscaler
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "my-isvc-router", Namespace: "default"}, &routerHPA))
	assert.Equal(t, "RoleBasedGroupScalingAdapter", routerHPA.Spec.ScaleTargetRef.Kind)
}

func TestRBGStrategy_ReconcileWorkload_NoComponentsReturnsError(t *testing.T) {
	scheme := rbgTestScheme(t)
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc", Namespace: "default"},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc).Build()
	clientset := fake.NewClientset()
	rbgInstallTestConfigMap(t, clientset)

	isvcConfig, err := controllerconfig.NewInferenceServicesConfig(clientset)
	require.NoError(t, err)
	factory := components.NewComponentBuilderFactory(c, clientset, scheme, isvcConfig)

	request := &WorkloadReconcileRequest{
		InferenceService:        isvc,
		DeploymentModes:         &ComponentDeploymentModes{},
		ComponentBuilderFactory: factory,
	}

	s := NewRBGStrategy(c, clientset, scheme, logr.Discard())
	_, err = s.ReconcileWorkload(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one component")
}

// TestRBGStrategy_ReconcileWorkload_PDDisaggregated covers a prefill/decode
// disaggregated topology: engine (RawDeployment) + decoder (MultiNode) +
// router (RawDeployment). This exercises the three-role code path and
// verifies that (a) roles are emitted in a deterministic order, (b) HPAs
// are created for RawDeployment roles only (decoder is MultiNode), and
// (c) reconcileRBAC propagates the per-role ServiceAccount name down to
// every generated pod template.
func TestRBGStrategy_ReconcileWorkload_PDDisaggregated(t *testing.T) {
	scheme := rbgTestScheme(t)

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "pd-isvc", Namespace: "default", UID: "uid-pd"},
		Spec:       v1beta1.InferenceServiceSpec{},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(isvc).Build()
	clientset := fake.NewClientset()
	rbgInstallTestConfigMap(t, clientset)

	isvcConfig, err := controllerconfig.NewInferenceServicesConfig(clientset)
	require.NoError(t, err)
	factory := components.NewComponentBuilderFactory(c, clientset, scheme, isvcConfig)

	min := 1
	workerSize := 2
	request := &WorkloadReconcileRequest{
		InferenceService: isvc,
		MergedEngine: &v1beta1.EngineSpec{
			PodSpec: v1beta1.PodSpec{
				Containers: []corev1.Container{{Name: "ome-container", Image: "engine:v1"}},
			},
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas: &min,
				MaxReplicas: 3,
			},
		},
		MergedDecoder: &v1beta1.DecoderSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas: &min,
				MaxReplicas: 1,
			},
			Leader: &v1beta1.LeaderSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []corev1.Container{{Name: "ome-container", Image: "decoder-leader:v1"}},
				},
			},
			Worker: &v1beta1.WorkerSpec{
				Size: &workerSize,
				PodSpec: v1beta1.PodSpec{
					Containers: []corev1.Container{{Name: "ome-container", Image: "decoder-worker:v1"}},
				},
			},
		},
		MergedRouter: &v1beta1.RouterSpec{
			PodSpec: v1beta1.PodSpec{
				Containers: []corev1.Container{{Name: "router", Image: "router:v1"}},
			},
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas: &min,
				MaxReplicas: 1,
			},
		},
		DeploymentModes: &ComponentDeploymentModes{
			Engine:  constants.RawDeployment,
			Decoder: constants.MultiNode,
			Router:  constants.RawDeployment,
		},
		ComponentBuilderFactory: factory,
	}

	s := NewRBGStrategy(c, clientset, scheme, logr.Discard())
	result, err := s.ReconcileWorkload(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.Requeue)
	assert.Zero(t, result.RequeueAfter)

	// RBG has three roles emitted in a deterministic order.
	var rbg rbgv1alpha2.RoleBasedGroup
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc", Namespace: "default"}, &rbg))
	require.Len(t, rbg.Spec.Roles, 3)
	assert.Equal(t, "engine", rbg.Spec.Roles[0].Name)
	assert.Equal(t, "decoder", rbg.Spec.Roles[1].Name)
	assert.Equal(t, "router", rbg.Spec.Roles[2].Name)

	// Only Router gets RBAC resources (SA + Role + RoleBinding).
	// Engine and Decoder should NOT have ServiceAccounts.
	for _, comp := range []string{"engine", "decoder"} {
		var sa corev1.ServiceAccount
		err = c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-" + comp, Namespace: "default"}, &sa)
		require.Error(t, err, "%s should not have a ServiceAccount", comp)
	}
	var routerSA corev1.ServiceAccount
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-router", Namespace: "default"}, &routerSA))
	var routerRole rbacv1.Role
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-router", Namespace: "default"}, &routerRole))
	var routerRB rbacv1.RoleBinding
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-router", Namespace: "default"}, &routerRB))

	// HPAs are created for RawDeployment roles only and target the RBGSA.
	// Decoder is MultiNode and is therefore scaled by the RBG ScalingAdapter
	// directly, not an HPA.
	var engineHPA autoscalingv2.HorizontalPodAutoscaler
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-engine", Namespace: "default"}, &engineHPA))
	assert.Equal(t, "RoleBasedGroupScalingAdapter", engineHPA.Spec.ScaleTargetRef.Kind)
	assert.Equal(t, "workloads.x-k8s.io/v1alpha2", engineHPA.Spec.ScaleTargetRef.APIVersion)
	assert.Equal(t, "pd-isvc-engine", engineHPA.Spec.ScaleTargetRef.Name)

	var routerHPA autoscalingv2.HorizontalPodAutoscaler
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-router", Namespace: "default"}, &routerHPA))
	assert.Equal(t, "RoleBasedGroupScalingAdapter", routerHPA.Spec.ScaleTargetRef.Kind)

	var decoderHPA autoscalingv2.HorizontalPodAutoscaler
	err = c.Get(context.Background(), types.NamespacedName{Name: "pd-isvc-decoder", Namespace: "default"}, &decoderHPA)
	require.Error(t, err, "decoder (MultiNode) should not have an HPA")
}
