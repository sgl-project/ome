package autoscaler

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

func TestDeleteExistingScaledObject(t *testing.T) {
	serviceName := "my-model"
	namespace := "test"

	testCases := []struct {
		name          string
		componentMeta metav1.ObjectMeta
		setupClient   func() client.Client
		expectError   bool
		verifyDeleted bool
	}{
		{
			name: "ScaledObject does not exist - should return nil",
			componentMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			setupClient: func() client.Client {
				scheme := runtime.NewScheme()
				_ = kedav1.AddToScheme(scheme)
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			expectError:   false,
			verifyDeleted: false,
		},
		{
			name: "ScaledObject exists - should delete it",
			componentMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			setupClient: func() client.Client {
				scheme := runtime.NewScheme()
				_ = kedav1.AddToScheme(scheme)
				existingScaledObject := &kedav1.ScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("scaledobject-%s", serviceName),
						Namespace: namespace,
					},
				}
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(existingScaledObject).
					Build()
			},
			expectError:   false,
			verifyDeleted: true,
		},
		{
			name: "KEDA types not registered in scheme - should return nil gracefully",
			componentMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			setupClient: func() client.Client {
				// Create a scheme WITHOUT KEDA types registered
				scheme := runtime.NewScheme()
				// Don't add kedav1.AddToScheme(scheme) - this simulates KEDA not being available
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			expectError:   false,
			verifyDeleted: false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient()
			err := deleteExistingScaledObject(fakeClient, tt.componentMeta)

			if tt.expectError && err == nil {
				t.Errorf("Test %q expected error but got nil", t.Name())
			}
			if !tt.expectError && err != nil {
				t.Errorf("Test %q unexpected error: %v", t.Name(), err)
			}

			// Verify the ScaledObject was actually deleted
			if tt.verifyDeleted {
				scaledObject := &kedav1.ScaledObject{}
				getErr := fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: namespace,
					Name:      fmt.Sprintf("scaledobject-%s", serviceName),
				}, scaledObject)
				if !apierr.IsNotFound(getErr) {
					t.Errorf("Test %q expected ScaledObject to be deleted, but it still exists", t.Name())
				}
			}
		})
	}
}
