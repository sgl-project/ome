package omenative

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestEndpointSliceToISVC(t *testing.T) {
	cases := []struct {
		name        string
		serviceName string
		ns          string
		wantISVC    string
		wantOK      bool
	}{
		{"empty service-name", "", "prod", "", false},
		{"non-headless", "foo-bar", "prod", "", false},
		{"foreign suffix", "foo-bar-headless", "prod", "", false},
		{"engine headless", "llama-70b-engine-headless", "prod", "llama-70b", true},
		{"decoder headless", "llama-70b-decoder-headless", "prod", "llama-70b", true},
		{"router headless", "llama-70b-router-headless", "prod", "llama-70b", true},
		{"missing isvc name", "engine-headless", "prod", "", false},
		{"isvc name with dashes", "my-fancy-llama-engine-headless", "prod", "my-fancy-llama", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slice := &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.ns,
					Labels:    map[string]string{discoveryv1.LabelServiceName: tc.serviceName},
				},
			}
			got := EndpointSliceToISVC(context.Background(), slice)
			if !tc.wantOK {
				if len(got) != 0 {
					t.Errorf("expected no requests, got %v", got)
				}
				return
			}
			want := []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: tc.ns, Name: tc.wantISVC},
			}}
			if len(got) != 1 || got[0] != want[0] {
				t.Errorf("request mismatch: got=%v want=%v", got, want)
			}
		})
	}
}

func TestEndpointSliceToISVC_NonSliceObject(t *testing.T) {
	got := EndpointSliceToISVC(context.Background(), &corev1.Pod{})
	if len(got) != 0 {
		t.Errorf("expected no requests for non-EndpointSlice object, got %v", got)
	}
}
