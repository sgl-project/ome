package isvc

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

var (
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
	deployConfig, err := controllerconfig.NewDeployConfig(clientSet)
	if err != nil {
		panic(err)
	}
	DefaultInferenceService(isvc, deployConfig)
	return nil
}

// DefaultInferenceService sets default values on the InferenceService
func DefaultInferenceService(isvc *v1beta1.InferenceService, deployConfig *controllerconfig.DeployConfig) {
	_, ok := isvc.ObjectMeta.Annotations[constants.DeploymentMode]

	if !ok && deployConfig != nil {
		if deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
			if isvc.ObjectMeta.Annotations == nil {
				isvc.ObjectMeta.Annotations = map[string]string{}
			}
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
		}
	}
}
