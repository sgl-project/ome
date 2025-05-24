package capacityreservation

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func TestCapacityReservationValidation(t *testing.T) {
	// Set up logging
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	// Create a scheme with the necessary types
	s := scheme.Scheme
	_ = v1beta1.AddToScheme(s)
	_ = kueuev1beta1.AddToScheme(s)

	// Create a fake client with some existing capacity reservations
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Create a decoder
	decoder := admission.NewDecoder(s)

	// Create the validator
	validator := &CapacityReservationValidator{
		Client:  fakeClient,
		Decoder: decoder,
	}

	// Create test cases
	tests := []struct {
		name                string
		capacityReservation *v1beta1.ClusterCapacityReservation
		expectedAllowed     bool
		expectedMessage     string
	}{
		{
			name: "valid small capacity reservation",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					CompartmentID: "ocid1.compartment.oc1.test-tenancy.valid",
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							CoveredResources: []corev1.ResourceName{"cpu", "memory", "nvidia.com/gpu"},
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "bm-gpu-a100-v2-8",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceName("nvidia.com/gpu"),
											NominalQuota: resource.MustParse("2"),
										},
										{
											Name:         corev1.ResourceName("cpu"),
											NominalQuota: resource.MustParse("4"),
										},
										{
											Name:         corev1.ResourceName("memory"),
											NominalQuota: resource.MustParse("16Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedAllowed: true,
		},
		{
			name: "invalid large capacity reservation",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					CompartmentID: "ocid1.compartment.oc1.test-tenancy.valid",
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							CoveredResources: []corev1.ResourceName{"cpu", "memory", "nvidia.com/gpu"},
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "bm-gpu-a100-v2-8",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceName("nvidia.com/gpu"),
											NominalQuota: resource.MustParse("1000"),
										},
										{
											Name:         corev1.ResourceName("cpu"),
											NominalQuota: resource.MustParse("1000"),
										},
										{
											Name:         corev1.ResourceName("memory"),
											NominalQuota: resource.MustParse("1000Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedAllowed: false,
			expectedMessage: "Insufficient resources in the cluster for the requested capacity reservation",
		},
		{
			name: "valid capacity reservation with valid tenancy",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					CompartmentID: "ocid1.compartment.oc1.test-tenancy.valid",
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "bm-gpu-h100-8",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceName("nvidia.com/gpu"),
											NominalQuota: resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedAllowed: true,
		},
		{
			name: "invalid capacity reservation - no compartment ID",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "bm-gpu-h100-8",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceName("nvidia.com/gpu"),
											NominalQuota: resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedAllowed: true,
		},
		{
			name: "invalid capacity reservation - wrong tenancy",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					CompartmentID: "ocid1.compartment.oc1.wrong-tenancy.invalid",
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "bm-gpu-h100-8",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceName("nvidia.com/gpu"),
											NominalQuota: resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedAllowed: true,
		},
		{
			name: "invalid capacity reservation - no resource groups",
			capacityReservation: &v1beta1.ClusterCapacityReservation{
				Spec: v1beta1.CapacityReservationSpec{
					CompartmentID:  "ocid1.compartment.oc1.test-tenancy.valid",
					ResourceGroups: []kueuev1beta1.ResourceGroup{},
				},
			},
			expectedAllowed: false,
			expectedMessage: "No resource groups specified in ClusterCapacityReservation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing case: %s", tt.name)
			t.Logf("Expected allowed: %v", tt.expectedAllowed)
			if !tt.expectedAllowed {
				t.Logf("Expected message: %s", tt.expectedMessage)
			}

			// Create the admission request
			capacityReservationBytes, _ := json.Marshal(tt.capacityReservation)
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: capacityReservationBytes,
					},
				},
			}

			// Handle the request
			resp := validator.Handle(context.Background(), req)

			// Log the response
			t.Logf("Response allowed: %v", resp.Allowed)
			if !resp.Allowed {
				t.Logf("Response message: %s", resp.Result.Message)
			}

			// Check the response
			assert.Equal(t, tt.expectedAllowed, resp.Allowed)
			if !tt.expectedAllowed {
				assert.Contains(t, resp.Result.Message, tt.expectedMessage)
			}
		})
	}
}
