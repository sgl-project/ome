package components

import (
	"context"
	"strings"
	"testing"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	omeTesting "github.com/sgl-project/ome/pkg/testing"
)

// Helper functions for creating pointers
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func TestEngineReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	testEnv := omeTesting.SetupEnvTest()
	cfg, err := testEnv.Start()
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(cfg).NotTo(gomega.BeNil())
	defer func(testEnv *envtest.Environment) {
		_ = testEnv.Stop()
	}(testEnv)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(appsv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(knservingv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(lws.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(kedav1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(autoscalingv2.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(policyv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name           string
		deploymentMode constants.DeploymentModeType
		baseModel      *v1beta1.BaseModelSpec
		baseModelMeta  *metav1.ObjectMeta
		engineSpec     *v1beta1.EngineSpec
		runtime        *v1beta1.ServingRuntimeSpec
		runtimeName    string
		isvc           *v1beta1.InferenceService
		setupMocks     func(client.Client, kubernetes.Interface)
		validate       func(*testing.T, client.Client, *v1beta1.InferenceService)
		wantErr        bool
	}{
		{
			name:           "Raw deployment with basic engine spec",
			deploymentMode: constants.RawDeployment,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/mnt/models/model1"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "base-model-1",
				Namespace: "default",
				Annotations: map[string]string{
					constants.ModelCategoryAnnotation: "LARGE",
				},
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "engine:latest",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("1"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime:latest",
							Env: []v1.EnvVar{
								{Name: "RUNTIME_ENV", Value: "test"},
							},
						},
					},
				},
			},
			runtimeName: "test-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config in both clients
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
					},
				}
				// Create in controller-runtime client
				err := c.Create(context.TODO(), cm)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Also create in clientset (if different)
				_, err = cs.CoreV1().ConfigMaps("ome").Create(context.TODO(), cm, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta1.InferenceService) {
				// Check deployment was created
				deployment := &appsv1.Deployment{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-isvc-engine",
					Namespace: "default",
				}, deployment)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("engine:latest"))
				// For a raw deployment test without a runner specified, we shouldn't expect MODEL_PATH
				// since environment variables are only applied to the runner container
				g.Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(gomega.BeEmpty())

				// Check node selector was added for the base model
				expectedNodeSelector := map[string]string{
					"models.ome.io/default.basemodel.base-model-1": "Ready",
				}
				g.Expect(deployment.Spec.Template.Spec.NodeSelector).To(gomega.Equal(expectedNodeSelector))

				// Check service was created
				service := &v1.Service{}
				err = c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-isvc-engine",
					Namespace: "default",
				}, service)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
		},
		{
			name:           "Multi-node deployment with leader and worker specs",
			deploymentMode: constants.MultiNode,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/mnt/models/model2"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "base-model-2",
				Namespace: "default",
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
				},
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader-container",
								Image: "leader:latest",
							},
						},
					},
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Name:  "leader-container",
							Image: "runtime-leader:latest",
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(2),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker-container",
								Image: "worker:latest",
							},
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []v1.Container{
						{
							Name:  "runtime-container",
							Image: "runtime:latest",
						},
					},
				},
			},
			runtimeName: "multi-node-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mn-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config in both clients
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
					},
				}
				// Create in controller-runtime client
				err := c.Create(context.TODO(), cm)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Also create in clientset (if different)
				_, err = cs.CoreV1().ConfigMaps("ome").Create(context.TODO(), cm, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta1.InferenceService) {
				// Check LeaderWorkerSet was created
				lwsList := &lws.LeaderWorkerSetList{}
				err := c.List(context.TODO(), lwsList, client.InNamespace("default"))
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(lwsList.Items).To(gomega.HaveLen(1))
				g.Expect(lwsList.Items[0].Spec.Replicas).To(gomega.Equal(int32Ptr(2)))

				// Check node selector was added for both leader and worker pods
				expectedNodeSelector := map[string]string{
					"models.ome.io/default.basemodel.base-model-2": "Ready",
				}
				g.Expect(lwsList.Items[0].Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.NodeSelector).To(gomega.Equal(expectedNodeSelector))
				g.Expect(lwsList.Items[0].Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.NodeSelector).To(gomega.Equal(expectedNodeSelector))
			},
		},
		{
			name:           "Serverless deployment",
			deploymentMode: constants.Serverless,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "base-model-3",
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "serverless-container",
							Image: "serverless:latest",
						},
					},
				},
			},
			runtime:     &v1beta1.ServingRuntimeSpec{},
			runtimeName: "serverless-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sl-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config in both clients
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
					},
				}
				// Create in controller-runtime client
				err := c.Create(context.TODO(), cm)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Also create in clientset (if different)
				_, err = cs.CoreV1().ConfigMaps("ome").Create(context.TODO(), cm, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta1.InferenceService) {
				// Check Knative Service was created
				ksvc := &knservingv1.Service{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-sl-isvc-engine",
					Namespace: "default",
				}, ksvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
		},
		{
			name:           "Fine-tuned serving with single weight",
			deploymentMode: constants.RawDeployment,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/mnt/models/base"),
				},
				ModelExtensionSpec: v1beta1.ModelExtensionSpec{
					Vendor: stringPtr("meta"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "llama-base",
			},
			engineSpec: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ft-container",
							Image: "ft:latest",
						},
					},
				},
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "ft-container", // Using same name so it will match and merge
						Image: "ft:latest",
					},
				},
			},
			runtime:     &v1beta1.ServingRuntimeSpec{},
			runtimeName: "ft-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ft-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						FineTunedWeights: []string{"ft-weight-1"},
					},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config in both clients
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
					},
				}
				// Create in controller-runtime client
				err := c.Create(context.TODO(), cm)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Also create in clientset (if different)
				_, err = cs.CoreV1().ConfigMaps("ome").Create(context.TODO(), cm, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta1.InferenceService) {
				deployment := &appsv1.Deployment{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-ft-isvc-engine",
					Namespace: "default",
				}, deployment)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Check annotations
				annotations := deployment.Spec.Template.Annotations
				g.Expect(annotations[constants.FineTunedAdapterInjectionKey]).To(gomega.Equal("ft-weight-1"))
				g.Expect(annotations[constants.FineTunedWeightFTStrategyKey]).To(gomega.Equal("lora"))

				// Check volume mounts
				container := deployment.Spec.Template.Spec.Containers[0]
				var hasModelMount bool
				for _, vm := range container.VolumeMounts {
					if vm.Name == constants.ModelEmptyDirVolumeName {
						hasModelMount = true
						break
					}
				}
				g.Expect(hasModelMount).To(gomega.BeTrue())
			},
		},
		{
			name:           "ClusterBaseModel with node selector",
			deploymentMode: constants.RawDeployment,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/mnt/models/cluster-model"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "cluster-base-model",
				// No namespace for ClusterBaseModel
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "engine",
							Image: "engine:latest",
						},
					},
				},
			},
			runtime:     &v1beta1.ServingRuntimeSpec{},
			runtimeName: "test-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
					},
				}
				err := c.Create(context.TODO(), cm)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				_, err = cs.CoreV1().ConfigMaps("ome").Create(context.TODO(), cm, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta1.InferenceService) {
				// Check deployment was created
				deployment := &appsv1.Deployment{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-cluster-isvc-engine",
					Namespace: "default",
				}, deployment)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Check node selector for ClusterBaseModel (no namespace in label)
				expectedNodeSelector := map[string]string{
					"models.ome.io/clusterbasemodel.cluster-base-model": "Ready",
				}
				g.Expect(deployment.Spec.Template.Spec.NodeSelector).To(gomega.Equal(expectedNodeSelector))
			},
		},
		{
			name:           "Engine with nil spec should error",
			deploymentMode: constants.RawDeployment,
			engineSpec:     nil,
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nil-isvc",
					Namespace: "default",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create objects to add to fake client
			objects := []client.Object{tt.isvc}

			// For fine-tuned serving test, add the FineTunedWeight
			if tt.name == "Fine-tuned serving with single weight" {
				ftWeight := &v1beta1.FineTunedWeight{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ft-weight-1",
					},
					Spec: v1beta1.FineTunedWeightSpec{
						HyperParameters: runtime.RawExtension{
							Raw: []byte(`{"strategy": "lora"}`),
						},
						Configuration: runtime.RawExtension{
							Raw: []byte(`{}`), // Empty config to avoid JSON parsing error
						},
					},
				}
				objects = append(objects, ftWeight)
			}

			// Create fake client
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create fake clientset
			clientset := fake.NewClientset()

			// Setup mocks if needed
			if tt.setupMocks != nil {
				tt.setupMocks(c, clientset)
			}

			// Create engine using the constructor
			engine := NewEngine(
				c,
				clientset,
				scheme,
				&controllerconfig.InferenceServicesConfig{},
				tt.deploymentMode,
				tt.baseModel,
				tt.baseModelMeta,
				tt.engineSpec,
				nil, // runtime
				tt.runtimeName,
			).(*Engine)

			// Set fine-tuned fields if needed
			if tt.name == "Fine-tuned serving with single weight" {
				engine.FineTunedServing = true
			}

			// Reconcile
			result, err := engine.Reconcile(tt.isvc)

			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(result).To(gomega.Equal(ctrl.Result{}))

				// Run validations
				if tt.validate != nil {
					tt.validate(t, c, tt.isvc)
				}
			}
		})
	}
}

func TestEngineReconcileObjectMeta(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                string
		isvc                *v1beta1.InferenceService
		engineSpec          *v1beta1.EngineSpec
		baseModel           *v1beta1.BaseModelSpec
		baseModelMeta       *metav1.ObjectMeta
		runtimeName         string
		fineTunedServing    bool
		fineTunedWeights    []*v1beta1.FineTunedWeight
		expectedAnnotations map[string]string
		expectedLabels      map[string]string
		expectedName        string
	}{
		{
			name: "Basic object metadata",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						"custom-annotation": "value",
					},
					Labels: map[string]string{
						"custom-label": "value",
					},
				},
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{
						"engine-annotation": "engine-value",
					},
					Labels: map[string]string{
						"engine-label": "engine-value",
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: stringPtr("1.0"),
				},
				ModelExtensionSpec: v1beta1.ModelExtensionSpec{
					Vendor: stringPtr("meta"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "base-model",
				Annotations: map[string]string{
					constants.ModelCategoryAnnotation: "LARGE",
				},
			},
			runtimeName: "test-runtime",
			expectedAnnotations: map[string]string{
				"custom-annotation":                    "value",
				"engine-annotation":                    "engine-value",
				constants.BaseModelName:                "base-model",
				constants.BaseModelFormat:              "safetensors",
				constants.BaseModelFormatVersion:       "1.0",
				constants.BaseModelVendorAnnotationKey: "meta",
				constants.ServingRuntimeKeyName:        "test-runtime",
			},
			expectedLabels: map[string]string{
				"custom-label":                                  "value",
				"engine-label":                                  "engine-value",
				constants.InferenceServicePodLabelKey:           "test-isvc",
				constants.OMEComponentLabel:                     "engine",
				constants.ServingRuntimeLabelKey:                "test-runtime",
				constants.InferenceServiceBaseModelNameLabelKey: "base-model",
				constants.InferenceServiceBaseModelSizeLabelKey: "LARGE",
				constants.BaseModelTypeLabelKey:                 "Serving",
				constants.BaseModelVendorLabelKey:               "meta",
				constants.FTServingLabelKey:                     "false",
			},
			expectedName: "test-isvc-engine",
		},
		{
			name: "Fine-tuned serving metadata",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ft-isvc",
					Namespace: "default",
				},
			},
			engineSpec: &v1beta1.EngineSpec{},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				ModelExtensionSpec: v1beta1.ModelExtensionSpec{
					Vendor: stringPtr("meta"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "llama-base",
			},
			fineTunedServing: true,
			fineTunedWeights: []*v1beta1.FineTunedWeight{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ft-weight",
					},
					Spec: v1beta1.FineTunedWeightSpec{
						HyperParameters: runtime.RawExtension{
							Raw: []byte(`{"strategy": "lora"}`),
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: "ft-weight",
				constants.FineTunedWeightFTStrategyKey: "lora",
				constants.BaseModelName:                "llama-base",
				constants.BaseModelFormat:              "safetensors",
				constants.BaseModelVendorAnnotationKey: "meta",
			},
			expectedLabels: map[string]string{
				constants.InferenceServicePodLabelKey:           "ft-isvc",
				constants.OMEComponentLabel:                     "engine",
				constants.FTServingLabelKey:                     "true",
				constants.FineTunedWeightFTStrategyLabelKey:     "lora",
				constants.FTServingWithMergedWeightsLabelKey:    "false",
				constants.InferenceServiceBaseModelNameLabelKey: "llama-base",
				constants.InferenceServiceBaseModelSizeLabelKey: "SMALL",
				constants.BaseModelTypeLabelKey:                 "Serving",
				constants.BaseModelVendorLabelKey:               "meta",
			},
			expectedName: "ft-isvc-engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme
			scheme := runtime.NewScheme()
			clientset := fake.NewClientset()
			c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

			// Create engine using the constructor
			engine := NewEngine(
				c,
				clientset,
				scheme,
				&controllerconfig.InferenceServicesConfig{},
				constants.RawDeployment,
				tt.baseModel,
				tt.baseModelMeta,
				tt.engineSpec,
				nil, // runtime
				tt.runtimeName,
			).(*Engine)

			// Set fine-tuned fields if needed
			if tt.fineTunedServing {
				engine.FineTunedServing = tt.fineTunedServing
			}
			if tt.fineTunedWeights != nil {
				engine.FineTunedWeights = tt.fineTunedWeights
			}

			// Test reconcileObjectMeta
			objectMeta, err := engine.reconcileObjectMeta(tt.isvc)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Validate name
			g.Expect(objectMeta.Name).To(gomega.Equal(tt.expectedName))
			g.Expect(objectMeta.Namespace).To(gomega.Equal("default"))

			// Validate annotations
			for k, v := range tt.expectedAnnotations {
				g.Expect(objectMeta.Annotations).To(gomega.HaveKeyWithValue(k, v))
			}

			// Validate labels
			for k, v := range tt.expectedLabels {
				g.Expect(objectMeta.Labels).To(gomega.HaveKeyWithValue(k, v))
			}
		})
	}
}

func TestEngineWorkerPodSpec(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name               string
		engineSpec         *v1beta1.EngineSpec
		expectedError      bool
		expectedContainers int
		validatePodSpec    func(*v1.PodSpec)
	}{
		{
			name: "Worker with leader runner",
			engineSpec: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Name:  "leader-runner",
							Image: "leader-runtime:latest",
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(2),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker-container",
								Image: "worker:latest",
							},
						},
					},
				},
			},
			expectedContainers: 1,
			validatePodSpec: func(ps *v1.PodSpec) {
				// Should have worker container (leader runner is not merged into worker pod spec)
				found := false
				for _, c := range ps.Containers {
					if c.Name == "worker-container" {
						found = true
						g.Expect(c.Image).To(gomega.Equal("worker:latest"))
					}
				}
				g.Expect(found).To(gomega.BeTrue())
			},
		},
		{
			name: "Worker without leader runner",
			engineSpec: &v1beta1.EngineSpec{
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(1),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker-container",
								Image: "worker:latest",
							},
						},
					},
				},
			},
			expectedContainers: 1,
			validatePodSpec: func(ps *v1.PodSpec) {
				g.Expect(ps.Containers[0].Name).To(gomega.Equal("worker-container"))
				g.Expect(ps.Containers[0].Image).To(gomega.Equal("worker:latest"))
			},
		},
		{
			name:       "No worker spec returns nil",
			engineSpec: &v1beta1.EngineSpec{},
			validatePodSpec: func(ps *v1.PodSpec) {
				g.Expect(ps).To(gomega.BeNil())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			}

			objectMeta := &metav1.ObjectMeta{}

			// Create scheme
			scheme := runtime.NewScheme()
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			clientset := fake.NewClientset()
			c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

			// Create engine using the constructor
			engine := NewEngine(
				c,
				clientset,
				scheme,
				&controllerconfig.InferenceServicesConfig{},
				constants.RawDeployment,
				nil, // baseModel
				nil, // baseModelMeta
				tt.engineSpec,
				nil, // runtime
				"",  // runtimeName
			).(*Engine)

			podSpec, err := engine.reconcileWorkerPodSpec(isvc, objectMeta)

			if tt.expectedError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				if tt.validatePodSpec != nil {
					tt.validatePodSpec(podSpec)
				}
			}
		})
	}
}
