package predictorpv

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
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

func (c *PVReconciler) Reconcile(isvc *v1beta1.InferenceService, baseModelSpec *v1beta1.BaseModelSpec) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolume", "inference service", isvc.Name, "namespace", isvc.Namespace)
	pvName := constants.PVName(isvc.Name, isvc.Namespace)

	_, err := c.clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolume", "pv", pvName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		newPV := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
				Annotations: map[string]string{
					"inferenceService": isvc.Name,
					"namespace":        isvc.Namespace,
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: "manual",
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1000Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: *baseModelSpec.Storage.StorageUri,
					},
				},
			},
		}
		if err := c.client.Create(context.TODO(), newPV); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
