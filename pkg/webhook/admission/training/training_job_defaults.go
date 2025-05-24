package training

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/constants"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// TrainingJobDefaulter is responsible for setting default values on the TrainingJob
// when created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
// +k8s:openapi-gen=false
// +kubebuilder:object:generate=false
type TrainingJobDefaulter struct {
}

var (
	// logger for the mutating webhook.
	trainingJobMutatorLogger = logf.Log.WithName("trainingjob-v1beta1-mutating-webhook")
)

// +kubebuilder:webhook:path=/mutate-ome-io-v1beta1-trainingjob,mutating=true,failurePolicy=fail,groups=ome.io,resources=trainingjobs,verbs=create;update,versions=v1beta1,name=trainingjob.ome-webhook-server.defaulter
var _ webhook.CustomDefaulter = &TrainingJobDefaulter{}

func (tjd *TrainingJobDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	tjob, err := convertToTrainingJob(obj)
	if err != nil {
		trainingJobValidatorLogger.Error(err, "Unable to convert object to TrainingJob")
		return err
	}
	trainingJobMutatorLogger.Info("Defaulting TrainingJob", "namespace", tjob.Namespace, "name", tjob.Name)
	cfg, err := config.GetConfig()
	if err != nil {
		trainingJobMutatorLogger.Error(err, "unable to set up client config")
		panic(err)
	}

	trainingJobMutatorLogger.Info("Config", "config", cfg)

	DefaultTrainingJob(tjob)
	return nil
}

func DefaultTrainingJob(tjob *v1beta1.TrainingJob) {
	// Specify BaseModelName annotation so the mutator can inject init container for cohere model
	if tjob.Spec.Annotations == nil {
		tjob.Spec.Annotations = make(map[string]string)
	}
	tjob.Spec.Annotations[constants.BaseModelName] = *tjob.Spec.ModelConfig.InputModel

	// Specify TrainingJobPodLabelKey label so the mutator can inject both init and sidecar container
	if tjob.Spec.Labels == nil {
		tjob.Spec.Labels = make(map[string]string)
	}

	tjob.Spec.Labels[constants.TrainingJobPodLabelKey] = tjob.Name

	if tjob.APIVersion != "ome.io/v1beta1" {
		tjob.APIVersion = "ome.io/v1beta1"
	}

	if tjob.Kind != "TrainingJob" {
		tjob.Kind = "TrainingJob"
	}
}
