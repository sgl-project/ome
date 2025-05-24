package training

import (
	"context"
	"fmt"
	"strings"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// TrainingjobValidator is responsible for validating the TrainingJob resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type TrainingjobValidator struct{}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-trainingjob,mutating=false,failurePolicy=fail,groups=ome.io,resources=trainingjobs,versions=v1beta1,name=trainingjob.ome-webhook-server.validator
var _ webhook.CustomValidator = &TrainingjobValidator{}

var (
	// logger for the validation webhook.
	trainingJobValidatorLogger = logf.Log.WithName("trainingjob-v1beta1-validation-webhook")
)

func (t TrainingjobValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	tjob, err := convertToTrainingJob(obj)
	if err != nil {
		trainingJobValidatorLogger.Error(err, "Unable to convert object to TrainingJob")
		return nil, err
	}
	trainingJobValidatorLogger.Info("validate create", "name", tjob.Name)
	return validateTrainingJob(tjob)
}

func (t TrainingjobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	tjob, err := convertToTrainingJob(newObj)
	if err != nil {
		trainingJobValidatorLogger.Error(err, "Unable to convert object to TrainingJob")
		return nil, err
	}
	trainingJobValidatorLogger.Info("validate update", "name", tjob.Name)
	return validateTrainingJob(tjob)
}

func (t TrainingjobValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	tjob, err := convertToTrainingJob(obj)
	if err != nil {
		trainingJobValidatorLogger.Error(err, "Unable to convert object to TrainingJob")
		return nil, err
	}
	trainingJobValidatorLogger.Info("validate delete", "name", tjob.Name)
	return validateTrainingJob(tjob)
}

func validateTrainingJob(tjob *v1beta1.TrainingJob) (admission.Warnings, error) {
	var allWarnings admission.Warnings

	if err := validateTrainingJobName(tjob); err != nil {
		return allWarnings, err
	}

	return allWarnings, nil
}

// Validation of TrainingJob name
func validateTrainingJobName(tjob *v1beta1.TrainingJob) error {
	if !strings.HasPrefix(tjob.Name, "ft-") {
		return fmt.Errorf("invalid TrainingJob name %T, valid training job name starts with 'ft-'", tjob.Name)
	}
	return nil
}

// Convert runtime.Object into TrainingJob
func convertToTrainingJob(obj runtime.Object) (*v1beta1.TrainingJob, error) {
	tjob, ok := obj.(*v1beta1.TrainingJob)
	if !ok {
		return nil, fmt.Errorf("expected an TrainingJob object but got %T", obj)
	}
	return tjob, nil
}
