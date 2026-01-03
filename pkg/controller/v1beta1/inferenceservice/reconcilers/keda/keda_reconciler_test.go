package keda

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

func TestGetScaledObjectTriggers(t *testing.T) {
	serviceName := "my-model"
	namespace := "test"

	testCases := []struct {
		name                    string
		metadata                metav1.ObjectMeta
		inferenceServiceSpec    v1beta1.InferenceServiceSpec
		expectedAuthRef         *kedav1.AuthenticationRef
		expectedAuthModesInMeta string
		expectedServerAddress   string
	}{
		{
			name: "No authenticationRef - should create trigger without auth",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: &v1beta1.KedaConfig{
					EnableKeda:        true,
					PromServerAddress: "http://prometheus:9090",
					ScalingThreshold:  "10",
				},
			},
			expectedAuthRef:         nil,
			expectedAuthModesInMeta: "",
			expectedServerAddress:   "http://prometheus:9090",
		},
		{
			name: "With TriggerAuthentication - should include authenticationRef",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: &v1beta1.KedaConfig{
					EnableKeda:        true,
					PromServerAddress: "https://prometheus.grafana.net/api/prom",
					ScalingThreshold:  "0.07",
					AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
						Name: "grafana-cloud-auth",
						Kind: "TriggerAuthentication",
					},
					AuthModes: "basic",
				},
			},
			expectedAuthRef: &kedav1.AuthenticationRef{
				Name: "grafana-cloud-auth",
				Kind: "TriggerAuthentication",
			},
			expectedAuthModesInMeta: "basic",
			expectedServerAddress:   "https://prometheus.grafana.net/api/prom",
		},
		{
			name: "With ClusterTriggerAuthentication - should include authenticationRef with ClusterTriggerAuthentication kind",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: &v1beta1.KedaConfig{
					EnableKeda:        true,
					PromServerAddress: "https://prometheus.grafana.net/api/prom",
					ScalingThreshold:  "10",
					AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
						Name: "cluster-wide-auth",
						Kind: "ClusterTriggerAuthentication",
					},
					AuthModes: "tls,basic",
				},
			},
			expectedAuthRef: &kedav1.AuthenticationRef{
				Name: "cluster-wide-auth",
				Kind: "ClusterTriggerAuthentication",
			},
			expectedAuthModesInMeta: "tls,basic",
			expectedServerAddress:   "https://prometheus.grafana.net/api/prom",
		},
		{
			name: "With authenticationRef but no kind - should default to TriggerAuthentication",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: &v1beta1.KedaConfig{
					EnableKeda:        true,
					PromServerAddress: "https://prometheus.grafana.net/api/prom",
					ScalingThreshold:  "10",
					AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
						Name: "my-auth",
						// Kind is empty - should default to TriggerAuthentication
					},
					AuthModes: "bearer",
				},
			},
			expectedAuthRef: &kedav1.AuthenticationRef{
				Name: "my-auth",
				Kind: "TriggerAuthentication", // Default
			},
			expectedAuthModesInMeta: "bearer",
			expectedServerAddress:   "https://prometheus.grafana.net/api/prom",
		},
		{
			name: "With authModes but no authenticationRef - authModes should NOT be in metadata",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: &v1beta1.KedaConfig{
					EnableKeda:        true,
					PromServerAddress: "http://prometheus:9090",
					ScalingThreshold:  "10",
					AuthModes:         "basic", // This should be ignored without authenticationRef
				},
			},
			expectedAuthRef:         nil,
			expectedAuthModesInMeta: "", // authModes is NOT added without authenticationRef
			expectedServerAddress:   "http://prometheus:9090",
		},
		{
			name: "With nil kedaConfig - should use defaults",
			metadata: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			inferenceServiceSpec: v1beta1.InferenceServiceSpec{
				KedaConfig: nil,
			},
			expectedAuthRef:         nil,
			expectedAuthModesInMeta: "",
			expectedServerAddress:   "http://prometheus-operated.monitoring.svc.cluster.local:9090",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			triggers := getScaledObjectTriggers(tt.metadata, tt.inferenceServiceSpec)

			if len(triggers) != 1 {
				t.Fatalf("Expected 1 trigger, got %d", len(triggers))
			}

			trigger := triggers[0]

			// Check trigger type
			if trigger.Type != "prometheus" {
				t.Errorf("Expected trigger type 'prometheus', got '%s'", trigger.Type)
			}

			// Check authenticationRef
			if tt.expectedAuthRef == nil {
				if trigger.AuthenticationRef != nil {
					t.Errorf("Expected no authenticationRef, but got %+v", trigger.AuthenticationRef)
				}
			} else {
				if trigger.AuthenticationRef == nil {
					t.Errorf("Expected authenticationRef %+v, but got nil", tt.expectedAuthRef)
				} else {
					if diff := cmp.Diff(tt.expectedAuthRef, trigger.AuthenticationRef); diff != "" {
						t.Errorf("AuthenticationRef mismatch (-want +got): %s", diff)
					}
				}
			}

			// Check authModes in metadata
			authModes, hasAuthModes := trigger.Metadata["authModes"]
			if tt.expectedAuthModesInMeta == "" {
				if hasAuthModes {
					t.Errorf("Expected no authModes in metadata, but got '%s'", authModes)
				}
			} else {
				if !hasAuthModes {
					t.Errorf("Expected authModes '%s' in metadata, but not found", tt.expectedAuthModesInMeta)
				} else if authModes != tt.expectedAuthModesInMeta {
					t.Errorf("Expected authModes '%s', got '%s'", tt.expectedAuthModesInMeta, authModes)
				}
			}

			// Check serverAddress
			if trigger.Metadata["serverAddress"] != tt.expectedServerAddress {
				t.Errorf("Expected serverAddress '%s', got '%s'", tt.expectedServerAddress, trigger.Metadata["serverAddress"])
			}
		})
	}
}

func TestCalculateMinMaxReplicas(t *testing.T) {
	testCases := []struct {
		name               string
		componentExt       *v1beta1.ComponentExtensionSpec
		expectedMinReplica int32
		expectedMaxReplica int32
	}{
		{
			name: "Default min/max replicas",
			componentExt: &v1beta1.ComponentExtensionSpec{
				MinReplicas: nil,
				MaxReplicas: 0,
			},
			expectedMinReplica: 1,
			expectedMaxReplica: 1,
		},
		{
			name: "Custom min/max replicas",
			componentExt: &v1beta1.ComponentExtensionSpec{
				MinReplicas: intPtr(2),
				MaxReplicas: 10,
			},
			expectedMinReplica: 2,
			expectedMaxReplica: 10,
		},
		{
			name: "Max replicas less than min - should use min",
			componentExt: &v1beta1.ComponentExtensionSpec{
				MinReplicas: intPtr(5),
				MaxReplicas: 3,
			},
			expectedMinReplica: 5,
			expectedMaxReplica: 5,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			minReplicas := calculateMinReplicas(tt.componentExt)
			maxReplicas := calculateMaxReplicas(tt.componentExt, minReplicas)

			if minReplicas != tt.expectedMinReplica {
				t.Errorf("Expected minReplicas %d, got %d", tt.expectedMinReplica, minReplicas)
			}
			if maxReplicas != tt.expectedMaxReplica {
				t.Errorf("Expected maxReplicas %d, got %d", tt.expectedMaxReplica, maxReplicas)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
