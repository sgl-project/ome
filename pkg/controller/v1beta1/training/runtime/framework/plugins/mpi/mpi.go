package mpi

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
)

type MPI struct {
	client client.Client
}

var _ framework.EnforceMLPolicyPlugin = (*MPI)(nil)
var _ framework.CustomValidationPlugin = (*MPI)(nil)

const Name = "MPI"

func New(_ context.Context, client client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &MPI{
		client: client,
	}, nil
}

func (m *MPI) Name() string {
	return Name
}

func (m *MPI) EnforceMLPolicy(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.MPI == nil {
		return nil
	}
	// TODO: Need to implement main logic.
	return nil
}

// TODO: Need to implement validations for MPIJob.
func (m *MPI) Validate(oldObj, newObj *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}
