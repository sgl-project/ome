package benchmarkjobpvc

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"

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

var log = logf.Log.WithName("BenchmarkPersistentVolumeClaimReconciler")

type PVCReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewBenchmarkJobPVCReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *PVCReconciler {
	return &PVCReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *PVCReconciler) Reconcile(benchmarkJob *v1beta1.BenchmarkJob, baseModelName string) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolumeClaim", "benchmark job", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)

	pvcName := constants.PVCName(benchmarkJob.Name, baseModelName)
	if err := c.reconcilePVC(benchmarkJob, pvcName, "999Gi", corev1.ReadWriteOnce, constants.PVName(benchmarkJob.Name, benchmarkJob.Namespace, baseModelName)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcilePVC handles the creation or update of PersistentVolumeClaims
func (c *PVCReconciler) reconcilePVC(benchmarkJob *v1beta1.BenchmarkJob, pvcName, storageSize string, accessMode corev1.PersistentVolumeAccessMode, volumeName string) error {
	_, err := c.clientset.CoreV1().PersistentVolumeClaims(benchmarkJob.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating PersistentVolumeClaim", "pvc", pvcName, "benchmark job", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)
		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: benchmarkJob.Namespace,
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
		if err := controllerutil.SetControllerReference(benchmarkJob, newPVC, c.scheme); err != nil {
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
