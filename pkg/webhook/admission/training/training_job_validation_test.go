package training

import (
	"context"
	"strings"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/google/go-cmp/cmp"
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

	validTJob := v1beta1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ft-training-job",
		},
	}

	cases := map[string]struct {
		oldObject   runtime.Object
		newObject   runtime.Object
		trainingJob *v1beta1.TrainingJob
		wantErr     string
	}{
		"test valid training job update": {
			oldObject: validTJob.DeepCopyObject(),
			newObject: validTJob.DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
			},
		},
		"test valid training job update with same suspend value": {
			oldObject: makeTrainingJobWithSuspend("ft-training-job", true).DeepCopyObject(),
			newObject: makeTrainingJobWithSuspend("ft-training-job", true).DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
				Spec: v1beta1.TrainingJobSpec{
					Suspend: boolPtr(true),
				},
			},
		},
		"test invalid training job update from suspended to resumed": {
			oldObject: makeTrainingJobWithSuspend("ft-training-job", true).DeepCopyObject(),
			newObject: makeTrainingJobWithSuspend("ft-training-job", false).DeepCopyObject(),
			wantErr:   "spec.suspend is immutable",
		},
		"test invalid training job update from running to suspended": {
			oldObject: makeTrainingJobWithSuspend("ft-training-job", false).DeepCopyObject(),
			newObject: makeTrainingJobWithSuspend("ft-training-job", true).DeepCopyObject(),
			wantErr:   "spec.suspend is immutable",
		},
		"test valid training job update from nil suspend to false": {
			oldObject: validTJob.DeepCopyObject(),
			newObject: makeTrainingJobWithSuspend("ft-training-job", false).DeepCopyObject(),
			trainingJob: &v1beta1.TrainingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ft-training-job",
				},
				Spec: v1beta1.TrainingJobSpec{
					Suspend: boolPtr(false),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := validator.ValidateUpdate(context.Background(), tc.oldObject, tc.newObject)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error:\n%s", err)
			}
			if diff := cmp.Diff(tc.trainingJob, tc.newObject); len(diff) != 0 {
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

func makeTrainingJobWithSuspend(name string, suspend bool) *v1beta1.TrainingJob {
	return &v1beta1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.TrainingJobSpec{
			Suspend: boolPtr(suspend),
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}
