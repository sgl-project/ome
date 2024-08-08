package multimodelconfig

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/modelconfig"
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("ModelConfigMapReconciler")

type ConfigMapReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewModelConfigReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *ConfigMapReconciler) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	log.Info("Reconciling ModelConfig", "inference service", isvc.Name, "namespace", isvc.Namespace)
	modelConfigName := constants.ModelConfigName(isvc.Name)
	log.Info("Reconciling modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)

	_, err := c.clientset.CoreV1().ConfigMaps(isvc.Namespace).Get(context.TODO(), modelConfigName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
			newModelConfig, err := modelconfig.CreateEmptyModelConfig(isvc)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := controllerutil.SetControllerReference(isvc, newModelConfig, c.scheme); err != nil {
				return ctrl.Result{}, err
			}
			err = c.client.Create(context.TODO(), newModelConfig)
			if err != nil {
				return ctrl.Result{}, err
			}

		}
	}
	log.Info("Updating modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
	return ctrl.Result{}, nil
}
