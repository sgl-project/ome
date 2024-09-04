package predictorpvc

import (
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
	pvcName := constants.PVCName(isvc.Name)

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
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("999Gi"),
					},
				},
				VolumeName: constants.PVName(isvc.Name, isvc.Namespace),
			},
		}
		if err := controllerutil.SetControllerReference(isvc, newPVC, c.scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := c.client.Create(context.TODO(), newPVC); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func stringPtr(s string) *string {
	return &s
}
