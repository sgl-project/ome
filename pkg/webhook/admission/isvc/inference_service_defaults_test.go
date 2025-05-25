package isvc

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDefaultInferenceService tests the DefaultInferenceService function directly
// This avoids the need to mock the Kubernetes client
func TestDefaultInferenceService(t *testing.T) {
	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		deployConfig    *controllerconfig.DeployConfig
		wantAnnotations map[string]string
	}{
		{
			name: "no deployment mode annotation, deployConfig with RawDeployment",
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
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.RawDeployment),
			},
			wantAnnotations: map[string]string{
				constants.DeploymentMode: string(constants.RawDeployment),
			},
		},
		{
			name: "existing deployment mode annotation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.DeploymentMode: "serverless",
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
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.RawDeployment),
			},
			wantAnnotations: map[string]string{
				constants.DeploymentMode: "serverless",
			},
		},
		{
			name: "nil deployConfig",
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
			deployConfig:    nil,
			wantAnnotations: nil,
		},
		{
			name: "deployConfig with non-RawDeployment default",
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
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: "serverless",
			},
			wantAnnotations: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DefaultInferenceService(tt.isvc, tt.deployConfig)

			if tt.wantAnnotations == nil {
				assert.Nil(t, tt.isvc.ObjectMeta.Annotations)
			} else {
				assert.Equal(t, tt.wantAnnotations, tt.isvc.ObjectMeta.Annotations)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
