package v1beta1

import (
	"context"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
)

var (
	defaultResource = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1"),
		v1.ResourceMemory: resource.MustParse("2Gi"),
	}
	// logger for the mutating webhook.
	mutatorLogger = logf.Log.WithName("inferenceservice-v1beta1-mutating-webhook")
)

// InferenceServiceDefaulter is responsible for setting default values on the InferenceService
// when created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceDefaulter struct {
}

// +kubebuilder:webhook:path=/mutate-ome-io-v1beta1-inferenceservice,mutating=true,failurePolicy=fail,groups=ome.io,resources=inferenceservices,verbs=create;update,versions=v1beta1,name=inferenceservice.ome-webhook-server.defaulter
var _ webhook.CustomDefaulter = &InferenceServiceDefaulter{}

func setResourceRequirementDefaults(requirements *v1.ResourceRequirements) {
	if requirements.Requests == nil {
		requirements.Requests = v1.ResourceList{}
	}
	for k, v := range defaultResource {
		if _, ok := requirements.Requests[k]; !ok {
			requirements.Requests[k] = v
		}
	}

	if requirements.Limits == nil {
		requirements.Limits = v1.ResourceList{}
	}
	for k, v := range defaultResource {
		if _, ok := requirements.Limits[k]; !ok {
			requirements.Limits[k] = v
		}
	}
}

func (d *InferenceServiceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		inferenceServiceValidatorLogger.Error(err, "Unable to convert object to InferenceService")
		return err
	}
	mutatorLogger.Info("Defaulting InferenceService", "namespace", isvc.Namespace, "isvc", isvc.Spec.Predictor)
	cfg, err := config.GetConfig()
	if err != nil {
		mutatorLogger.Error(err, "unable to set up client config")
		panic(err)
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		mutatorLogger.Error(err, "unable to create clientSet")
		panic(err)
	}
	configMap, err := NewInferenceServicesConfig(clientSet)
	if err != nil {
		panic(err)
	}
	deployConfig, err := NewDeployConfig(clientSet)
	if err != nil {
		panic(err)
	}
	isvc.DefaultInferenceService(configMap, deployConfig)
	return nil
}

func (isvc *InferenceService) DefaultInferenceService(config *InferenceServicesConfig, deployConfig *DeployConfig) {
	_, ok := isvc.ObjectMeta.Annotations[constants.DeploymentMode]

	if !ok && deployConfig != nil {
		if deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
			if isvc.ObjectMeta.Annotations == nil {
				isvc.ObjectMeta.Annotations = map[string]string{}
			}
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
		}
	}
	var components []Component
	if !ok {
		components = append(components, &isvc.Spec.Predictor)
	} else {
		// If this is a modelmesh predictor, we still want to do "Exactly One" validation.
		if err := validateExactlyOneImplementation(&isvc.Spec.Predictor); err != nil {
			mutatorLogger.Error(ExactlyOneErrorFor(&isvc.Spec.Predictor), "Missing component implementation")
		}
	}

	for _, component := range components {
		if !reflect.ValueOf(component).IsNil() {
			if err := validateExactlyOneImplementation(component); err != nil {
				mutatorLogger.Error(ExactlyOneErrorFor(component), "Missing component implementation")
			} else {
				component.GetImplementation().Default(config)
				component.GetExtensions().Default(config)
			}
		}
	}
}
