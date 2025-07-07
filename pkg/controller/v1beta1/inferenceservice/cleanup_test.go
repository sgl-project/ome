package inferenceservice

import (
	"context"
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCleanupRemovedComponents(t *testing.T) {
	testCases := []struct {
		name               string
		isvc               *v1beta1.InferenceService
		existingResources  []client.Object
		engineSpec         *v1beta1.EngineSpec
		decoderSpec        *v1beta1.DecoderSpec
		routerSpec         *v1beta1.RouterSpec
		expectedDeleted    []string
		expectedNotDeleted []string
	}{
		{
			name: "Delete orphaned decoder resources when decoder removed from spec",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			existingResources: []client.Object{
				createDeployment("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createDeployment("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createService("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createService("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
			},
			engineSpec:         &v1beta1.EngineSpec{},
			decoderSpec:        nil, // Decoder removed
			routerSpec:         nil,
			expectedDeleted:    []string{"test-isvc-decoder"},
			expectedNotDeleted: []string{"test-isvc-engine"},
		},
		{
			name: "Delete orphaned router resources when router removed from spec",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			existingResources: []client.Object{
				createDeployment("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createDeployment("test-isvc-router", "default", "test-isvc", "test-uid", v1beta1.RouterComponent),
				createService("test-isvc-router", "default", "test-isvc", "test-uid", v1beta1.RouterComponent),
				createHPA("test-isvc-router-hpa", "default", "test-isvc", "test-uid", v1beta1.RouterComponent),
			},
			engineSpec:         &v1beta1.EngineSpec{},
			decoderSpec:        nil,
			routerSpec:         nil, // Router removed
			expectedDeleted:    []string{"test-isvc-router", "test-isvc-router-hpa"},
			expectedNotDeleted: []string{"test-isvc-engine"},
		},
		{
			name: "Keep all resources when all components are active",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			existingResources: []client.Object{
				createDeployment("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createDeployment("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createDeployment("test-isvc-router", "default", "test-isvc", "test-uid", v1beta1.RouterComponent),
			},
			engineSpec:         &v1beta1.EngineSpec{},
			decoderSpec:        &v1beta1.DecoderSpec{},
			routerSpec:         &v1beta1.RouterSpec{},
			expectedDeleted:    []string{},
			expectedNotDeleted: []string{"test-isvc-engine", "test-isvc-decoder", "test-isvc-router"},
		},
		{
			name: "Only delete resources with correct owner reference",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			existingResources: []client.Object{
				createDeployment("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createDeployment("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createDeploymentWithoutOwner("test-isvc-decoder-no-owner", "default", "test-isvc", v1beta1.DecoderComponent),
			},
			engineSpec:         &v1beta1.EngineSpec{},
			decoderSpec:        nil, // Decoder removed
			routerSpec:         nil,
			expectedDeleted:    []string{"test-isvc-decoder"},
			expectedNotDeleted: []string{"test-isvc-engine", "test-isvc-decoder-no-owner"},
		},
		{
			name: "Handle multiple resource types correctly",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			existingResources: []client.Object{
				createDeployment("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createService("test-isvc-engine", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createConfigMap("test-isvc-engine-config", "default", "test-isvc", "test-uid", v1beta1.EngineComponent),
				createDeployment("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createService("test-isvc-decoder", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createConfigMap("test-isvc-decoder-config", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
				createIngress("test-isvc-decoder-ingress", "default", "test-isvc", "test-uid", v1beta1.DecoderComponent),
			},
			engineSpec:         &v1beta1.EngineSpec{},
			decoderSpec:        nil, // Decoder removed
			routerSpec:         nil,
			expectedDeleted:    []string{"test-isvc-decoder", "test-isvc-decoder-config", "test-isvc-decoder-ingress"},
			expectedNotDeleted: []string{"test-isvc-engine", "test-isvc-engine-config"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = log.IntoContext(ctx, log.Log)

			// Create scheme
			scheme := runtime.NewScheme()
			_ = v1beta1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			_ = autoscalingv2.AddToScheme(scheme)
			_ = networkingv1.AddToScheme(scheme)

			// Create fake client with existing resources
			allObjects := append([]client.Object{tc.isvc}, tc.existingResources...)
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(allObjects...).
				Build()

			// Create reconciler
			r := &InferenceServiceReconciler{
				Client:    fakeClient,
				Clientset: fake.NewSimpleClientset(),
			}

			// Execute cleanup
			err := r.cleanupRemovedComponents(ctx, tc.isvc, tc.engineSpec, tc.decoderSpec, tc.routerSpec)
			require.NoError(t, err)

			// Verify expected deletions
			for _, name := range tc.expectedDeleted {
				// Check various resource types
				checkResourceDeleted(t, ctx, fakeClient, name, tc.isvc.Namespace)
			}

			// Verify expected resources still exist
			for _, name := range tc.expectedNotDeleted {
				checkResourceExists(t, ctx, fakeClient, name, tc.isvc.Namespace)
			}
		})
	}
}

func TestIsOwnedBy(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-isvc",
			Namespace: "default",
			UID:       "test-uid",
		},
	}

	testCases := []struct {
		name          string
		obj           *unstructured.Unstructured
		expectedOwned bool
	}{
		{
			name: "Object owned by InferenceService",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"kind":       "InferenceService",
								"apiVersion": v1beta1.SchemeGroupVersion.String(),
								"name":       "test-isvc",
								"uid":        "test-uid",
							},
						},
					},
				},
			},
			expectedOwned: true,
		},
		{
			name: "Object with wrong UID",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"kind":       "InferenceService",
								"apiVersion": v1beta1.SchemeGroupVersion.String(),
								"name":       "test-isvc",
								"uid":        "wrong-uid",
							},
						},
					},
				},
			},
			expectedOwned: false,
		},
		{
			name: "Object with wrong kind",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"kind":       "Deployment",
								"apiVersion": "apps/v1",
								"name":       "test-isvc",
								"uid":        "test-uid",
							},
						},
					},
				},
			},
			expectedOwned: false,
		},
		{
			name: "Object with no owner references",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
			expectedOwned: false,
		},
	}

	r := &InferenceServiceReconciler{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owned := r.isOwnedBy(tc.obj, isvc)
			assert.Equal(t, tc.expectedOwned, owned)
		})
	}
}

func TestGetAvailableResourceTypes(t *testing.T) {
	r := &InferenceServiceReconciler{
		ClientConfig: nil, // No config means only core resources
	}

	gvks, err := r.getAvailableResourceTypes()
	require.NoError(t, err)

	// Should contain at least the core resource types
	expectedCoreTypes := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"},
		{Group: "", Version: "v1", Kind: "ServiceAccount"},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
	}

	for _, expected := range expectedCoreTypes {
		found := false
		for _, gvk := range gvks {
			if gvk == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected GVK %v not found in available types", expected)
	}
}

func TestContains(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "Item exists in slice",
			slice:    []string{"list", "get", "delete"},
			item:     "delete",
			expected: true,
		},
		{
			name:     "Item does not exist in slice",
			slice:    []string{"list", "get"},
			item:     "delete",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			item:     "delete",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := contains(tc.slice, tc.item)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Helper functions to create test resources

func createDeployment(name, namespace, isvcName, uid string, component v1beta1.ComponentType) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "InferenceService",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Name:       isvcName,
					UID:        types.UID(uid),
				},
			},
		},
	}
}

func createDeploymentWithoutOwner(name, namespace, isvcName string, component v1beta1.ComponentType) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
		},
	}
}

func createService(name, namespace, isvcName, uid string, component v1beta1.ComponentType) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "InferenceService",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Name:       isvcName,
					UID:        types.UID(uid),
				},
			},
		},
	}
}

func createConfigMap(name, namespace, isvcName, uid string, component v1beta1.ComponentType) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "InferenceService",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Name:       isvcName,
					UID:        types.UID(uid),
				},
			},
		},
	}
}

func createHPA(name, namespace, isvcName, uid string, component v1beta1.ComponentType) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "InferenceService",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Name:       isvcName,
					UID:        types.UID(uid),
				},
			},
		},
	}
}

func createIngress(name, namespace, isvcName, uid string, component v1beta1.ComponentType) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "InferenceService",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Name:       isvcName,
					UID:        types.UID(uid),
				},
			},
		},
	}
}

// Helper functions to check resource existence

func checkResourceDeleted(t *testing.T, ctx context.Context, client client.Client, name, namespace string) {
	// Try multiple resource types
	deployment := &appsv1.Deployment{}
	err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment)
	if err == nil {
		t.Errorf("Expected deployment %s to be deleted, but it still exists", name)
		return
	}

	service := &corev1.Service{}
	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, service)
	if err == nil {
		t.Errorf("Expected service %s to be deleted, but it still exists", name)
		return
	}

	configMap := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, configMap)
	if err == nil {
		t.Errorf("Expected configmap %s to be deleted, but it still exists", name)
		return
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, hpa)
	if err == nil {
		t.Errorf("Expected HPA %s to be deleted, but it still exists", name)
		return
	}

	ingress := &networkingv1.Ingress{}
	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ingress)
	if err == nil {
		t.Errorf("Expected ingress %s to be deleted, but it still exists", name)
		return
	}
}

func checkResourceExists(t *testing.T, ctx context.Context, client client.Client, name, namespace string) {
	// Check if at least one resource type exists
	found := false

	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment); err == nil {
		found = true
	}

	service := &corev1.Service{}
	if err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, service); err == nil {
		found = true
	}

	configMap := &corev1.ConfigMap{}
	if err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, configMap); err == nil {
		found = true
	}

	if !found {
		t.Errorf("Expected resource %s to exist, but it was not found", name)
	}
}
