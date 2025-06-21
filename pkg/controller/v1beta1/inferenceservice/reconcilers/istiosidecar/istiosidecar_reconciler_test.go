package istiosidecar

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockClient implements kclient.Client interface for testing error conditions
type mockClient struct {
	kclient.Client
	shouldErrorOnGet    bool
	shouldErrorOnCreate bool
	shouldErrorOnUpdate bool
}

func (m *mockClient) Get(ctx context.Context, key types.NamespacedName, obj kclient.Object, opts ...kclient.GetOption) error {
	if m.shouldErrorOnGet {
		return fmt.Errorf("mock get error")
	}
	return m.Client.Get(ctx, key, obj, opts...)
}

func (m *mockClient) Create(ctx context.Context, obj kclient.Object, opts ...kclient.CreateOption) error {
	if m.shouldErrorOnCreate {
		return fmt.Errorf("mock create error")
	}
	return m.Client.Create(ctx, obj, opts...)
}

func (m *mockClient) Update(ctx context.Context, obj kclient.Object, opts ...kclient.UpdateOption) error {
	if m.shouldErrorOnUpdate {
		return fmt.Errorf("mock update error")
	}
	return m.Client.Update(ctx, obj, opts...)
}

func TestCreateSidecar(t *testing.T) {
	tests := []struct {
		name          string
		componentMeta metav1.ObjectMeta
	}{
		{
			name: "default component",
			componentMeta: metav1.ObjectMeta{
				Name:      "test-isvc",
				Namespace: "default",
			},
		},
		{
			name: "component with labels",
			componentMeta: metav1.ObjectMeta{
				Name:      "test-isvc-with-labels",
				Namespace: "custom-namespace",
				Labels: map[string]string{
					"app": "test-isvc",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sidecar := createSidecar(tt.componentMeta)

			// Verify basic metadata
			assert.Equal(t, tt.componentMeta.Name, sidecar.Name)
			assert.Equal(t, tt.componentMeta.Namespace, sidecar.Namespace)

			// Verify egress configuration
			assert.Len(t, sidecar.Spec.Egress, 1)
			assert.Contains(t, sidecar.Spec.Egress[0].Hosts, "./*")

			// Verify port configuration
			portInt, _ := strconv.Atoi(constants.InferenceServiceDefaultHttpPort)
			assert.Equal(t, uint32(portInt), sidecar.Spec.Egress[0].Port.Number)
			assert.Equal(t, "HTTP", sidecar.Spec.Egress[0].Port.Protocol)

			// Verify ingress configuration
			assert.Len(t, sidecar.Spec.Ingress, 1)
			assert.Equal(t, uint32(portInt), sidecar.Spec.Ingress[0].Port.Number)
			assert.Equal(t, "HTTP", sidecar.Spec.Ingress[0].Port.Protocol)

			// Verify workload selector
			assert.Equal(t, tt.componentMeta.Name, sidecar.Spec.WorkloadSelector.Labels[constants.InferenceServiceLabel])
		})
	}
}

func TestNewIstioSidecarReconciler(t *testing.T) {
	// Setup test scheme
	scheme := runtime.NewScheme()
	err := istioclientv1beta1.AddToScheme(scheme)
	assert.NoError(t, err)

	// Create fake client
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	componentMeta := metav1.ObjectMeta{
		Name:      "test-isvc",
		Namespace: "default",
	}

	tests := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "enabled",
			enabled: true,
		},
		{
			name:    "disabled",
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, tt.enabled)

			// Verify reconciler is properly initialized
			assert.NotNil(t, reconciler)
			assert.Equal(t, client, reconciler.client)
			assert.Equal(t, scheme, reconciler.scheme)
			assert.Equal(t, tt.enabled, reconciler.enabled)

			// Verify sidecar is properly created
			assert.NotNil(t, reconciler.Sidecar)
			assert.Equal(t, componentMeta.Name, reconciler.Sidecar.Name)
			assert.Equal(t, componentMeta.Namespace, reconciler.Sidecar.Namespace)
		})
	}
}

func TestCheckSidecarExist(t *testing.T) {
	// Setup test scheme
	scheme := runtime.NewScheme()
	err := istioclientv1beta1.AddToScheme(scheme)
	assert.NoError(t, err)

	componentMeta := metav1.ObjectMeta{
		Name:      "test-isvc",
		Namespace: "default",
	}

	// 1. Test case: Sidecar doesn't exist (should return CheckResultCreate)
	t.Run("Sidecar doesn't exist", func(t *testing.T) {
		// Create a fake client without any existing sidecar
		client := fake.NewClientBuilder().WithScheme(scheme).Build()

		// Create a reconciler
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call checkSidecarExist
		result, sidecarObj, err := reconciler.checkSidecarExist(client)
		assert.NoError(t, err)
		assert.Equal(t, constants.CheckResultCreate, result)
		assert.Nil(t, sidecarObj)
	})

	// 2. Test case: Sidecar exists (should return CheckResultExisted)
	t.Run("Sidecar exists", func(t *testing.T) {
		// Create an existing sidecar
		existingSidecar := createSidecar(componentMeta)

		// Create a fake client with the existing sidecar
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSidecar).Build()

		// Create a reconciler
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call checkSidecarExist
		result, sidecarObj, err := reconciler.checkSidecarExist(client)
		assert.NoError(t, err)
		assert.Equal(t, constants.CheckResultExisted, result)
		assert.NotNil(t, sidecarObj)
		assert.Equal(t, existingSidecar.Name, sidecarObj.Name)
	})

	// 3. Test case: Error handling for client.Get failure
	t.Run("Get error", func(t *testing.T) {
		// Create a fake client with a custom client that will return error on Get
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		client := &mockClient{
			Client:           baseClient,
			shouldErrorOnGet: true,
		}

		// Create a reconciler
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call checkSidecarExist
		result, sidecarObj, err := reconciler.checkSidecarExist(client)
		assert.Error(t, err)
		assert.Equal(t, constants.CheckResultUnknown, result)
		assert.Nil(t, sidecarObj)
		assert.Contains(t, err.Error(), "mock get error")
	})

	// 4. Test case: Not found error but not a real error
	t.Run("Not found error", func(t *testing.T) {
		// Create a fake client that will return NotFound error
		// For simplicity, we'll just use a mockClient without modifying its Get method
		// and let checkSidecarExist handle the NotFound error
		client := fake.NewClientBuilder().WithScheme(scheme).Build()

		// Create a reconciler
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call checkSidecarExist - since the sidecar doesn't exist, it should return CheckResultCreate
		result, sidecarObj, err := reconciler.checkSidecarExist(client)
		assert.NoError(t, err)
		assert.Equal(t, constants.CheckResultCreate, result)
		assert.Nil(t, sidecarObj)
	})
}

func TestReconcile(t *testing.T) {
	// Setup test scheme
	scheme := runtime.NewScheme()
	err := istioclientv1beta1.AddToScheme(scheme)
	assert.NoError(t, err)

	componentMeta := metav1.ObjectMeta{
		Name:      "test-isvc",
		Namespace: "default",
	}

	// 1. Test case: Reconcile when Sidecar is disabled
	t.Run("Sidecar disabled", func(t *testing.T) {
		// Create a fake client
		client := fake.NewClientBuilder().WithScheme(scheme).Build()

		// Create a reconciler with enabled=false
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, false)

		// Call Reconcile
		result, err := reconciler.Reconcile()
		assert.NoError(t, err)
		assert.Nil(t, result)

		// Verify no sidecar was created
		sidecar := &istioclientv1beta1.Sidecar{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: componentMeta.Name, Namespace: componentMeta.Namespace}, sidecar)
		assert.Error(t, err)
		assert.True(t, apierr.IsNotFound(err))
	})

	// 2. Test case: Create a new Sidecar when it doesn't exist
	t.Run("Create new Sidecar", func(t *testing.T) {
		// Create a fake client without any existing sidecar
		client := fake.NewClientBuilder().WithScheme(scheme).Build()

		// Create a reconciler with enabled=true
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call Reconcile
		result, err := reconciler.Reconcile()
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Verify Sidecar was created
		verifySidecar := &istioclientv1beta1.Sidecar{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: componentMeta.Name, Namespace: componentMeta.Namespace}, verifySidecar)
		assert.NoError(t, err)
		assert.Equal(t, componentMeta.Name, verifySidecar.Name)
	})

	// 3. Test case: No changes needed when Sidecar already exists
	t.Run("No changes needed", func(t *testing.T) {
		// Create an existing sidecar
		existingSidecar := createSidecar(componentMeta)

		// Create a fake client with the existing sidecar
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSidecar).Build()

		// Create a reconciler with enabled=true
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call Reconcile
		result, err := reconciler.Reconcile()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, existingSidecar.Name, result.Name)
	})

	// 4. Test case: Error handling for client.Get failure
	t.Run("Get error", func(t *testing.T) {
		// Create a fake client with a custom client that will return error on Get
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		client := &mockClient{
			Client:           baseClient,
			shouldErrorOnGet: true,
		}

		// Create a reconciler with enabled=true
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call Reconcile
		result, err := reconciler.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "mock get error")
	})

	// 5. Test case: Error handling for client.Create failure
	t.Run("Create error", func(t *testing.T) {
		// Create a fake client with a custom client that will return error on Create
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		client := &mockClient{
			Client:              baseClient,
			shouldErrorOnCreate: true,
		}

		// Create a reconciler with enabled=true
		reconciler := NewIstioSidecarReconciler(client, scheme, componentMeta, true)

		// Call Reconcile
		result, err := reconciler.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "mock create error")
	})
}
