package multinodevllm

import (
	"context"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	knapis "knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MultiNodeProberReconciler reconciles the Kubernetes Deployment resource for MultiNodeProber
type MultiNodeProberReconciler struct {
	client      kclient.Client
	scheme      *runtime.Scheme
	Deployments []*appsv1.Deployment
	URL         *knapis.URL
}

// NewMultiNodeProberReconciler initializes a new MultiNodeProberReconciler
func NewMultiNodeProberReconciler(
	client kclient.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	multiNodeProberConfig *controllerconfig.MultiNodeProberConfig,
) *MultiNodeProberReconciler {
	deployments := make([]*appsv1.Deployment, 0, *componentExt.MinReplicas)
	for i := 0; i < *componentExt.MinReplicas; i++ {
		url := &knapis.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s.%s.svc.cluster.local", constants.DefaultRayHeadServiceName(componentMeta.Name, i), componentMeta.Namespace),
		}
		dply := createRawDeployment(componentMeta, multiNodeProberConfig, url, i)
		deployments = append(deployments, dply)
	}
	return &MultiNodeProberReconciler{
		client:      client,
		scheme:      scheme,
		Deployments: deployments,
	}
}

func createRawDeployment(
	componentMeta metav1.ObjectMeta,
	multiNodeProberConfig *controllerconfig.MultiNodeProberConfig,
	url *knapis.URL,
	index int,
) *appsv1.Deployment {
	podMetadata := componentMeta.DeepCopy()
	podMetadata.Name = rayutils.CheckName(fmt.Sprintf("%s-%d-mnp", componentMeta.Name, index))
	podMetadata.Labels["app"] = constants.GetRawServiceLabel(componentMeta.Name)
	utils.SetPodLabelsFromAnnotations(podMetadata)

	podSpec := getDefaultPodSpec(multiNodeProberConfig, url)
	deployment := &appsv1.Deployment{
		ObjectMeta: *podMetadata,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": constants.GetRawServiceLabel(componentMeta.Name),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: *podMetadata,
				Spec:       *podSpec,
			},
		},
	}

	setDefaultDeploymentSpec(&deployment.Spec)
	return deployment
}

func getDefaultPodSpec(
	multiNodeProberConfig *controllerconfig.MultiNodeProberConfig,
	url *knapis.URL,
) *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            constants.MultiNodeProberContainerName,
				Image:           multiNodeProberConfig.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPULimit),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryLimit),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPURequest),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryRequest),
					},
				},
				ReadinessProbe: createProbe("/healthz"),
				LivenessProbe:  createProbe("/readyz"),
				StartupProbe:   createStartupProbe(multiNodeProberConfig),
				Args: []string{
					"--vllm-endpoint",
					fmt.Sprintf("%s:%s", url.String(), constants.InferenceServiceDefaultHttpPort),
					"--addr",
					"0.0.0.0:8080",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: constants.MultiNodeProberContainerPort,
					},
				},
			},
		},
	}
}

func createProbe(path string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.IntOrString{
					IntVal: constants.MultiNodeProberContainerPort,
				},
				Path: path,
			},
		},
		TimeoutSeconds:   5,
		PeriodSeconds:    30,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}
}

func createStartupProbe(config *controllerconfig.MultiNodeProberConfig) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.IntOrString{
					IntVal: constants.MultiNodeProberContainerPort,
				},
				Path: "/startupz",
			},
		},
		TimeoutSeconds:      config.StartupTimeoutSeconds,
		PeriodSeconds:       config.StartupPeriodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    config.StartupFailureThreshold,
		InitialDelaySeconds: config.StartupInitialDelaySeconds,
	}
}

func setDefaultDeploymentSpec(spec *appsv1.DeploymentSpec) {
	if spec.Strategy.Type == "" {
		spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	}
	if spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType && spec.Strategy.RollingUpdate == nil {
		spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
		}
	}
	if spec.RevisionHistoryLimit == nil {
		revisionHistoryLimit := int32(10)
		spec.RevisionHistoryLimit = &revisionHistoryLimit
	}
	if spec.ProgressDeadlineSeconds == nil {
		progressDeadlineSeconds := int32(600)
		spec.ProgressDeadlineSeconds = &progressDeadlineSeconds
	}
}

// Reconcile reconciles the Deployments managed by MultiNodeProberReconciler
func (r *MultiNodeProberReconciler) Reconcile() error {
	for _, deployment := range r.Deployments {
		result, existingDeployment, err := r.checkDeploymentExist(deployment)
		if err != nil {
			return err
		}

		var opErr error
		switch result {
		case constants.CheckResultCreate:
			opErr = r.client.Create(context.TODO(), deployment)
		case constants.CheckResultUpdate:
			deployment.ResourceVersion = existingDeployment.ResourceVersion
			opErr = r.client.Update(context.TODO(), deployment)
		}

		if opErr != nil {
			return opErr
		}
	}

	return nil
}

// checkDeploymentExist checks if the deployment exists and determines if it should be created, updated, or is already present
func (r *MultiNodeProberReconciler) checkDeploymentExist(dply *appsv1.Deployment) (constants.CheckResultType, *appsv1.Deployment, error) {
	existingDeployment := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: dply.ObjectMeta.Namespace,
		Name:      dply.ObjectMeta.Name,
	}, existingDeployment)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	// Ignore Replicas field for HPA scaling when comparing deployments
	ignoreFields := cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas")

	// Perform a dry-run update to compare deployments
	if err := r.client.Update(context.TODO(), dply, kclient.DryRunAll); err != nil {
		log.Error(err, "Failed to perform dry-run update of deployment", "Deployments", dply.Name)
		return constants.CheckResultUnknown, nil, err
	}

	if diff, err := kmp.SafeDiff(dply.Spec, existingDeployment.Spec, ignoreFields); err != nil {
		return constants.CheckResultUnknown, nil, err
	} else if diff != "" {
		log.Info("Deployments Updated", "Diff", diff)
		return constants.CheckResultUpdate, existingDeployment, nil
	}

	return constants.CheckResultExisted, existingDeployment, nil
}
