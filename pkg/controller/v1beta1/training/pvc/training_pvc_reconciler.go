package pvc

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("TrainingPersistentVolumeClaimReconciler")

type PVCReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewTrainingPVCReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVCReconciler {
	return &PVCReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *PVCReconciler) Reconcile(trainjob *v1beta1.TrainingJob) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolumeClaim", "trainjob", trainjob.Name, "namespace", trainjob.Namespace)

	// Reconcile the main PVC
	pvcName := constants.GetPvcName(trainjob.Name, trainjob.Namespace, *trainjob.Spec.ModelConfig.InputModel)
	pvName := constants.GetPvName(trainjob.Name, trainjob.Namespace, *trainjob.Spec.ModelConfig.InputModel)
	if err := c.reconcilePVC(trainjob, pvcName, "999Gi", corev1.ReadWriteOnce, pvName); err != nil {
		return ctrl.Result{}, err
	}

	// Todo: reconcile chainsaw sidecar pvc

	return ctrl.Result{}, nil
}

// reconcilePVC handles the creation or update of PersistentVolumeClaims
func (c *PVCReconciler) reconcilePVC(trainjob *v1beta1.TrainingJob, pvcName, storageSize string, accessMode corev1.PersistentVolumeAccessMode, volumeName string) error {
	_, err := c.clientset.CoreV1().PersistentVolumeClaims(trainjob.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolumeClaim", "pvc", pvcName, "trainjob", trainjob.Name, "namespace", trainjob.Namespace)
		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: trainjob.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: stringPtr("manual"),
				AccessModes: []corev1.PersistentVolumeAccessMode{
					accessMode,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(storageSize),
					},
				},
				VolumeName: volumeName,
			},
		}
		if err := controllerutil.SetControllerReference(trainjob, newPVC, c.scheme); err != nil {
			return err
		}
		if err := c.client.Create(context.TODO(), newPVC); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// stringPtr returns a pointer to the string
func stringPtr(s string) *string {
	return &s
}
