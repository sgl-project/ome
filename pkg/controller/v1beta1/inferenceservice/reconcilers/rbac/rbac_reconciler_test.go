package rbac

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

const (
	testNamespace        = "test-namespace"
	testInferenceService = "test-inference-service"
	testServiceName      = "test-service"
)

func TestNewRBACReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	objectMeta := metav1.ObjectMeta{
		Name:      testServiceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app": "test-app",
		},
	}

	tests := []struct {
		name             string
		client           client.Client
		scheme           *runtime.Scheme
		objectMeta       metav1.ObjectMeta
		componentType    v1beta1.ComponentType
		inferenceService string
		expectError      bool
	}{
		{
			name:             "valid inputs",
			client:           fakeClient,
			scheme:           scheme,
			objectMeta:       objectMeta,
			componentType:    v1beta1.RouterComponent,
			inferenceService: testInferenceService,
			expectError:      false,
		},
		{
			name:             "valid with different component type",
			client:           fakeClient,
			scheme:           scheme,
			objectMeta:       objectMeta,
			componentType:    v1beta1.PredictorComponent,
			inferenceService: testInferenceService,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := NewRBACReconciler(
				tt.client,
				tt.scheme,
				tt.objectMeta,
				tt.componentType,
				tt.inferenceService,
			)

			assert.NotNil(t, reconciler)
			assert.Equal(t, tt.client, reconciler.client)
			assert.Equal(t, tt.scheme, reconciler.scheme)
			assert.Equal(t, tt.objectMeta, reconciler.objectMeta)
			assert.Equal(t, tt.componentType, reconciler.componentType)
			assert.Equal(t, tt.inferenceService, reconciler.inferenceService)
			assert.NotNil(t, reconciler.Log)
		})
	}
}

func TestRBACReconciler_GetServiceAccountName(t *testing.T) {
	tests := []struct {
		name             string
		inferenceService string
		componentType    v1beta1.ComponentType
		expected         string
	}{
		{
			name:             "router component",
			inferenceService: "my-service",
			componentType:    v1beta1.RouterComponent,
			expected:         "my-service-router",
		},
		{
			name:             "predictor component",
			inferenceService: "inference-svc",
			componentType:    v1beta1.PredictorComponent,
			expected:         "inference-svc-predictor",
		},
		{
			name:             "decoder component",
			inferenceService: "test-svc",
			componentType:    v1beta1.DecoderComponent,
			expected:         "test-svc-decoder",
		},
		{
			name:             "engine component",
			inferenceService: "engine-svc",
			componentType:    v1beta1.EngineComponent,
			expected:         "engine-svc-engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &RBACReconciler{
				inferenceService: tt.inferenceService,
				componentType:    tt.componentType,
			}

			result := reconciler.GetServiceAccountName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBACReconciler_Reconcile_RouterComponent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme))

	ownerRef := metav1.OwnerReference{
		APIVersion: "ome.sgl-project.io/v1beta1",
		Kind:       "InferenceService",
		Name:       testInferenceService,
		UID:        "test-uid",
	}

	objectMeta := metav1.ObjectMeta{
		Name:      testServiceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app": "test-app",
		},
		OwnerReferences: []metav1.OwnerReference{ownerRef},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewRBACReconciler(
		fakeClient,
		scheme,
		objectMeta,
		v1beta1.RouterComponent,
		testInferenceService,
	)

	// Test successful reconciliation
	err := reconciler.Reconcile()
	require.NoError(t, err)

	expectedServiceAccountName := "test-inference-service-router"

	// Verify ServiceAccount was created
	sa := &corev1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, sa)
	require.NoError(t, err)
	assert.Equal(t, expectedServiceAccountName, sa.Name)
	assert.Equal(t, testNamespace, sa.Namespace)
	assert.Equal(t, objectMeta.Labels, sa.Labels)
	assert.Equal(t, []metav1.OwnerReference{ownerRef}, sa.OwnerReferences)

	// Verify Role was created
	role := &rbacv1.Role{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, role)
	require.NoError(t, err)
	assert.Equal(t, expectedServiceAccountName, role.Name)
	assert.Equal(t, testNamespace, role.Namespace)
	assert.Equal(t, objectMeta.Labels, role.Labels)
	assert.Equal(t, []metav1.OwnerReference{ownerRef}, role.OwnerReferences)

	// Verify Role rules
	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	assert.Equal(t, expectedRules, role.Rules)

	// Verify RoleBinding was created
	rb := &rbacv1.RoleBinding{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, rb)
	require.NoError(t, err)
	assert.Equal(t, expectedServiceAccountName, rb.Name)
	assert.Equal(t, testNamespace, rb.Namespace)
	assert.Equal(t, objectMeta.Labels, rb.Labels)
	assert.Equal(t, []metav1.OwnerReference{ownerRef}, rb.OwnerReferences)

	// Verify RoleBinding subjects and roleRef
	expectedSubjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      expectedServiceAccountName,
			Namespace: testNamespace,
		},
	}
	assert.Equal(t, expectedSubjects, rb.Subjects)

	expectedRoleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     expectedServiceAccountName,
	}
	assert.Equal(t, expectedRoleRef, rb.RoleRef)
}

func TestRBACReconciler_Reconcile_NonRouterComponent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme))

	objectMeta := metav1.ObjectMeta{
		Name:      testServiceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app": "test-app",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewRBACReconciler(
		fakeClient,
		scheme,
		objectMeta,
		v1beta1.PredictorComponent, // Non-router component
		testInferenceService,
	)

	// Test successful reconciliation
	err := reconciler.Reconcile()
	require.NoError(t, err)

	expectedServiceAccountName := "test-inference-service-predictor"

	// Verify ServiceAccount was created
	sa := &corev1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, sa)
	require.NoError(t, err)

	// Verify Role was NOT created for non-router component
	role := &rbacv1.Role{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, role)
	assert.True(t, apierrors.IsNotFound(err), "Role should not exist for non-router component")

	// Verify RoleBinding was NOT created for non-router component
	rb := &rbacv1.RoleBinding{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, rb)
	assert.True(t, apierrors.IsNotFound(err), "RoleBinding should not exist for non-router component")
}

func TestRBACReconciler_Reconcile_Update(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme))

	objectMeta := metav1.ObjectMeta{
		Name:      testServiceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app": "test-app",
		},
	}

	expectedServiceAccountName := "test-inference-service-router"

	// Pre-create resources with different labels
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedServiceAccountName,
			Namespace: testNamespace,
			Labels: map[string]string{
				"old-label": "old-value",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingSA).
		Build()

	reconciler := NewRBACReconciler(
		fakeClient,
		scheme,
		objectMeta,
		v1beta1.RouterComponent,
		testInferenceService,
	)

	// Test reconciliation updates existing resources
	err := reconciler.Reconcile()
	require.NoError(t, err)

	// Verify ServiceAccount was updated with new labels
	sa := &corev1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      expectedServiceAccountName,
		Namespace: testNamespace,
	}, sa)
	require.NoError(t, err)
	assert.Equal(t, objectMeta.Labels, sa.Labels)
}

func TestRBACReconciler_createOrUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RBACReconciler{
		client: fakeClient,
		Log:    logr.Discard(),
	}

	// Test creating new resource
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: testNamespace,
		},
	}

	err := reconciler.createOrUpdate(sa)
	require.NoError(t, err)

	// Verify resource was created
	createdSA := &corev1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-sa",
		Namespace: testNamespace,
	}, createdSA)
	require.NoError(t, err)
	assert.Equal(t, "test-sa", createdSA.Name)

	// Test updating existing resource
	sa.Labels = map[string]string{"updated": "true"}
	err = reconciler.createOrUpdate(sa)
	require.NoError(t, err)

	// Verify resource was updated
	updatedSA := &corev1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-sa",
		Namespace: testNamespace,
	}, updatedSA)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"updated": "true"}, updatedSA.Labels)
}

func TestRBACReconciler_createOrUpdate_Error(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	// Create a failing client that returns errors
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &RBACReconciler{
		client: fakeClient,
		Log:    logr.Discard(),
	}

	// Test with invalid resource (missing required fields)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "", // Invalid empty name
		},
	}

	err := reconciler.createOrUpdate(sa)
	assert.Error(t, err)
}

// Benchmark tests
func BenchmarkRBACReconciler_Reconcile(b *testing.B) {
	scheme := runtime.NewScheme()
	require.NoError(b, clientgoscheme.AddToScheme(scheme))
	require.NoError(b, v1beta1.AddToScheme(scheme))

	objectMeta := metav1.ObjectMeta{
		Name:      testServiceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app": "test-app",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewRBACReconciler(
		fakeClient,
		scheme,
		objectMeta,
		v1beta1.RouterComponent,
		testInferenceService,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := reconciler.Reconcile()
		require.NoError(b, err)
	}
}
