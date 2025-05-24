package training

import (
	"context"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	v1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
)

var log = logf.Log.WithName(constants.TrainingRuntimeValidatorWebhookName)

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-trainingruntime,mutating=false,failurePolicy=fail,groups=ome.io,resources=trainingruntimes,versions=v1beta1,name=trainingruntime.ome-webhook-server.validator

type TrainingRuntimeValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-clustertrainingruntime,mutating=false,failurePolicy=fail,groups=ome.io,resources=clustertrainingruntimes,versions=v1beta1,name=clustertrainingruntime.ome-webhook-server.validator

type ClusterTrainingRuntimeValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

func (tr *TrainingRuntimeValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	trainingRuntime := &v1beta1.TrainingRuntime{}
	if err := tr.Decoder.Decode(req, trainingRuntime); err != nil {
		log.Error(err, "Failed to decode training runtime", "name", trainingRuntime.Name, "namespace", trainingRuntime.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := validateReplicatedJobs(trainingRuntime.Spec.Template.Spec.ReplicatedJobs); err != nil {
		log.Error(err, "Validation failed for TrainingRuntime", "name", trainingRuntime.Name, "namespace", trainingRuntime.Namespace)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func (ctr *ClusterTrainingRuntimeValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	clusterTrainingRuntime := &v1beta1.ClusterTrainingRuntime{}
	if err := ctr.Decoder.Decode(req, clusterTrainingRuntime); err != nil {
		log.Error(err, "Failed to decode cluster training runtime", "name", clusterTrainingRuntime.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := validateReplicatedJobs(clusterTrainingRuntime.Spec.Template.Spec.ReplicatedJobs); err != nil {
		log.Error(err, "Validation failed for ClusterTrainingRuntime", "name", clusterTrainingRuntime.Name)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func validateReplicatedJobs(rJobs []jobsetv1alpha2.ReplicatedJob) error {
	for i, rJob := range rJobs {
		if rJob.Replicas != 1 {
			return fmt.Errorf("replicas for job %d must be 1, got %d", i, rJob.Replicas)
		}
	}
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-trainingjob,mutating=false,failurePolicy=fail,groups=ome.io,resources=trainingjobs,versions=v1beta1,name=trainingjob.ome-webhook-server.validator

type TrainingJobValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

func (tj *TrainingJobValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	trainingJob := &v1beta1.TrainingJob{}
	if err := tj.Decoder.Decode(req, trainingJob); err != nil {
		log.Error(err, "Failed to decode training job", "name", trainingJob.Name, "namespace", trainingJob.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// TODO: Add validation logic here

	return admission.Allowed("")
}
