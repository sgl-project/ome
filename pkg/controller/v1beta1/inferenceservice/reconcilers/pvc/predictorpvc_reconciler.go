package predictorpvc

import (
	isvcutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
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

var log = logf.Log.WithName("PredictorPersistentVolumeClaimReconciler")

type PVCReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewPredictorPVCReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVCReconciler {
	return &PVCReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *PVCReconciler) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolumeClaim", "inference service", isvc.Name, "namespace", isvc.Namespace)

	// Reconcile the main PVC
	pvcName := constants.PVCName(isvc.Name, *isvc.Spec.Predictor.Model.BaseModel)
	if err := c.reconcilePVC(isvc, pvcName, "999Gi", corev1.ReadWriteOnce, constants.PVName(isvc.Name, isvc.Namespace, *isvc.Spec.Predictor.Model.BaseModel)); err != nil {
		return ctrl.Result{}, err
	}

	// If Chainsaw Sidecar is enabled, reconcile additional PVCs
	if isvcutils.IsChainsawInjectEnabled(isvc.Annotations) {
		for name, _ := range constants.OCIETCHostPath {
			pvcName := constants.PVCName(isvc.Name, name)
			if err := c.reconcilePVC(isvc, pvcName, "10Mi", corev1.ReadOnlyMany, constants.PVName(isvc.Name, isvc.Namespace, name)); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePVC handles the creation or update of PersistentVolumeClaims
func (c *PVCReconciler) reconcilePVC(isvc *v1beta1.InferenceService, pvcName, storageSize string, accessMode corev1.PersistentVolumeAccessMode, volumeName string) error {
	_, err := c.clientset.CoreV1().PersistentVolumeClaims(isvc.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolumeClaim", "pvc", pvcName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: isvc.Namespace,
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
		if err := controllerutil.SetControllerReference(isvc, newPVC, c.scheme); err != nil {
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
