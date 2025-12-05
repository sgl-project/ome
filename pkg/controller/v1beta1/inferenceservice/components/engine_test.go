package components

import (
	"context"
	"strings"
	"testing"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/onsi/gomega"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
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
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
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
	g.Expect(ray.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

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

				// Check preferred node affinity was added for the base model
				g.Expect(deployment.Spec.Template.Spec.Affinity).NotTo(gomega.BeNil())
				g.Expect(deployment.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(gomega.BeNil())
				preferredTerms := deployment.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
				g.Expect(preferredTerms).NotTo(gomega.BeEmpty())
				// Find the model affinity term
				var foundModelTerm bool
				for _, term := range preferredTerms {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "models.ome.io/default.basemodel.base-model-1" {
							g.Expect(term.Weight).To(gomega.Equal(int32(100)))
							g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
							g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
							foundModelTerm = true
						}
					}
				}
				g.Expect(foundModelTerm).To(gomega.BeTrue(), "Model affinity term not found")

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

				// Check preferred node affinity was added for both leader and worker pods
				leaderAffinity := lwsList.Items[0].Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Affinity
				workerAffinity := lwsList.Items[0].Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.Affinity
				g.Expect(leaderAffinity).NotTo(gomega.BeNil())
				g.Expect(leaderAffinity.NodeAffinity).NotTo(gomega.BeNil())
				g.Expect(workerAffinity).NotTo(gomega.BeNil())
				g.Expect(workerAffinity.NodeAffinity).NotTo(gomega.BeNil())

				// Verify leader pod has model affinity
				var foundLeaderModelTerm bool
				for _, term := range leaderAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "models.ome.io/default.basemodel.base-model-2" {
							g.Expect(term.Weight).To(gomega.Equal(int32(100)))
							g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
							g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
							foundLeaderModelTerm = true
							break
						}
					}
					if foundLeaderModelTerm {
						break
					}
				}
				g.Expect(foundLeaderModelTerm).To(gomega.BeTrue(), "Leader model affinity term not found")

				// Verify worker pod has model affinity
				var foundWorkerModelTerm bool
				for _, term := range workerAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "models.ome.io/default.basemodel.base-model-2" {
							g.Expect(term.Weight).To(gomega.Equal(int32(100)))
							g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
							g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
							foundWorkerModelTerm = true
							break
						}
					}
					if foundWorkerModelTerm {
						break
					}
				}
				g.Expect(foundWorkerModelTerm).To(gomega.BeTrue(), "Worker model affinity term not found")
			},
		},
		{
			name:           "Multi-node Ray VLLM deployment",
			deploymentMode: constants.MultiNodeRayVLLM,
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/mnt/models/model-ray"),
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "base-model-ray",
				Namespace: "default",
			},
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					Annotations: map[string]string{
						constants.DeploymentMode: string(constants.MultiNodeRayVLLM),
					},
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ray-container",
							Image: "ray-vllm:latest",
						},
					},
				},
			},
			runtime:     &v1beta1.ServingRuntimeSpec{},
			runtimeName: "ray-runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ray-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{},
				},
			},
			setupMocks: func(c client.Client, cs kubernetes.Interface) {
				// Create inferenceservice config in both clients with multinodeProber config
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "inferenceservice-config",
						Namespace: "ome",
					},
					Data: map[string]string{
						"config": "{}",
						"multinodeProber": `{
							"image": "multinode-prober:latest",
							"memoryRequest": "100Mi",
							"memoryLimit": "100Mi",
							"cpuRequest": "100m",
							"cpuLimit": "100m",
							"startupFailureThreshold": 150,
							"startupPeriodSeconds": 30,
							"startupTimeoutSeconds": 60,
							"startupInitialDelaySeconds": 200,
							"unavailableThresholdSeconds": 1800
						}`,
						"ingress": `{
							"ingressGateway": "knative-serving/knative-ingress-gateway",
							"ingressService": "istio-ingressgateway.istio-system.svc.cluster.local",
							"ingressDomain": "svc.cluster.local",
							"domainTemplate": "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"
						}`,
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
				// Check RayCluster was created (not LeaderWorkerSet)
				rayClusterList := &ray.RayClusterList{}
				err := c.List(context.TODO(), rayClusterList, client.InNamespace("default"))
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(rayClusterList.Items).To(gomega.HaveLen(2)) // MinReplicas is 2

				// Verify first RayCluster
				rayCluster := &ray.RayCluster{}
				err = c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-ray-isvc-engine-0",
					Namespace: "default",
				}, rayCluster)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(rayCluster.Spec.HeadGroupSpec.Template.Spec.Containers[0].Image).To(gomega.Equal("ray-vllm:latest"))

				// Check preferred node affinity was added for head and worker pods
				headAffinity := rayCluster.Spec.HeadGroupSpec.Template.Spec.Affinity
				g.Expect(headAffinity).NotTo(gomega.BeNil())
				g.Expect(headAffinity.NodeAffinity).NotTo(gomega.BeNil())

				// Verify head pod has model affinity
				var foundHeadModelTerm bool
				for _, term := range headAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "models.ome.io/default.basemodel.base-model-ray" {
							g.Expect(term.Weight).To(gomega.Equal(int32(100)))
							g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
							g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
							foundHeadModelTerm = true
							break
						}
					}
					if foundHeadModelTerm {
						break
					}
				}
				g.Expect(foundHeadModelTerm).To(gomega.BeTrue(), "Head model affinity term not found")

				// Verify worker pod has model affinity if workers exist
				if len(rayCluster.Spec.WorkerGroupSpecs) > 0 {
					workerAffinity := rayCluster.Spec.WorkerGroupSpecs[0].Template.Spec.Affinity
					g.Expect(workerAffinity).NotTo(gomega.BeNil())
					g.Expect(workerAffinity.NodeAffinity).NotTo(gomega.BeNil())
					var foundWorkerModelTerm bool
					for _, term := range workerAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
						for _, expr := range term.Preference.MatchExpressions {
							if expr.Key == "models.ome.io/default.basemodel.base-model-ray" {
								g.Expect(term.Weight).To(gomega.Equal(int32(100)))
								g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
								g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
								foundWorkerModelTerm = true
								break
							}
						}
						if foundWorkerModelTerm {
							break
						}
					}
					g.Expect(foundWorkerModelTerm).To(gomega.BeTrue(), "Worker model affinity term not found")
				}
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

				// Check preferred node affinity for ClusterBaseModel (no namespace in label)
				g.Expect(deployment.Spec.Template.Spec.Affinity).NotTo(gomega.BeNil())
				g.Expect(deployment.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(gomega.BeNil())
				preferredTerms := deployment.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
				g.Expect(preferredTerms).NotTo(gomega.BeEmpty())
				// Find the model affinity term
				var foundModelTerm bool
				for _, term := range preferredTerms {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "models.ome.io/clusterbasemodel.cluster-base-model" {
							g.Expect(term.Weight).To(gomega.Equal(int32(100)))
							g.Expect(expr.Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
							g.Expect(expr.Values).To(gomega.Equal([]string{"Ready"}))
							foundModelTerm = true
						}
					}
				}
				g.Expect(foundModelTerm).To(gomega.BeTrue(), "Model affinity term not found")
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
				nil, // supportedModelFormat
				nil, // acceleratorClass
				"",  // acceleratorClassName
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
				nil, // supportedModelFormat
				nil, // acceleratorClass
				"",  // acceleratorClassName
			).(*Engine)

			// Set fine-tuned fields if needed
			if tt.fineTunedServing {
				engine.FineTunedServing = tt.fineTunedServing
			}
			if tt.fineTunedWeights != nil {
				engine.FineTunedWeights = tt.fineTunedWeights
			}

			// Test reconcileObjectMeta
			objectMeta, err := engine.reconcileObjectMeta(tt.isvc, true)
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
				nil, // supportedModelFormat
				nil, // acceleratorClass
				"",  // acceleratorClassName
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

// Add these test cases to engine_test.go

func TestEngineResourceMerging(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                  string
		engineSpec            *v1beta1.EngineSpec
		runtime               *v1beta1.ServingRuntimeSpec
		acceleratorClass      *v1beta1.AcceleratorClassSpec
		expectResourcesMerged bool
		validateResources     func(*v1.Container)
	}{
		{
			name: "User specified resources - should NOT merge",
			engineSpec: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "ome-container",
						Image: "engine:latest",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("2"),
								v1.ResourceMemory: resource.MustParse("4Gi"),
							},
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []v1.Container{
						{
							Name: "ome-container",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("4"),
									v1.ResourceMemory: resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
			expectResourcesMerged: false,
			validateResources: func(c *v1.Container) {
				// Should keep user's values (2 CPU, 4Gi memory)
				cpu := c.Resources.Requests[v1.ResourceCPU]
				g.Expect(cpu.String()).To(gomega.Equal("2"))
				memory := c.Resources.Requests[v1.ResourceMemory]
				g.Expect(memory.String()).To(gomega.Equal("4Gi"))
			},
		},
		{
			name: "User did NOT specify resources - should merge from runtime",
			engineSpec: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "ome-container",
						Image: "engine:latest",
						// No resources specified
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []v1.Container{
						{
							Name: "ome-container",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("4"),
									v1.ResourceMemory: resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
			expectResourcesMerged: true,
			validateResources: func(c *v1.Container) {
				// Should use runtime's values (4 CPU, 8Gi memory)
				cpu := c.Resources.Requests[v1.ResourceCPU]
				g.Expect(cpu.String()).To(gomega.Equal("4"))
				memory := c.Resources.Requests[v1.ResourceMemory]
				g.Expect(memory.String()).To(gomega.Equal("8Gi"))
			},
		},
		{
			name: "User did NOT specify resources - should merge from AcceleratorClass",
			engineSpec: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "ome-container",
						Image: "engine:latest",
						// No resources specified
					},
				},
			},
			acceleratorClass: &v1beta1.AcceleratorClassSpec{
				Resources: []v1beta1.AcceleratorResource{
					{
						Name:     "nvidia.com/gpu",
						Quantity: resource.MustParse("2"),
					},
				},
			},
			expectResourcesMerged: true,
			validateResources: func(c *v1.Container) {
				// Should have GPU from AC
				gpu := c.Resources.Requests[v1.ResourceName("nvidia.com/gpu")]
				g.Expect(gpu.String()).To(gomega.Equal("2"))
			},
		},
		{
			name: "No runner specified - should NOT merge",
			engineSpec: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "engine:latest",
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []v1.Container{
						{
							Name: "ome-container",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("4"),
									v1.ResourceMemory: resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
			expectResourcesMerged: false,
			validateResources: func(c *v1.Container) {
				// Should not have resources merged
				g.Expect(c.Resources.Requests).To(gomega.BeNil())
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
				Spec: v1beta1.InferenceServiceSpec{
					Model:  &v1beta1.ModelRef{},
					Engine: tt.engineSpec,
				},
			}

			scheme := runtime.NewScheme()
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			clientset := fake.NewClientset()
			c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

			engine := NewEngine(
				c,
				clientset,
				scheme,
				&controllerconfig.InferenceServicesConfig{},
				constants.RawDeployment,
				nil, // baseModel
				nil, // baseModelMeta
				tt.engineSpec,
				tt.runtime,
				"test-runtime",
				nil, // supportedModelFormat
				tt.acceleratorClass,
				"test-accel-class",
			).(*Engine)

			// Call reconcilePodSpec which internally calls MergeEngineResources
			objectMeta := &metav1.ObjectMeta{Name: "test", Namespace: "default"}
			podSpec, err := engine.reconcilePodSpec(isvc, objectMeta)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Find the runner container
			var runnerContainer *v1.Container
			for i := range podSpec.Containers {
				if podSpec.Containers[i].Name == "ome-container" {
					runnerContainer = &podSpec.Containers[i]
					break
				}
			}
			g.Expect(runnerContainer).NotTo(gomega.BeNil())

			// Validate resources
			if tt.validateResources != nil {
				tt.validateResources(runnerContainer)
			}
		})
	}
}

func TestEngineAffinityMerging(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                 string
		engineSpec           *v1beta1.EngineSpec
		acceleratorClass     *v1beta1.AcceleratorClassSpec
		expectAffinityMerged bool
		validateAffinity     func(*v1.Affinity)
	}{
		{
			name: "User specified affinity - should NOT merge",
			engineSpec: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "custom-key",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"custom-value"},
											},
										},
									},
								},
							},
						},
					},
					Containers: []v1.Container{
						{Name: "ome-container", Image: "engine:latest"},
					},
				},
			},
			acceleratorClass: &v1beta1.AcceleratorClassSpec{
				Discovery: v1beta1.AcceleratorDiscovery{
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "ac-key",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"ac-value"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectAffinityMerged: false,
			validateAffinity: func(affinity *v1.Affinity) {
				// Should keep user's affinity (custom-key)
				g.Expect(affinity).NotTo(gomega.BeNil())
				g.Expect(affinity.NodeAffinity).NotTo(gomega.BeNil())
				terms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
				g.Expect(terms[0].MatchExpressions[0].Key).To(gomega.Equal("custom-key"))
			},
		},
		{
			name: "User did NOT specify affinity - should merge from AC",
			engineSpec: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					// No affinity specified
					Containers: []v1.Container{
						{Name: "ome-container", Image: "engine:latest"},
					},
				},
			},
			acceleratorClass: &v1beta1.AcceleratorClassSpec{
				Discovery: v1beta1.AcceleratorDiscovery{
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "ac-key",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"ac-value"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectAffinityMerged: true,
			validateAffinity: func(affinity *v1.Affinity) {
				// Should use AC's affinity (ac-key)
				g.Expect(affinity).NotTo(gomega.BeNil())
				g.Expect(affinity.NodeAffinity).NotTo(gomega.BeNil())
				terms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
				g.Expect(terms[0].MatchExpressions[0].Key).To(gomega.Equal("ac-key"))
			},
		},
		{
			name: "No affinity from AC - should remain nil",
			engineSpec: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{Name: "ome-container", Image: "engine:latest"},
					},
				},
			},
			acceleratorClass:     nil,
			expectAffinityMerged: false,
			validateAffinity: func(affinity *v1.Affinity) {
				// Should be nil
				g.Expect(affinity).To(gomega.BeNil())
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
				Spec: v1beta1.InferenceServiceSpec{
					Model:  &v1beta1.ModelRef{},
					Engine: tt.engineSpec,
				},
			}

			scheme := runtime.NewScheme()
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			clientset := fake.NewClientset()
			c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

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
				"test-runtime",
				nil, // supportedModelFormat
				tt.acceleratorClass,
				"test-accel-class",
			).(*Engine)

			// Call reconcilePodSpec which internally calls UpdateEngineAffinity
			objectMeta := &metav1.ObjectMeta{Name: "test", Namespace: "default"}
			podSpec, err := engine.reconcilePodSpec(isvc, objectMeta)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Validate affinity
			if tt.validateAffinity != nil {
				tt.validateAffinity(podSpec.Affinity)
			}
		})
	}
}

// Note: Worker resource and affinity tests are not included because MergeEngineResources and
// UpdateEngineAffinity check isvc.Spec.Engine.Runner and isvc.Spec.Engine.PodSpec.Affinity,
// not the worker-specific fields. This means the merging decision is based on the engine/leader
// spec, not the worker spec. This is the current implementation behavior.
