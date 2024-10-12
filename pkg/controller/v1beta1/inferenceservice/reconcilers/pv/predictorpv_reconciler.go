package predictorpv

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	isvcutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"context"
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

var log = logf.Log.WithName("PredictorPersistentVolumeReconciler")

type PVReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewPredictorPVReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVReconciler {
	return &PVReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

// Reconcile handles the creation of PersistentVolumes based on the InferenceService and BaseModelSpec.
func (c *PVReconciler) Reconcile(isvc *v1beta1.InferenceService, baseModelSpec *v1beta1.BaseModelSpec) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolume", "inference service", isvc.Name, "namespace", isvc.Namespace)

	// Reconcile primary PersistentVolume
	pvName := constants.PVName(isvc.Name, isvc.Namespace, *isvc.Spec.Predictor.Model.BaseModel)
	if err := c.reconcilePV(pvName, isvc, *baseModelSpec.Storage.StorageUri, "1000Gi", corev1.ReadWriteOnce); err != nil {
		return ctrl.Result{}, err
	}

	// Check if Chainsaw sidecar injection is enabled
	if isvcutils.IsChainsawInjectEnabled(isvc.Annotations) {
		// Reconcile PersistentVolumes for Chainsaw sidecar
		for _, item := range constants.OCIETCHostPaths {
			pvName := constants.PVName(isvc.Name, isvc.Namespace, item.Name)
			if err := c.reconcilePV(pvName, isvc, item.HostPath, "10Mi", corev1.ReadOnlyMany); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePV is a helper method to create or update a PersistentVolume if it does not already exist.
func (c *PVReconciler) reconcilePV(pvName string, isvc *v1beta1.InferenceService, hostPath string, storageSize string, accessMode corev1.PersistentVolumeAccessMode) error {
	_, err := c.clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolume", "pv", pvName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		newPV := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
				Annotations: map[string]string{
					"inferenceService": isvc.Name,
					"namespace":        isvc.Namespace,
					"path":             hostPath,
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
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
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
