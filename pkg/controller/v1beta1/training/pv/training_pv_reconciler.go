package pv

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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("TrainingPersistentVolumeReconciler")

type PVReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewTrainingPVReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVReconciler {
	return &PVReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *PVReconciler) Reconcile(trainjob *v1beta1.TrainingJob, baseModelSpec *v1beta1.BaseModelSpec) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolume", "trainjob", trainjob.Name, "namespace", trainjob.Namespace)

	// Reconcile primary PersistentVolume
	pvName := constants.GetPvName(trainjob.Name, trainjob.Namespace, *trainjob.Spec.ModelConfig.InputModel)
	if err := c.reconcilePV(pvName, trainjob, *baseModelSpec.Storage.Path, "1000Gi", corev1.ReadWriteOnce); err != nil {
		return ctrl.Result{}, err
	}

	// Todo: reconcile chainsaw sidecar pv

	return ctrl.Result{}, nil
}

// reconcilePV is a helper method to create or update a PersistentVolume if it does not already exist.
func (c *PVReconciler) reconcilePV(pvName string, trainjob *v1beta1.TrainingJob, hostPath string, storageSize string, accessMode corev1.PersistentVolumeAccessMode) error {
	_, err := c.clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolume", "pv", pvName, "trainjob", trainjob.Name, "namespace", trainjob.Namespace)
		newPV := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
				Annotations: map[string]string{
					"trainingJob": trainjob.Name,
					"namespace":   trainjob.Namespace,
					"path":        hostPath,
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: "manual",
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					accessMode,
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRecycle,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: hostPath,
					},
				},
			},
		}
		if err := c.client.Create(context.TODO(), newPV); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
