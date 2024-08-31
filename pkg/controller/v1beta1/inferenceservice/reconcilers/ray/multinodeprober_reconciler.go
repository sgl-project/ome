package raycluster

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	knapis "knative.dev/pkg/apis"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/kmp"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MultiNodeProberReconciler reconciles the raw kubernetes deployment resource for multi node prober
type MultiNodeProberReconciler struct {
	client      kclient.Client
	scheme      *runtime.Scheme
	Deployments []*appsv1.Deployment
	URL         *knapis.URL
}

func NewMultiNodeProberReconciler(client kclient.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	multiNodeProberConfig *v1beta1.MultiNodeProberConfig) *MultiNodeProberReconciler {
	deployments := make([]*appsv1.Deployment, 0)
	for i := 0; i < *componentExt.MinReplicas; i++ {
		url := &knapis.URL{}
		url.Scheme = "http"
		url.Host = fmt.Sprintf("%s-%d.%s.svc.cluster.local", componentMeta.Name, i, componentMeta.Namespace)
		dply := createRawDeployment(componentMeta, multiNodeProberConfig, url, i)
		deployments = append(deployments, dply)
	}
	return &MultiNodeProberReconciler{
		client:      client,
		scheme:      scheme,
		Deployments: deployments,
	}
}

func createRawDeployment(componentMeta metav1.ObjectMeta, multiNodeProberConfig *v1beta1.MultiNodeProberConfig,
	url *knapis.URL, index int) *appsv1.Deployment {
	podMetadata := componentMeta
	podMetadata.Name = fmt.Sprintf("%s-mnp-%d", componentMeta.Name, index)
	podMetadata.Labels["app"] = constants.GetRawServiceLabel(componentMeta.Name)
	utils.SetPodLabelsFromAnnotations(&podMetadata)
	podSpec := getDefaultPodSpec(multiNodeProberConfig, url, index)
	deployment := &appsv1.Deployment{
		ObjectMeta: podMetadata,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": constants.GetRawServiceLabel(componentMeta.Name),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: podMetadata,
				Spec:       *podSpec,
			},
		},
	}
	setDefaultDeploymentSpec(&deployment.Spec)
	return deployment
}

// checkDeploymentExist checks if the deployment exists?
func (r *MultiNodeProberReconciler) checkDeploymentExist(client kclient.Client, dply *appsv1.Deployment) (constants.CheckResultType, *appsv1.Deployment, error) {
	// get deployment
	existingDeployment := &appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: dply.ObjectMeta.Namespace,
		Name:      dply.ObjectMeta.Name,
	}, existingDeployment)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	// existed, check equivalence
	// for HPA scaling, we should ignore Replicas of Deployments
	ignoreFields := cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas")
	// Do a dry-run update. This will populate our local deployment object with any default values
	// that are present on the remote version.
	if err := client.Update(context.TODO(), dply, kclient.DryRunAll); err != nil {
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

func getDefaultPodSpec(multiNodeProberConfig *v1beta1.MultiNodeProberConfig, url *knapis.URL, index int) *corev1.PodSpec {
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
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/healthz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/readyz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/startupz",
						},
					},
					TimeoutSeconds:      multiNodeProberConfig.StartupTimeoutSeconds,
					PeriodSeconds:       multiNodeProberConfig.StartupPeriodSeconds,
					SuccessThreshold:    1,
					FailureThreshold:    multiNodeProberConfig.StartupFailureThreshold,
					InitialDelaySeconds: multiNodeProberConfig.StartupInitialDelaySeconds,
				},
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

// Reconcile ...
func (r *MultiNodeProberReconciler) Reconcile() error {
	// Reconcile Deployments
	for _, deployment := range r.Deployments {
		result, _, err := r.checkDeploymentExist(r.client, deployment)
		if err != nil {
			return err
		}
		log.Info("deployment reconcile", "checkResult", result, "err", err)
		var opErr error
		switch result {
		case constants.CheckResultCreate:
			opErr = r.client.Create(context.TODO(), deployment)
		case constants.CheckResultUpdate:
			opErr = r.client.Update(context.TODO(), deployment)
		}
		if opErr != nil {
			return opErr
		}
	}
	return nil
}
