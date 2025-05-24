package torch

import (
	"context"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
)

type Torch struct{}

var _ framework.EnforceMLPolicyPlugin = (*Torch)(nil)
var _ framework.CustomValidationPlugin = (*Torch)(nil)

const Name = "Torch"

func New(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &Torch{}, nil
}

func (t *Torch) Name() string {
	return Name
}

// TODO: Add support for PyTorch elastic when JobSet supports Elastic Jobs.
func (t *Torch) EnforceMLPolicy(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.Torch == nil {
		return nil
	}

	// TrainJob contains the actual information for the Trainer.
	numNodes := info.RuntimePolicy.MLPolicy.NumNodes
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		numNodes = trainJob.Spec.Trainer.NumNodes
	}
	info.Trainer.NumNodes = numNodes

	numProcPerNode := info.RuntimePolicy.MLPolicy.Torch.NumProcPerNode
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumProcPerNode != nil {
		numProcPerNode = trainJob.Spec.Trainer.NumProcPerNode
	}

	// Update envs for Info object.
	// Add PyTorch distributed "PET_" values for torchrun
	// TODO: Add validation to check that TrainJob doesn't have "PET_" envs.
	// TODO: We should validate that envs from different plugins don't conflict with each other.
	// Ref: https://github.com/kubeflow/training-operator/pull/2308#discussion_r1823229940
	infoEnvs := []corev1.EnvVar{
		{
			Name:  constants.TorchEnvNumNodes,
			Value: fmt.Sprintf("%d", ptr.Deref(numNodes, 1)),
		},
		{
			Name:  constants.TorchEnvNumProcPerNode,
			Value: ptr.Deref(numProcPerNode, "auto"),
		},
		{
			Name: constants.TorchEnvNodeRank,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: constants.JobCompletionIndexFieldPath,
				},
			},
		},
		{
			Name:  constants.TorchEnvMasterAddr,
			Value: fmt.Sprintf("%s-%s-0-0.%s", utils.GetShortTrainJobName(trainJob.Name), constants.JobTrainerNode, utils.GetShortTrainJobName(trainJob.Name)),
		},
		{
			Name:  constants.TorchEnvMasterPort,
			Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
		},
	}

	// Set for all Info envs.
	envNames := sets.New[string]()
	for _, env := range infoEnvs {
		envNames.Insert(env.Name)
	}
	// Info envs take precedence over TrainJob envs.
	if trainJob.Spec.Trainer != nil {
		for _, env := range trainJob.Spec.Trainer.Env {
			if !envNames.Has(env.Name) {
				info.Trainer.Env = append(info.Trainer.Env, corev1.EnvVar{Name: env.Name, Value: env.Value})
			}
		}
	}
	// Insert Torch distributed envs into the list end.
	info.Trainer.Env = append(info.Trainer.Env, infoEnvs...)

	// Add container port for the headless service.
	info.Trainer.ContainerPort = &corev1.ContainerPort{
		ContainerPort: constants.ContainerTrainerPort,
	}

	// Update total Pod requests for the PodGroupPolicy plugin.
	for rName := range info.TotalRequests {
		// For other Jobs like the Initializer, replica is always equal to 1.
		// TODO: Add support for total requests from the TrainJob's ResourcesPerNode.
		if rName == constants.JobTrainerNode {
			info.TotalRequests[rName] = runtime.TotalResourceRequest{
				Replicas:    ptr.Deref(numNodes, constants.DefaultJobReplicas),
				PodRequests: info.TotalRequests[rName].PodRequests,
			}
		}
	}

	return nil
}

// TODO: Need to implement validateions for TorchJob.
func (t *Torch) Validate(oldObj, newObj *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}
