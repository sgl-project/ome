package validation

import (
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestValidateTrafficSpec(t *testing.T) {
	consistentHash := v1beta1.LoadBalancingTypeConsistentHash
	leastRequest := v1beta1.LoadBalancingTypeLeastRequest
	ttlSecs := int64(3600)
	tests := []struct {
		name   string
		in     *v1beta1.TrafficSpec
		wantOK bool
		// Substring of the expected error message. Empty when wantOK.
		wantContains string
	}{
		{
			name:   "nil traffic",
			in:     nil,
			wantOK: true,
		},
		{
			name:   "empty traffic",
			in:     &v1beta1.TrafficSpec{},
			wantOK: true,
		},
		{
			name: "round-robin only",
			in: &v1beta1.TrafficSpec{
				Algorithm: ptrLB(v1beta1.LoadBalancingTypeRoundRobin),
			},
			wantOK: true,
		},
		{
			name: "consistent hash on header — happy path",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
			},
			wantOK: true,
		},
		{
			name: "consistent hash on cookie with TTL — happy path",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:   v1beta1.HashTypeCookie,
					Cookie: &v1beta1.HashCookie{Name: "ome-session", TTLSeconds: &ttlSecs},
				},
			},
			wantOK: true,
		},
		{
			name: "consistent hash on source IP — happy path",
			in: &v1beta1.TrafficSpec{
				Algorithm:      &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{Type: v1beta1.HashTypeSourceIP},
			},
			wantOK: true,
		},
		{
			name: "consistent hash declared without consistentHash block",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
			},
			wantContains: "MissingConsistentHashSpec",
		},
		{
			name: "consistent hash block declared without ConsistentHash algorithm",
			in: &v1beta1.TrafficSpec{
				Algorithm: &leastRequest,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
			},
			wantContains: "UnexpectedConsistentHashSpec",
		},
		{
			name: "consistent hash Header without headers",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type: v1beta1.HashTypeHeader,
				},
			},
			wantContains: "MissingHashKey",
		},
		{
			name: "consistent hash Header with empty header name",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: ""}},
				},
			},
			wantContains: "MissingHashKey",
		},
		{
			name: "consistent hash Header with cookie also set",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
					Cookie:  &v1beta1.HashCookie{Name: "ome"},
				},
			},
			wantContains: "MultipleHashKeys",
		},
		{
			name: "consistent hash Cookie without cookie block",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type: v1beta1.HashTypeCookie,
				},
			},
			wantContains: "MissingHashKey",
		},
		{
			name: "consistent hash Cookie with headers also set",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeCookie,
					Cookie:  &v1beta1.HashCookie{Name: "ome"},
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
			},
			wantContains: "MultipleHashKeys",
		},
		{
			name: "consistent hash Cookie with empty name",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:   v1beta1.HashTypeCookie,
					Cookie: &v1beta1.HashCookie{Name: ""},
				},
			},
			wantContains: "MissingHashKey",
		},
		{
			name: "consistent hash SourceIP with stray headers",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeSourceIP,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
			},
			wantContains: "MultipleHashKeys",
		},
		{
			name: "endpoint override Header without headers",
			in: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type: v1beta1.EndpointOverrideTypeHeader,
				},
			},
			wantContains: "MissingEndpointOverrideKey",
		},
		{
			name: "endpoint override Header with empty header name",
			in: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type:    v1beta1.EndpointOverrideTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: ""}},
				},
			},
			wantContains: "MissingEndpointOverrideKey",
		},
		{
			name: "endpoint override Metadata — no required sub-fields",
			in: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type: v1beta1.EndpointOverrideTypeMetadata,
				},
			},
			wantOK: true,
		},
		{
			name: "endpoint override Header — happy path",
			in: &v1beta1.TrafficSpec{
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type:    v1beta1.EndpointOverrideTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Endpoint-HostPort"}},
				},
			},
			wantOK: true,
		},
		{
			name: "consistent hash + endpoint override compose",
			in: &v1beta1.TrafficSpec{
				Algorithm: &consistentHash,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
				EndpointOverride: &v1beta1.EndpointOverrideSpec{
					Type:    v1beta1.EndpointOverrideTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Endpoint-HostPort"}},
				},
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTrafficSpec(tc.in)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected ok, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantContains)
			}
			if !strings.Contains(err.Error(), tc.wantContains) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantContains, err)
			}
		})
	}
}

func ptrLB(t v1beta1.LoadBalancingType) *v1beta1.LoadBalancingType {
	return &t
}
