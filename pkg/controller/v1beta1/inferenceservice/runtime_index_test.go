package inferenceservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestISVCRuntimeNameIndexExtractor(t *testing.T) {
	ptr := func(s string) *string { return &s }
	tb := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		obj  client.Object
		want []string
	}{
		{
			name: "runtime ref with name -> indexed",
			obj: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "my-runtime"},
				},
			},
			want: []string{"my-runtime"},
		},
		{
			name: "name independent of kind/apiGroup/autoSync",
			obj: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "ns-runtime",
						Kind:     ptr("ServingRuntime"),
						APIGroup: ptr("ome.io"),
						AutoSync: tb(false),
					},
				},
			},
			want: []string{"ns-runtime"},
		},
		{
			name: "nil runtime ref -> not indexed",
			obj: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Runtime: nil},
			},
			want: nil,
		},
		{
			name: "empty runtime name -> not indexed",
			obj: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: ""},
				},
			},
			want: nil,
		},
		{
			name: "non-ISVC object -> not indexed",
			obj:  &v1beta1.ServingRuntime{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isvcRuntimeNameIndexExtractor(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}
