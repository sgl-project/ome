package modelagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSameStringPtr(t *testing.T) {
	left := "value"
	same := "value"
	different := "different"

	tests := []struct {
		name  string
		left  *string
		right *string
		want  bool
	}{
		{name: "both nil", want: true},
		{name: "left nil", right: &same},
		{name: "right nil", left: &left},
		{name: "same value", left: &left, right: &same, want: true},
		{name: "different value", left: &left, right: &different},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameStringPtr(tt.left, tt.right))
		})
	}
}

func TestSameStringMapPtr(t *testing.T) {
	left := map[string]string{"region": "us-phoenix-1", "auth": "InstancePrincipal"}
	same := map[string]string{"auth": "InstancePrincipal", "region": "us-phoenix-1"}
	differentValue := map[string]string{"region": "us-ashburn-1", "auth": "InstancePrincipal"}
	missingKey := map[string]string{"region": "us-phoenix-1"}
	emptyLeft := map[string]string{}
	emptyRight := map[string]string{}

	tests := []struct {
		name  string
		left  *map[string]string
		right *map[string]string
		want  bool
	}{
		{name: "both nil", want: true},
		{name: "left nil", right: &same},
		{name: "right nil", left: &left},
		{name: "same entries", left: &left, right: &same, want: true},
		{name: "different value", left: &left, right: &differentValue},
		{name: "missing key", left: &left, right: &missingKey},
		{name: "both empty maps", left: &emptyLeft, right: &emptyRight, want: true},
		{name: "nil and empty map", right: &emptyRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameStringMapPtr(tt.left, tt.right))
		})
	}
}
