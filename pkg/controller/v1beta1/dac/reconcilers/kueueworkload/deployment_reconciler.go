package kueueworkload

import (
	"context"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmp"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("ReservationDeploymentReconciler")

// DeploymentReconciler reconciles a native Kubernetes Deployment resource
type DeploymentReconciler struct {
	client                    client.Client
	scheme                    *runtime.Scheme
	ReservationWorkloadConfig *controllerconfig.DacReservationWorkloadConfig
	Deployment                *appsv1.Deployment
}

func NewDeploymentReconciler(
	client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	namespace string,
	resources *corev1.ResourceRequirements,
	affinity *corev1.Affinity,
	count int) (*DeploymentReconciler, error) {

	reservationWorkloadConfig, err := controllerconfig.NewDacReservationWorkloadConfig(clientset)
	if err != nil {
		return nil, err
	}

	return &DeploymentReconciler{
		client:                    client,
		scheme:                    scheme,
		ReservationWorkloadConfig: reservationWorkloadConfig,
		Deployment:                createDeployment(namespace, resources, affinity, reservationWorkloadConfig, count),
	}, nil
}

func createDeployment(
	namespace string,
	resources *corev1.ResourceRequirements,
	affinity *corev1.Affinity,
	reservationWorkloadConfig *controllerconfig.DacReservationWorkloadConfig,
	count int) *appsv1.Deployment {

	podMetadata := metav1.ObjectMeta{
		Labels: map[string]string{
			"app": constants.DACMainTaskName,
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DACMainTaskName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.KueueQueueLabelKey:                 namespace,
				constants.KueueWorkloadPriorityClassLabelKey: constants.DedicatedAiClusterReservationWorkloadPriorityClass,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": constants.DACMainTaskName,
				},
			},
			Replicas: utils.GetInt32Pointer(count),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: podMetadata,
				Spec: corev1.PodSpec{
					Affinity:                      affinity,
					SchedulerName:                 reservationWorkloadConfig.SchedulerName,
					TerminationGracePeriodSeconds: &constants.DACReservationJobTerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:  constants.DACMainTaskName,
							Image: reservationWorkloadConfig.Image,
							Command: []string{
								"/bin/bash",
							},
							Args: []string{
								"-c",
								"/bin/sleep infinity",
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									constants.NvidiaGPUResourceType: resources.Requests[corev1.ResourceName(constants.NvidiaGPUResourceType)],
								},
								Limits: corev1.ResourceList{
									constants.NvidiaGPUResourceType: resources.Requests[corev1.ResourceName(constants.NvidiaGPUResourceType)],
								},
							},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: utils.GetPointerOfIntOrString(0),
					MaxSurge:       utils.GetPointerOfIntOrString(1),
				},
			},
		},
	}

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
			log.Info("Reservation deployment not found, will be created", "namespace", r.Deployment.Namespace, "Deployment", r.Deployment.Name)
			return constants.CheckResultCreate, nil, nil
		}
		log.Error(err, "Failed to get reservation deployment", "namespace", r.Deployment.Namespace, "Deployment", r.Deployment.Name)
		return constants.CheckResultUnknown, nil, err
	}

	// Perform a dry-run update to populate default values
	if err := r.client.Update(context.TODO(), r.Deployment, client.DryRunAll); err != nil {
		log.Error(err, "Failed to perform dry-run update of reservation deployment", "namespace", r.Deployment.Namespace, "Deployment", r.Deployment.Name)
		return constants.CheckResultUnknown, nil, err
	}

	diff, err := kmp.SafeDiff(r.Deployment.Spec, existingDeployment.Spec)
	if err != nil {
		return constants.CheckResultUnknown, nil, err
	}
	if diff != "" {
		log.Info("Reservation deployment differ", "namespace", r.Deployment.Namespace, "Deployment", r.Deployment.Name, "diff", diff)
		return constants.CheckResultUpdate, existingDeployment, nil
	}

	return constants.CheckResultExisted, existingDeployment, nil
}

func (r *DeploymentReconciler) Reconcile() (*appsv1.Deployment, error) {
	checkResult, deployment, err := r.checkDeploymentExist()
	if err != nil {
		return nil, err
	}
	log.Info("Reconciling reservation deployment", "namespace", r.Deployment.Namespace, "Deployment", r.Deployment.Name, "checkResult", checkResult)

	switch checkResult {
	case constants.CheckResultCreate:
		err = r.client.Create(context.TODO(), r.Deployment)
	case constants.CheckResultUpdate:
		updateLastUpdatedTimeInAnnotation(r.Deployment)
		err = r.client.Update(context.TODO(), r.Deployment)
	default:
		return deployment, nil
	}

	if err != nil {
		log.Error(err, "Failed to reconcile reservation deployment", "namespace", r.Deployment.Namespace, "name", r.Deployment.Name)
		return nil, err
	}

	return r.Deployment, nil
}

func updateLastUpdatedTimeInAnnotation(deployment *appsv1.Deployment) {
	t := time.Now().UTC()
	formattedTime := t.Format(time.RFC3339)
	if deployment.ObjectMeta.Annotations == nil {
		deployment.ObjectMeta.Annotations = map[string]string{constants.DACLastUpdateTimeAnnotationKey: formattedTime}
	} else {
		deployment.ObjectMeta.Annotations[constants.DACLastUpdateTimeAnnotationKey] = formattedTime
	}
}
