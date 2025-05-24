package multimodelconfig

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/modelconfig"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/constants"
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
	log.Info("Checking existence of modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)

	// Retrieve the ConfigMap
	_, err := c.clientset.CoreV1().ConfigMaps(isvc.Namespace).Get(context.TODO(), modelConfigName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// ConfigMap is not found, create a new one
		log.Info("Creating modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		return c.createModelConfig(isvc, modelConfigName)
	} else if err != nil {
		// Unexpected error while retrieving ConfigMap
		log.Error(err, "Failed to get ConfigMap", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		return ctrl.Result{}, err
	}

	// ConfigMap exists, you may want to update it here if necessary
	log.Info("ModelConfig already exists, no action required", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
	// Additional update logic can be added here if needed

	return ctrl.Result{}, nil
}

func (c *ConfigMapReconciler) createModelConfig(isvc *v1beta1.InferenceService, modelConfigName string) (ctrl.Result, error) {
	newModelConfig, err := modelconfig.CreateEmptyModelConfig(isvc)
	if err != nil {
		log.Error(err, "Failed to create empty modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(isvc, newModelConfig, c.scheme); err != nil {
		log.Error(err, "Failed to set controller reference for modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		return ctrl.Result{}, err
	}

	if err := c.client.Create(context.TODO(), newModelConfig); err != nil {
		log.Error(err, "Failed to create ConfigMap", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
		return ctrl.Result{}, err
	}

	log.Info("Successfully created modelConfig", "configmap", modelConfigName, "inference service", isvc.Name, "namespace", isvc.Namespace)
	return ctrl.Result{}, nil
}
