package external_service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestExternalServiceReconciler_shouldCreateExternalService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		isvc          *v1beta1.InferenceService
		ingressConfig *controllerconfig.IngressConfig
		expected      bool
		description   string
	}{
		{
			name: "should create when ingress disabled and has engine",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    true,
			description: "should create external service when ingress is disabled and engine component exists",
		},
		{
			name: "should create when ingress disabled and has router",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Router: &v1beta1.RouterSpec{},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    true,
			description: "should create external service when ingress is disabled and router component exists",
		},
		{
			name: "should create when ingress disabled and has predictor",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    true,
			description: "should create external service when ingress is disabled and predictor component exists",
		},
		{
			name: "should not create when ingress enabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: false,
			},
			expected:    false,
			description: "should not create external service when ingress is enabled",
		},
		{
			name: "should not create for cluster-local service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Labels: map[string]string{
						constants.VisibilityLabel: constants.ClusterLocalVisibility,
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    false,
			description: "should not create external service for cluster-local services",
		},
		{
			name: "should create for multinode deployment when ingress disabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Leader: &v1beta1.LeaderSpec{},
						Worker: &v1beta1.WorkerSpec{},
					},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    true,
			description: "should create external service for multinode deployments when ingress disabled",
		},
		{
			name: "should not create when no components",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			expected:    false,
			description: "should not create external service when no servable components exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ctrlclient.NewClientBuilder().WithScheme(scheme).Build()
			clientset := fake.NewSimpleClientset()

			reconciler := NewExternalServiceReconciler(client, clientset, scheme, tt.ingressConfig)
			result := reconciler.shouldCreateExternalService(tt.isvc)

			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestExternalServiceReconciler_determineTargetSelector(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name             string
		isvc             *v1beta1.InferenceService
		expectedSelector map[string]string
		description      string
	}{
		{
			name: "router component takes priority",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Router: &v1beta1.RouterSpec{},
					Engine: &v1beta1.EngineSpec{},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedSelector: map[string]string{
				constants.InferenceServicePodLabelKey: "test-service",
				constants.OMEComponentLabel:           string(v1beta1.RouterComponent),
			},
			description: "router component should be selected when multiple components exist",
		},
		{
			name: "engine component when no router",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedSelector: map[string]string{
				constants.InferenceServicePodLabelKey: "test-service",
				constants.OMEComponentLabel:           string(v1beta1.EngineComponent),
			},
			description: "engine component should be selected when router doesn't exist",
		},
		{
			name: "predictor component fallback",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedSelector: map[string]string{
				constants.InferenceServicePodLabelKey: "test-service",
				constants.OMEComponentLabel:           string(constants.Predictor),
			},
			description: "predictor component should be selected as fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ctrlclient.NewClientBuilder().WithScheme(scheme).Build()
			clientset := fake.NewSimpleClientset()
			ingressConfig := &controllerconfig.IngressConfig{}

			reconciler := NewExternalServiceReconciler(client, clientset, scheme, ingressConfig)
			result := reconciler.determineTargetSelector(tt.isvc)

			assert.Equal(t, tt.expectedSelector, result, tt.description)
		})
	}
}

func TestExternalServiceReconciler_buildExternalService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name        string
		isvc        *v1beta1.InferenceService
		description string
	}{
		{
			name: "basic service creation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			description: "should create basic external service",
		},
		{
			name: "service with LoadBalancer type annotation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Annotations: map[string]string{
						constants.ServiceType: "LoadBalancer",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			description: "should create LoadBalancer type external service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ctrlclient.NewClientBuilder().WithScheme(scheme).Build()
			clientset := fake.NewSimpleClientset()
			ingressConfig := &controllerconfig.IngressConfig{}

			reconciler := NewExternalServiceReconciler(client, clientset, scheme, ingressConfig)
			service, err := reconciler.buildExternalService(tt.isvc)

			assert.NoError(t, err, "should not return error when building external service")
			assert.NotNil(t, service, "external service should not be nil")
			assert.Equal(t, tt.isvc.Name, service.Name, "service name should match InferenceService name")
			assert.Equal(t, tt.isvc.Namespace, service.Namespace, "service namespace should match InferenceService namespace")

			// Check if service type is set correctly
			if serviceType, ok := tt.isvc.Annotations[constants.ServiceType]; ok && serviceType == "LoadBalancer" {
				assert.Equal(t, corev1.ServiceTypeLoadBalancer, service.Spec.Type, "service type should be LoadBalancer")
			} else {
				assert.Equal(t, corev1.ServiceTypeClusterIP, service.Spec.Type, "service type should default to ClusterIP")
			}

			// Check ports
			assert.Len(t, service.Spec.Ports, 1, "should have one port")
			assert.Equal(t, "http", service.Spec.Ports[0].Name, "port name should be http")
			assert.Equal(t, int32(80), service.Spec.Ports[0].Port, "port should be 80")
			assert.Equal(t, intstr.FromInt(8080), service.Spec.Ports[0].TargetPort, "target port should be 8080")
		})
	}
}

func TestExternalServiceReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		ingressConfig   *controllerconfig.IngressConfig
		existingService *corev1.Service
		expectedAction  string
		expectError     bool
		description     string
	}{
		{
			name: "create external service when ingress disabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			existingService: nil,
			expectedAction:  "create",
			expectError:     false,
			description:     "should create external service when none exists and ingress is disabled",
		},
		{
			name: "update external service when spec changes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Annotations: map[string]string{
						constants.ServiceType: "LoadBalancer",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Router: &v1beta1.RouterSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: true,
			},
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						constants.InferenceServicePodLabelKey: "test-service",
						constants.OMEComponentLabel:           string(v1beta1.EngineComponent),
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedAction: "update",
			expectError:    false,
			description:    "should update external service when spec changes",
		},
		{
			name: "delete external service when ingress enabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: false,
			},
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						constants.InferenceServicePodLabelKey: "test-service",
						constants.OMEComponentLabel:           string(v1beta1.EngineComponent),
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedAction: "delete",
			expectError:    false,
			description:    "should delete external service when ingress is enabled",
		},
		{
			name: "no action when ingress enabled and no service exists",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIngressCreation: false,
			},
			existingService: nil,
			expectedAction:  "none",
			expectError:     false,
			description:     "should take no action when ingress is enabled and no service exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.existingService != nil {
				objects = append(objects, tt.existingService)
			}

			client := ctrlclient.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			clientset := fake.NewSimpleClientset()

			reconciler := NewExternalServiceReconciler(client, clientset, scheme, tt.ingressConfig)

			err := reconciler.Reconcile(context.TODO(), tt.isvc)

			if tt.expectError {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")

				// Verify the expected action was performed
				service := &corev1.Service{}
				getErr := client.Get(context.TODO(), types.NamespacedName{
					Namespace: tt.isvc.Namespace,
					Name:      tt.isvc.Name,
				}, service)

				switch tt.expectedAction {
				case "create", "update":
					assert.NoError(t, getErr, "external service should exist after creation/update")
					assert.Equal(t, tt.isvc.Name, service.Name, "service name should match")
				case "delete", "none":
					assert.True(t, apierrors.IsNotFound(getErr), "external service should not exist after deletion or when not created")
				}
			}
		})
	}
}

func TestExternalServiceReconciler_getDeploymentMode(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		expectedMode constants.DeploymentModeType
		description  string
	}{
		{
			name: "multinode when leader and worker exist",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Leader: &v1beta1.LeaderSpec{},
						Worker: &v1beta1.WorkerSpec{},
					},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedMode: constants.MultiNode,
			description:  "should detect multinode deployment when both leader and worker exist",
		},
		{
			name: "multinode when only leader exists",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Leader: &v1beta1.LeaderSpec{},
					},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedMode: constants.MultiNode,
			description:  "should detect multinode deployment when only leader exists",
		},
		{
			name: "raw deployment for regular engine",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedMode: constants.RawDeployment,
			description:  "should detect raw deployment for regular engine without leader/worker",
		},
		{
			name: "raw deployment for predictor",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expectedMode: constants.RawDeployment,
			description:  "should detect raw deployment for predictor-only spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ctrlclient.NewClientBuilder().WithScheme(scheme).Build()
			clientset := fake.NewSimpleClientset()
			ingressConfig := &controllerconfig.IngressConfig{}

			reconciler := NewExternalServiceReconciler(client, clientset, scheme, ingressConfig)
			result := reconciler.getDeploymentMode(tt.isvc)

			assert.Equal(t, tt.expectedMode, result, tt.description)
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
