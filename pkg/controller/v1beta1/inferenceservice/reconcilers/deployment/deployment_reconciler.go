package deployment

import (
	"context"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/kmp"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("DeploymentReconciler")

// DeploymentReconciler reconciles raw Kubernetes Deployment resources
type DeploymentReconciler struct {
	client       kclient.Client
	scheme       *runtime.Scheme
	Deployment   *appsv1.Deployment
	componentExt *v1beta1.ComponentExtensionSpec
}

func NewDeploymentReconciler(client kclient.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) *DeploymentReconciler {
	return &DeploymentReconciler{
		client:       client,
		scheme:       scheme,
		Deployment:   createRawDeployment(componentMeta, componentExt, podSpec),
		componentExt: componentExt,
	}
}

func createRawDeployment(componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) *appsv1.Deployment {

	podMetadata := componentMeta
	podMetadata.Labels["app"] = constants.GetRawServiceLabel(componentMeta.Name)
	utils.SetPodLabelsFromAnnotations(&podMetadata)
	setDefaultPodSpec(podSpec)

	deployment := &appsv1.Deployment{
		ObjectMeta: componentMeta,
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

	if componentExt.DeploymentStrategy != nil {
		deployment.Spec.Strategy = *componentExt.DeploymentStrategy
	}

	setDefaultDeploymentSpec(&deployment.Spec)

	// Only update deployment name after we use it to populate pod selector label value
	updateDeploymentName(deployment)
	return deployment
}

func (r *DeploymentReconciler) checkDeploymentExist() (constants.CheckResultType, *appsv1.Deployment, error) {
	existingDeployment := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Deployment.ObjectMeta.Namespace,
		Name:      r.Deployment.ObjectMeta.Name,
	}, existingDeployment)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	// Ignore fields related to HPA scaling
	ignoreFields := cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas")

	// Perform a dry-run update to populate default values
	if err := r.client.Update(context.TODO(), r.Deployment, kclient.DryRunAll); err != nil {
		log.Error(err, "Failed to perform dry-run update of deployment", "namespace", r.Deployment.Namespace, "name", r.Deployment.Name)
		return constants.CheckResultUnknown, nil, err
	}

	diff, err := kmp.SafeDiff(r.Deployment.Spec, existingDeployment.Spec, ignoreFields)
	if err != nil {
		return constants.CheckResultUnknown, nil, err
	}
	if diff != "" {
		log.Info("Deployments differ", "namespace", r.Deployment.Namespace, "name", r.Deployment.Name, "diff", diff)
		return constants.CheckResultUpdate, existingDeployment, nil
	}
	return constants.CheckResultExisted, existingDeployment, nil
}

func setDefaultPodSpec(podSpec *corev1.PodSpec) {
	if podSpec.DNSPolicy == "" {
		podSpec.DNSPolicy = corev1.DNSClusterFirst
	}
	if podSpec.RestartPolicy == "" {
		podSpec.RestartPolicy = corev1.RestartPolicyAlways
	}
	if podSpec.TerminationGracePeriodSeconds == nil {
		terminationGracePeriodSeconds := int64(corev1.DefaultTerminationGracePeriodSeconds)
		podSpec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
	}
	if podSpec.SecurityContext == nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if podSpec.SchedulerName == "" {
		podSpec.SchedulerName = corev1.DefaultSchedulerName
	}
	setDefaultContainerSettings(podSpec)
}

func setDefaultContainerSettings(podSpec *corev1.PodSpec) {
	for i := range podSpec.Containers {
		container := &podSpec.Containers[i]
		if container.TerminationMessagePath == "" {
			container.TerminationMessagePath = "/dev/termination-log"
		}
		if container.TerminationMessagePolicy == "" {
			container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		if container.ImagePullPolicy == "" {
			container.ImagePullPolicy = corev1.PullIfNotPresent
		}
		setDefaultReadinessProbe(container)
	}
}

func setDefaultReadinessProbe(container *corev1.Container) {
	if container.Name == constants.MainContainerName || container.Name == constants.TransformerContainerName {
		if container.ReadinessProbe == nil {
			port := int32(8080)
			if len(container.Ports) > 0 {
				port = container.Ports[0].ContainerPort
			}
			container.ReadinessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{
							IntVal: port,
						},
					},
				},
				TimeoutSeconds:   1,
				PeriodSeconds:    10,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			}
		}
	}
}

func setDefaultDeploymentSpec(spec *appsv1.DeploymentSpec) {
	if spec.Strategy.Type == "" {
		spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	}
	if spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType && spec.Strategy.RollingUpdate == nil {
		spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
			MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
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

func (r *DeploymentReconciler) Reconcile() (*appsv1.Deployment, error) {
	checkResult, deployment, err := r.checkDeploymentExist()
	if err != nil {
		return nil, err
	}
	log.Info("Reconciling deployment", "namespace", r.Deployment.Namespace, "name", r.Deployment.Name, "checkResult", checkResult.String())

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.Deployment)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.Deployment)
	default:
		return deployment, nil
	}

	if opErr != nil {
		log.Error(opErr, "Failed to reconcile deployment", "namespace", r.Deployment.Namespace, "name", r.Deployment.Name)
		return nil, opErr
	}

	return r.Deployment, nil
}

/* Need a different name for ome.io based DAC inference service raw deployment under OME migration context since:
 *  1. Kueue required labels: kueue.x-k8s.io/queue-name & kueue.x-k8s.io/priority-class are 2 immutable fields;
 *  2. Kueue is only introduced in new OME, not old OME. So for OME migration from old OME to new OME, need to recreate a
 *     new deployment resource with a different name so new OME inference service can be up successfully with Kueue,
 *     it cannot directly update the existing old OME deployment resource due to above point #1;
 *  Note: Only need to adopt a new deployment name when it comes to migrate old OME DAC inference service, no need to do
 *        this for below:
 *     1). on-demand model serving;
 *     2). DAC inference service deployment from new OME with Volcano reconciled; (Out of scope, will handle its migration
 *         separately)
 */
func updateDeploymentName(deployment *appsv1.Deployment) {
	if _, ok := deployment.Annotations[constants.DedicatedAICluster]; ok {
		if _, ok = deployment.Annotations[constants.VolcanoScheduler]; !ok {
			deployment.Name = fmt.Sprintf("%s-%s", deployment.Name, "new")
		}
	}
}
