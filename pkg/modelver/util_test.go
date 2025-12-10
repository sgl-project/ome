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
		{
			name:        "v0.8.0 equal to v0.8.0",
			v1:          "v0.8.0",
			v2:          "v0.8.0",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "v1 equal to v1",
			v1:          "v1",
			v2:          "v1",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "1 equal to 1",
			v1:          "1",
			v2:          "1",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "two-part version 1.12 equal to 1.12",
			v1:          "1.12",
			v2:          "1.12",
			equal:       true,
			greaterThan: false,
		},
		{
			name:        "less major version, with precision 1",
			v1:          "1",
			v2:          "2",
			equal:       false,
			greaterThan: false,
		},
		{
			name:        "less major version, with major prefix and precision 3",
			v1:          "v1.9.0",
			v2:          "v2.9.0",
			equal:       false,
			greaterThan: false,
		},
		{
			name:        "greater minor version, with major prefix and precision 3",
			v1:          "v0.9.0",
			v2:          "v0.8.0",
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
