package training

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDefaultTrainingJob_Default(t *testing.T) {
	defaulter := TrainingJobDefaulter{}

	inputModel := "input-model"
	tjob := v1beta1.TrainingJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "training-job",
		},
		Spec: v1beta1.TrainingJobSpec{
			ModelConfig: &v1beta1.ModelConfig{
				InputModel: &inputModel,
			},
		},
	}

	cases := map[string]struct {
		object      runtime.Object
		trainingJob *v1beta1.TrainingJob
	}{
		"should default training job": {
			object: tjob.DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				TypeMeta: metav1.TypeMeta{
					Kind:       "TrainingJob",
					APIVersion: "ome.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "training-job",
				},
				Spec: v1beta1.TrainingJobSpec{
					Labels: map[string]string{
						constants.TrainingJobPodLabelKey: tjob.Name,
					},
					Annotations: map[string]string{
						constants.BaseModelName: *tjob.Spec.ModelConfig.InputModel,
					},
					ModelConfig: &v1beta1.ModelConfig{
						InputModel: &inputModel,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := defaulter.Default(context.TODO(), tc.object)
			if err != nil {
				t.Errorf("Default training job got unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.trainingJob, tc.object, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
		})
	}
}
