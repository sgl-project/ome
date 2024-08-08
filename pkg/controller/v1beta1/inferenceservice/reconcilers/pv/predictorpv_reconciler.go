package predictorpv

import (
	"context"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
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

func (c *PVReconciler) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	log.Info("Reconciling PersistentVolume", "inference service", isvc.Name, "namespace", isvc.Namespace)
	pvName := constants.PVName(isvc.Name)

	// Check if the PersistentVolume needs to be deleted
	if shouldDeletePV(isvc) {
		if err := c.ForceDeletePV(pvName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

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
					Local: &corev1.LocalVolumeSource{
						Path: "/raid/models",
					},
				},
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/instance-type",
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"BM.GPU2.2", "BM.GPU3.8", "BM.GPU4.8",
											"VM.GPU2.1", "VM.GPU3.1", "VM.GPU3.2",
											"VM.GPU3.4", "BM.GPU.T1.2", "BM.GPU.T1-2.4",
											"BM.GPU.A100-v2.8", "BM.GPU.GM4.8", "BM.GPU.A10.4",
											"BM.GPU.GU1.4", "VM.GPU.A10.1", "VM.GPU.GU1.1",
											"VM.GPU.A10.2", "VM.GPU.GU1.2", "BM.GPU.B4.8",
											"BM.GPU.H100.8",
										},
									},
								},
							},
						},
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

func shouldDeletePV(isvc *v1beta1.InferenceService) bool {
	return !isvc.DeletionTimestamp.IsZero()
}

func (c *PVReconciler) ForceDeletePV(pvName string) error {
	log.Info("Force deleting PersistentVolume", "pv", pvName)
	pv := &corev1.PersistentVolume{}
	if err := c.client.Get(context.TODO(), client.ObjectKey{Name: pvName}, pv); err != nil {
		return err
	}
	deletePolicy := metav1.DeletePropagationForeground
	if err := c.client.Delete(context.TODO(), pv, &client.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		return err
	}
	return nil
}
