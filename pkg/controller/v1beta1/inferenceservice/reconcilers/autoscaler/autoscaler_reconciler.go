package autoscaler

import (
	"context"
	"fmt"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	hpa "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/hpa"
	keda "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/keda"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Autoscaler Interface implemented by all autoscalers
type Autoscaler interface {
	Reconcile() (runtime.Object, error)
	SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error
}

// NoOpAutoscaler Autoscaler that does nothing. Can be used to disable creation of autoscaler resources.
type NoOpAutoscaler struct{}

func (*NoOpAutoscaler) Reconcile() (*autoscalingv2.HorizontalPodAutoscaler, error) {
	return nil, nil
}

func (a *NoOpAutoscaler) SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error {
	return nil
}

var log = logf.Log.WithName("AutoscalerReconciler")

// AutoscalerReconciler is the struct of Raw K8S Object
type AutoscalerReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	Autoscaler   Autoscaler
	componentExt *v1beta1.ComponentExtensionSpec
}

func NewAutoscalerReconciler(
	client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	inferenceServiceSpec *v1beta1.InferenceServiceSpec,
) (*AutoscalerReconciler, error) {
	as, err := createAutoscaler(client, clientset, scheme, componentMeta, inferenceServiceSpec)
	if err != nil {
		return nil, err
	}
	return &AutoscalerReconciler{
		client:       client,
		scheme:       scheme,
		Autoscaler:   as,
		componentExt: &inferenceServiceSpec.Predictor.ComponentExtensionSpec,
	}, err
}

func getAutoscalerClass(metadata metav1.ObjectMeta) constants.AutoscalerClassType {
	annotations := metadata.Annotations
	if value, ok := annotations[constants.AutoscalerClass]; ok {
		return constants.AutoscalerClassType(value)
	} else {
		return constants.DefaultAutoscalerClass
	}
}

func createAutoscaler(client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme, componentMeta metav1.ObjectMeta,
	inferenceServiceSpec *v1beta1.InferenceServiceSpec,
) (Autoscaler, error) {
	log.Info("Current Autoscaler Class Info", "componentMeta", componentMeta, "inferenceServiceSpec", inferenceServiceSpec)
	ac := getAutoscalerClass(componentMeta)

	switch ac {
	// HPA and KEDA can not coexist for the same deployment
	case constants.AutoscalerClassHPA, constants.AutoscalerClassExternal:
		// Before creating HPA, ensure any existing ScaledObject is deleted
		err := deleteExistingScaledObject(client, componentMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing ScaledObject: %w", err)
		}
		return hpa.NewHPAReconciler(client, scheme, componentMeta, &inferenceServiceSpec.Predictor.ComponentExtensionSpec), nil
	case constants.AutoscalerClassKEDA:
		// Before creating ScaledObject, ensure any existing HPA is deleted
		err := deleteExistingHPA(client, componentMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing HPA: %w", err)
		}
		return keda.NewKEDAReconciler(client, scheme, componentMeta, inferenceServiceSpec)
	default:
		return nil, fmt.Errorf("unknown autoscaler class type: %v", ac)
	}
}

// deleteExistingScaledObject deletes any existing ScaledObject for the Deployment
func deleteExistingScaledObject(client client.Client, componentMeta metav1.ObjectMeta) error {
	scaledObjectName := utils.GetScaledObjectName(componentMeta.Name)
	scaledObject := &kedav1.ScaledObject{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: componentMeta.Namespace,
		Name:      scaledObjectName,
	}, scaledObject)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil
		}
		return err
	}
	// Delete the existing ScaledObject
	return client.Delete(context.TODO(), scaledObject)
}

// deleteExistingHPA deletes any existing HPA for the Deployment
func deleteExistingHPA(client client.Client, componentMeta metav1.ObjectMeta) error {
	hpaName := componentMeta.Name
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: componentMeta.Namespace,
		Name:      hpaName,
	}, hpa)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil
		}
		return err
	}
	// Delete the existing HPA
	return client.Delete(context.TODO(), hpa)
}

// Reconcile ...
func (r *AutoscalerReconciler) Reconcile() error {
	// reconcile Autoscaler
	_, err := r.Autoscaler.Reconcile()
	if err != nil {
		return err
	}
	return nil
}
