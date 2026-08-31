package validation

import (
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestRequiredTrafficCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		traffic *v1beta1.TrafficSpec
		want    []string
	}{
		{
			name:    "nil traffic requires nothing",
			traffic: nil,
			want:    nil,
		},
		{
			name:    "empty traffic requires nothing",
			traffic: &v1beta1.TrafficSpec{},
			want:    nil,
		},
		{
			name:    "algorithm only",
			traffic: &v1beta1.TrafficSpec{Algorithm: ptrLB(v1beta1.LoadBalancingTypeRoundRobin)},
			want:    []string{constants.TrafficCapabilityAlgorithm},
		},
		{
			name: "single-header hash",
			traffic: &v1beta1.TrafficSpec{
				Algorithm: ptrLB(v1beta1.LoadBalancingTypeConsistentHash),
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "x-tenant"}},
				},
			},
			want: []string{
				constants.TrafficCapabilityAlgorithm,
				constants.TrafficCapabilityHashHeader,
			},
		},
		{
			name: "multi-header hash adds the multi-header token",
			traffic: &v1beta1.TrafficSpec{
				Algorithm: ptrLB(v1beta1.LoadBalancingTypeConsistentHash),
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "x-tenant"}, {Name: "x-session"}},
				},
			},
			want: []string{
				constants.TrafficCapabilityAlgorithm,
				constants.TrafficCapabilityHashHeader,
				constants.TrafficCapabilityHashMultipleHeaders,
			},
		},
		{
			name: "cookie hash",
			traffic: &v1beta1.TrafficSpec{
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:   v1beta1.HashTypeCookie,
					Cookie: &v1beta1.HashCookie{Name: "session"},
				},
			},
			want: []string{constants.TrafficCapabilityHashCookie},
		},
		{
			name: "source-ip hash",
			traffic: &v1beta1.TrafficSpec{
				ConsistentHash: &v1beta1.ConsistentHashSpec{Type: v1beta1.HashTypeSourceIP},
			},
			want: []string{constants.TrafficCapabilityHashSourceIP},
		},
		{
			name: "endpoint override header",
			traffic: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type:    v1beta1.EndpointOverrideTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "x-endpoint"}},
				},
			},
			want: []string{constants.TrafficCapabilityEndpointOverrideHeader},
		},
		{
			name: "endpoint override metadata",
			traffic: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type: v1beta1.EndpointOverrideTypeMetadata,
				},
			},
			want: []string{constants.TrafficCapabilityEndpointOverrideMetadata},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RequiredTrafficCapabilities(tc.traffic)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RequiredTrafficCapabilities() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateTrafficCapabilities_MetadataOverride(t *testing.T) {
	metadata := &v1beta1.TrafficSpec{
		EndpointOverride: &v1beta1.EndpointOverrideSpec{
			Type: v1beta1.EndpointOverrideTypeMetadata,
		},
	}

	t.Run("rejected when the capability set is empty", func(t *testing.T) {
		err := ValidateTrafficCapabilities(metadata, nil)
		if err == nil {
			t.Fatal("expected rejection, got nil")
		}
		if !strings.Contains(err.Error(), "ReservedEndpointOverrideType") {
			t.Fatalf("error should carry the ReservedEndpointOverrideType rule name: %v", err)
		}
	})

	t.Run("rejected when the active translator does not declare it", func(t *testing.T) {
		supported := []string{
			constants.TrafficCapabilityAlgorithm,
			constants.TrafficCapabilityEndpointOverrideHeader,
		}
		if err := ValidateTrafficCapabilities(metadata, supported); err == nil {
			t.Fatal("expected rejection, got nil")
		}
	})

	t.Run("accepted once a translator declares the capability", func(t *testing.T) {
		supported := []string{constants.TrafficCapabilityEndpointOverrideMetadata}
		if err := ValidateTrafficCapabilities(metadata, supported); err != nil {
			t.Fatalf("expected acceptance, got %v", err)
		}
	})
}

func TestValidateTrafficCapabilities_MultiHeaderHash(t *testing.T) {
	multiHeader := &v1beta1.TrafficSpec{
		Algorithm: ptrLB(v1beta1.LoadBalancingTypeConsistentHash),
		ConsistentHash: &v1beta1.ConsistentHashSpec{
			Type:    v1beta1.HashTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-tenant"}, {Name: "x-session"}},
		},
	}

	t.Run("permissive when the capability set is empty (provider unknown)", func(t *testing.T) {
		if err := ValidateTrafficCapabilities(multiHeader, nil); err != nil {
			t.Fatalf("empty set must not gate multi-header hashing, got %v", err)
		}
	})

	t.Run("rejected when the active translator hashes a single header only", func(t *testing.T) {
		supported := []string{
			constants.TrafficCapabilityAlgorithm,
			constants.TrafficCapabilityHashHeader,
		}
		err := ValidateTrafficCapabilities(multiHeader, supported)
		if err == nil {
			t.Fatal("expected rejection, got nil")
		}
		if !strings.Contains(err.Error(), "UnsupportedMultiHeaderHash") {
			t.Fatalf("error should carry the UnsupportedMultiHeaderHash rule name: %v", err)
		}
	})

	t.Run("accepted when the active translator declares multi-header hashing", func(t *testing.T) {
		supported := []string{
			constants.TrafficCapabilityAlgorithm,
			constants.TrafficCapabilityHashHeader,
			constants.TrafficCapabilityHashMultipleHeaders,
		}
		if err := ValidateTrafficCapabilities(multiHeader, supported); err != nil {
			t.Fatalf("expected acceptance, got %v", err)
		}
	})

	t.Run("single header passes without the multi-header capability", func(t *testing.T) {
		single := &v1beta1.TrafficSpec{
			Algorithm: ptrLB(v1beta1.LoadBalancingTypeConsistentHash),
			ConsistentHash: &v1beta1.ConsistentHashSpec{
				Type:    v1beta1.HashTypeHeader,
				Headers: []v1beta1.HashHeader{{Name: "x-tenant"}},
			},
		}
		supported := []string{constants.TrafficCapabilityHashHeader}
		if err := ValidateTrafficCapabilities(single, supported); err != nil {
			t.Fatalf("expected acceptance, got %v", err)
		}
	})
}

func TestValidateTrafficCapabilities_NilTraffic(t *testing.T) {
	if err := ValidateTrafficCapabilities(nil, nil); err != nil {
		t.Fatalf("nil traffic must pass, got %v", err)
	}
}
