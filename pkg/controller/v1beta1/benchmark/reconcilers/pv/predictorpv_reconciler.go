package benchmarkjobpv

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
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

var log = logf.Log.WithName("BenchmarkPersistentVolumeReconciler")

type PVReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewBenchmarkJobPVReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVReconciler {
	return &PVReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

// Reconcile handles the creation of PersistentVolumes based on the BenchmarkJob, BaseModelName and BaseModelSpec.
func (c *PVReconciler) Reconcile(benchmarkJob *v1beta1.BenchmarkJob, baseModelName string, baseModelSpec *v1beta1.BaseModelSpec) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolume", "benchmark job", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)

	// Reconcile primary PersistentVolume
	pvName := constants.PVName(benchmarkJob.Name, benchmarkJob.Namespace, baseModelName)
	if err := c.reconcilePV(pvName, benchmarkJob, *baseModelSpec.Storage.StorageUri, "1000Gi", corev1.ReadWriteOnce); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcilePV is a helper method to create or update a PersistentVolume if it does not already exist.
// It takes the PersistentVolume name, BenchmarkJob object, host path, storage size, and access mode as parameters.
// The PersistentVolume is created if it does not already exist, and the annotations are updated with the BenchmarkJob name and namespace.
func (c *PVReconciler) reconcilePV(pvName string, benchmarkJob *v1beta1.BenchmarkJob, hostPath string, storageSize string, accessMode corev1.PersistentVolumeAccessMode) error {
	// Get the PersistentVolume
	_, err := c.clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// Create the PersistentVolume if it does not already exist
		log.Info("Creating PersistentVolume", "pv", pvName, "benchmark job", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)
		newPV := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
				Annotations: map[string]string{
					"benchmarkJob": benchmarkJob.Name,
					"namespace":    benchmarkJob.Namespace,
					"path":         hostPath,
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
		// Return the error if it is not a NotFound error
		return err
	}
	return nil
}
