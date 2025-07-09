package modelver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		name        string
		v1          string
		v2          string
		equal       bool
		greaterThan bool
	}{
		{
			name:        "equal versions",
			v1:          "1.8.0",
			v2:          "1.8.0",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "greater major version",
			v1:          "2.0.0",
			v2:          "1.9.0",
			equal:       false,
			greaterThan: true,
		},
		{
			name:        "greater minor version",
			v1:          "1.9.0",
			v2:          "1.8.0",
			equal:       false,
			greaterThan: true,
		},
		{
			name:        "greater patch version",
			v1:          "1.8.5",
			v2:          "1.8.0",
			equal:       false,
			greaterThan: true,
		},
		{
			name:        "less major version",
			v1:          "1.0.0",
			v2:          "2.0.0",
			equal:       false,
			greaterThan: false,
		},
		{
			name:        "pre-release versions equal",
			v1:          "1.8.0-alpha",
			v2:          "1.8.0-alpha",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "different pre-release versions",
			v1:          "1.8.0-beta",
			v2:          "1.8.0-alpha",
			equal:       false,
			greaterThan: true, // Implementation might differ on pre-release ordering
		},
		{
			name:        "build metadata equal",
			v1:          "1.8.0+20240707",
			v2:          "1.8.0+20240707",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "different build metadata",
			v1:          "1.8.0+20240708",
			v2:          "1.8.0+20240707",
			equal:       false,
			greaterThan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := Parse(tt.v1)
			assert.NoError(t, err)
			v2, err := Parse(tt.v2)
			assert.NoError(t, err)

			assert.Equal(t, tt.equal, Equal(v1, v2), "Equal comparison failed for %s", tt.name)
			assert.Equal(t, tt.greaterThan, GreaterThan(v1, v2), "GreaterThan comparison failed for %s", tt.name)
		})
	}
}
