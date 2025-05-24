package isvc

import (
	"context"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInferenceServiceValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid inference service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name format",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Test-ISVC", // Invalid name format
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid autoscaler class",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: "invalid-class",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid target utilization percentage",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "150", // Invalid percentage
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateCreate(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

func TestInferenceServiceValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldIsvc *v1beta1.InferenceService
		newIsvc *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid update",
			oldIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			newIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("updated-model"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name format in update",
			oldIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			newIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Test-ISVC", // Invalid name format
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateUpdate(context.Background(), tt.oldIsvc, tt.newIsvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

func TestInferenceServiceValidator_ValidateDelete(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid inference service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateDelete(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

func TestValidateInferenceServiceName(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid name",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-name",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name with uppercase",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Invalid-Name",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid name with special characters",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid@name",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInferenceServiceName(tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInferenceServiceAutoscaler(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "no autoscaler class",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
				},
			},
			wantErr: false,
		},
		{
			name: "valid HPA autoscaler class",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassHPA),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid external autoscaler class",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassExternal),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid autoscaler class",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.AutoscalerClass: "invalid-class",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "HPA autoscaler with invalid metrics",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.AutoscalerClass:   string(constants.AutoscalerClassHPA),
						constants.AutoscalerMetrics: "invalid-metric",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInferenceServiceAutoscaler(tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAutoscalerTargetUtilizationPercentage(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "no target utilization percentage",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
				},
			},
			wantErr: false,
		},
		{
			name: "valid target utilization percentage",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "50",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid target utilization percentage (too low)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid target utilization percentage (too high)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "150",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid target utilization percentage (not a number)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "not-a-number",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAutoscalerTargetUtilizationPercentage(tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
