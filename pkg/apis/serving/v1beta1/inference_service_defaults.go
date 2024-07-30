package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
)

var (
	defaultResource = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1"),
		v1.ResourceMemory: resource.MustParse("2Gi"),
	}
	// logger for the mutating webhook.
	mutatorLogger = logf.Log.WithName("inferenceservice-v1beta1-mutating-webhook")
)

// +kubebuilder:webhook:path=/mutate-inferenceservices,mutating=true,failurePolicy=fail,groups=ome.oracle.com,resources=inferenceservices,verbs=create;update,versions=v1beta1,name=inferenceservice.ome-webhook-server.defaulter
var _ webhook.Defaulter = &InferenceService{}

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

func (isvc *InferenceService) Default() {
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
		// Only attempt to assign runtimes and apply defaulting logic for non-modelmesh predictors
		isvc.setPredictorModelDefaults()
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

func (isvc *InferenceService) setPredictorModelDefaults() {
	openAIProtocol := constants.OpenAIProtocol
	isvc.Spec.Predictor.Model.ProtocolVersion = &openAIProtocol
}

func (isvc *InferenceService) SetRuntimeDefaults() {
	switch {
	case *isvc.Spec.Predictor.Model.Runtime == constants.TritonServer:
		isvc.SetTritonDefaults()

	case *isvc.Spec.Predictor.Model.Runtime == constants.VLLMServer:
		isvc.SetVLLMDefaults()

	case *isvc.Spec.Predictor.Model.Runtime == constants.TGIServer:
		isvc.SetTGIServerDefaults()
	}
}

func (isvc *InferenceService) SetVLLMDefaults() {
	if isvc.Spec.Predictor.Model.ProtocolVersion == nil {
		protocolV2 := constants.OpenAIProtocol
		isvc.Spec.Predictor.Model.ProtocolVersion = &protocolV2
	}
}

func (isvc *InferenceService) SetTGIServerDefaults() {
	if isvc.Spec.Predictor.Model.ProtocolVersion == nil {
		protocolV2 := constants.OpenAIProtocol
		isvc.Spec.Predictor.Model.ProtocolVersion = &protocolV2
	}
}

func (isvc *InferenceService) SetTritonDefaults() {
	if isvc.Spec.Predictor.Model.ProtocolVersion == nil {
		protocolV2 := constants.OpenInferenceProtocolV2
		isvc.Spec.Predictor.Model.ProtocolVersion = &protocolV2
	}
}
