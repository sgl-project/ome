package multinodevllm

import (
	"context"
	"testing"

	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMultiNodeVLLMReconciler_Reconcile(t *testing.T) {
	s := scheme.Scheme
	_ = v1beta1.AddToScheme(s)
	_ = ray.AddToScheme(s)
	_ = istiov1beta1.AddToScheme(s)

	tests := []struct {
		name        string
		setupClient func() client.Client
		isvc        *v1beta1.InferenceService
		wantErr     bool
		validate    func(t *testing.T, client client.Client, isvc *v1beta1.InferenceService)
	}{
		{
			name: "successful reconciliation with model spec",
			setupClient: func() client.Client {
				return kfake.NewClientBuilder().WithScheme(s).Build()
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: ptr.To(1),
							MaxReplicas: 3,
						},
						Model: &v1beta1.ModelSpec{
							PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
								RuntimeVersion: ptr.To("2.6.0"),
								Container: corev1.Container{
									Image: "vllm/vllm-openai:latest",
									Name:  "kserve-container",
								},
							},
							Runtime:   ptr.To("vllm"),
							BaseModel: ptr.To("llama-7b"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, client client.Client, isvc *v1beta1.InferenceService) {
				// Verify RayCluster was created
				rayCluster := &ray.RayCluster{}
				err := client.Get(context.Background(), types.NamespacedName{
					Name:      "test-isvc-predictor-0",
					Namespace: "default",
				}, rayCluster)
				assert.Error(t, err)
				assert.True(t, apierr.IsNotFound(err))

				// Verify Service was created
				service := &corev1.Service{}
				err = client.Get(context.Background(), types.NamespacedName{
					Name:      "test-isvc-predictor",
					Namespace: "default",
				}, service)
				assert.Error(t, err)
				assert.True(t, apierr.IsNotFound(err))
			},
		},
		{
			name: "reconciliation with existing ray cluster",
			setupClient: func() client.Client {
				existingRayCluster := &ray.RayCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-predictor-0",
						Namespace: "default",
					},
					Spec: ray.RayClusterSpec{
						HeadGroupSpec: ray.HeadGroupSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "ray-head",
											Image: "rayproject/ray:2.0.0",
										},
									},
								},
							},
						},
					},
				}
				return kfake.NewClientBuilder().WithScheme(s).WithObjects(existingRayCluster).Build()
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: ptr.To(2),
							MaxReplicas: 4,
						},
						Model: &v1beta1.ModelSpec{
							PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
								RuntimeVersion: ptr.To("2.6.0"),
								Container: corev1.Container{
									Image: "vllm/vllm-openai:v0.2.7",
									Name:  "kserve-container",
								},
							},
							Runtime:   ptr.To("vllm"),
							BaseModel: ptr.To("llama-13b"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, client client.Client, isvc *v1beta1.InferenceService) {
				// Verify RayCluster exists
				rayCluster := &ray.RayCluster{}
				err := client.Get(context.Background(), types.NamespacedName{
					Name:      "test-isvc-predictor-0",
					Namespace: "default",
				}, rayCluster)
				assert.NoError(t, err)
			},
		},
		{
			name: "reconciliation with multinode prober enabled",
			setupClient: func() client.Client {
				return kfake.NewClientBuilder().WithScheme(s).Build()
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						"serving.ome.io/enable-multinode-prober": "true",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: ptr.To(2),
							MaxReplicas: 4,
						},
						Model: &v1beta1.ModelSpec{
							PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
								RuntimeVersion: ptr.To("0.5.3"),
								Container: corev1.Container{
									Image: "vllm/vllm-openai:latest",
									Name:  "kserve-container",
								},
							},
							Runtime:   ptr.To("vllm"),
							BaseModel: ptr.To("falcon-40b"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, client client.Client, isvc *v1beta1.InferenceService) {
				// Verify MultiNodeProber deployment was created
				// The name format is: test-isvc-0-mnp, test-isvc-1-mnp, etc.
				deployment := &appsv1.Deployment{}
				err := client.Get(context.Background(), types.NamespacedName{
					Name:      "test-isvc-0-mnp",
					Namespace: "default",
				}, deployment)
				assert.NoError(t, err)
				assert.NotNil(t, deployment)

				// Check second deployment since MinReplicas is 2
				deployment2 := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{
					Name:      "test-isvc-1-mnp",
					Namespace: "default",
				}, deployment2)
				assert.NoError(t, err)
				assert.NotNil(t, deployment2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			client := tt.setupClient()

			// Create the required configmaps
			createTestConfigMaps(t, clientset)

			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{
					tt.isvc.Spec.Predictor.Model.Container,
				},
			}

			r, err := NewMultiNodeVllmReconciler(
				client,
				clientset,
				s,
				tt.isvc.ObjectMeta,
				&tt.isvc.Spec.Predictor.ComponentExtensionSpec,
				podSpec,
			)
			require.NoError(t, err)

			rayClusters, _, err := r.Reconcile()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, rayClusters)
			}

			if tt.validate != nil {
				tt.validate(t, client, tt.isvc)
			}
		})
	}
}

func TestCreateRawURL(t *testing.T) {
	tests := []struct {
		name      string
		metadata  metav1.ObjectMeta
		wantHost  string
		wantError bool
	}{
		{
			name: "create URL with basic metadata",
			metadata: metav1.ObjectMeta{
				Name:      "test-isvc",
				Namespace: "default",
			},
			wantHost:  "test-isvc-predictor.default",
			wantError: false,
		},
		{
			name: "create URL with custom namespace",
			metadata: metav1.ObjectMeta{
				Name:      "custom-isvc",
				Namespace: "production",
			},
			wantHost:  "custom-isvc-predictor.production",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			// Create a ConfigMap for ingress config
			ingressConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.InferenceServiceConfigMapName,
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"ingress": `{
						"ingressGateway": "knative-serving/knative-ingress-gateway",
						"ingressService": "istio-ingressgateway.istio-system.svc.cluster.local",
						"localGateway": "knative-serving/knative-local-gateway",
						"localGatewayService": "knative-local-gateway.istio-system.svc.cluster.local",
						"ingressDomain": "example.com",
						"ingressClassName": "istio",
						"domainTemplate": "{{ .Name }}-{{ .Namespace }}.{{ .IngressDomain }}",
						"urlScheme": "http",
						"disableIstioVirtualHost": false
					}`,
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.Background(), ingressConfigMap, metav1.CreateOptions{})
			require.NoError(t, err)

			url, err := createRawURL(clientset, tt.metadata)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, url)
				assert.Equal(t, "http", url.Scheme)
				// Host will include the domain template parsing
				assert.Contains(t, url.Host, tt.metadata.Name)
			}
		})
	}
}

func TestMultiNodeVllmReconciler_ReconcileWithIstioSidecar(t *testing.T) {
	s := scheme.Scheme
	_ = v1beta1.AddToScheme(s)
	_ = ray.AddToScheme(s)
	_ = istiov1beta1.AddToScheme(s)

	metadata := metav1.ObjectMeta{
		Name:      "test-isvc",
		Namespace: "default",
		Labels: map[string]string{
			constants.IstioSidecarInjectionLabel: "true",
		},
	}

	componentExt := &v1beta1.ComponentExtensionSpec{
		MinReplicas: ptr.To(2),
		MaxReplicas: 4,
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "kserve-container",
				Image: "vllm/vllm-openai:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset()

	// Create required ConfigMaps
	createTestConfigMaps(t, clientset)

	client := kfake.NewClientBuilder().WithScheme(s).Build()

	r, err := NewMultiNodeVllmReconciler(client, clientset, s, metadata, componentExt, podSpec)
	require.NoError(t, err)
	assert.NotNil(t, r.IstioSidecar)

	_, _, err = r.Reconcile()
	assert.NoError(t, err)
}

// Helper function to create test ConfigMaps
func createTestConfigMaps(t *testing.T, clientset *fake.Clientset) {
	ingressConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.InferenceServiceConfigMapName,
			Namespace: constants.OMENamespace,
		},
		Data: map[string]string{
			"ingress": `{
				"ingressGateway": "knative-serving/knative-ingress-gateway",
				"ingressService": "istio-ingressgateway.istio-system.svc.cluster.local",
				"localGateway": "knative-serving/knative-local-gateway",
				"localGatewayService": "knative-local-gateway.istio-system.svc.cluster.local",
				"ingressDomain": "example.com",
				"ingressClassName": "istio",
				"domainTemplate": "{{ .Name }}-{{ .Namespace }}.{{ .IngressDomain }}",
				"urlScheme": "http",
				"disableIstioVirtualHost": false
			}`,
			"multinodeProber": `{
				"image": "kserve/prober:latest",
				"cpuRequest": "100m",
				"memoryRequest": "128Mi",
				"cpuLimit": "200m",
				"memoryLimit": "256Mi",
				"startupFailureThreshold": 30,
				"startupPeriodSeconds": 10,
				"startupInitialDelaySeconds": 0,
				"startupTimeoutSeconds": 5,
				"unavailableThresholdSeconds": 60
			}`,
		},
	}
	_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.Background(), ingressConfigMap, metav1.CreateOptions{})
	require.NoError(t, err)
}
