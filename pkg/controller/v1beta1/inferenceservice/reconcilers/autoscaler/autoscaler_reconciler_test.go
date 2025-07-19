package autoscaler

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/constants"
)

func TestGetAutoscalerClass(t *testing.T) {
	serviceName := "my-model"
	namespace := "test"
	testCases := []struct {
		name                   string
		isvcMetaData           *metav1.ObjectMeta
		expectedAutoScalerType constants.AutoscalerClassType
	}{
		{
			name: "Return default AutoScaler,if the autoscalerClass annotation is not set",
			isvcMetaData: &metav1.ObjectMeta{
				Name:        serviceName,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},

			expectedAutoScalerType: constants.AutoscalerClassHPA,
		},
		{
			name: "Return default AutoScaler,if the autoscalerClass annotation set hpa",
			isvcMetaData: &metav1.ObjectMeta{
				Name:        serviceName,
				Namespace:   namespace,
				Annotations: map[string]string{"ome.io/autoscalerClass": "hpa"},
			},

			expectedAutoScalerType: constants.AutoscalerClassHPA,
		},
		{
			name: "Return external AutoScaler,if the autoscalerClass annotation set external",
			isvcMetaData: &metav1.ObjectMeta{
				Name:        serviceName,
				Namespace:   namespace,
				Annotations: map[string]string{"ome.io/autoscalerClass": "external"},
			},
			expectedAutoScalerType: constants.AutoscalerClassExternal,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := getAutoscalerClass(*tt.isvcMetaData)
			if diff := cmp.Diff(tt.expectedAutoScalerType, result); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", t.Name(), diff)
			}
		})
	}
}
