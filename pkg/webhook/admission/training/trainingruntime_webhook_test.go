package training

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	v1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

func TestTrainingRuntimeValidator_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name            string
		trainingRuntime *v1beta1.TrainingRuntime
		wantAllowed     bool
		wantReason      string
	}{
		{
			name: "valid training runtime with replicas=1",
			trainingRuntime: &v1beta1.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Spec: v1beta1.TrainingRuntimeSpec{
					Template: v1beta1.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			wantAllowed: true,
			wantReason:  "",
		},
		{
			name: "invalid training runtime with replicas>1",
			trainingRuntime: &v1beta1.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Spec: v1beta1.TrainingRuntimeSpec{
					Template: v1beta1.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			wantAllowed: false,
			wantReason:  "replicas for job 0 must be 1, got 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &TrainingRuntimeValidator{
				Decoder: admission.NewDecoder(scheme),
			}

			objBytes, _ := json.Marshal(tt.trainingRuntime)
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: objBytes,
					},
				},
			}

			got := validator.Handle(context.Background(), req)
			assert.Equal(t, tt.wantAllowed, got.Allowed)
			if !tt.wantAllowed {
				assert.Contains(t, got.Result.Message, tt.wantReason)
			}
		})
	}
}

func TestClusterTrainingRuntimeValidator_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name                   string
		clusterTrainingRuntime *v1beta1.ClusterTrainingRuntime
		wantAllowed            bool
		wantReason             string
	}{
		{
			name: "valid cluster training runtime with replicas=1",
			clusterTrainingRuntime: &v1beta1.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-runtime",
				},
				Spec: v1beta1.TrainingRuntimeSpec{
					Template: v1beta1.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			wantAllowed: true,
			wantReason:  "",
		},
		{
			name: "invalid cluster training runtime with replicas>1",
			clusterTrainingRuntime: &v1beta1.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-runtime",
				},
				Spec: v1beta1.TrainingRuntimeSpec{
					Template: v1beta1.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			wantAllowed: false,
			wantReason:  "replicas for job 0 must be 1, got 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &ClusterTrainingRuntimeValidator{
				Decoder: admission.NewDecoder(scheme),
			}

			objBytes, _ := json.Marshal(tt.clusterTrainingRuntime)
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: objBytes,
					},
				},
			}

			got := validator.Handle(context.Background(), req)
			assert.Equal(t, tt.wantAllowed, got.Allowed)
			if !tt.wantAllowed {
				assert.Contains(t, got.Result.Message, tt.wantReason)
			}
		})
	}
}
