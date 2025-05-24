package training

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestTrainingjobValidator_ValidateCreate(t *testing.T) {
	validator := &TrainingjobValidator{}

	valid_tjob := v1beta1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ft-training-job",
		},
	}

	cases := map[string]struct {
		object      runtime.Object
		trainingJob *v1beta1.TrainingJob
		wantError   error
	}{
		"test valid training job create": {
			object: valid_tjob.DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := validator.ValidateCreate(context.Background(), tc.object)

			if err != nil {
				t.Errorf("Unexpected error:\n%s", err)
			}
			if diff := cmp.Diff(tc.trainingJob, tc.object); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestTrainingjobValidator_ValidateUpdate(t *testing.T) {
	validator := &TrainingjobValidator{}

	valid_tjob := v1beta1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ft-training-job",
		},
	}

	cases := map[string]struct {
		object      runtime.Object
		trainingJob *v1beta1.TrainingJob
		wantError   error
	}{
		"test valid training job update": {
			object: valid_tjob.DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := validator.ValidateUpdate(context.Background(), tc.object, tc.object)

			if err != nil {
				t.Errorf("Unexpected error:\n%s", err)
			}
			if diff := cmp.Diff(tc.trainingJob, tc.object); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestTrainingjobValidator_ValidateDelete(t *testing.T) {
	validator := &TrainingjobValidator{}

	valid_tjob := v1beta1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ft-training-job",
		},
	}

	cases := map[string]struct {
		object      runtime.Object
		trainingJob *v1beta1.TrainingJob
		wantError   error
	}{
		"test valid training job delete": {
			object: valid_tjob.DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := validator.ValidateDelete(context.Background(), tc.object)

			if err != nil {
				t.Errorf("Unexpected error:\n%s", err)
			}
			if diff := cmp.Diff(tc.trainingJob, tc.object); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}
