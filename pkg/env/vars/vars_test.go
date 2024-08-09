package vars

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVar_New(t *testing.T) {
	tcs := []struct {
		name        string
		expectedErr bool
	}{
		{name: "valid"},
		{name: "also-valid"},
		{name: "also_valid"},
		{name: "pretty-much-valid"},
		{name: "hello-_there-bruh1234"},
		{name: "AlSo_VeRy_GoOD_VaRiABLE12345"},
		{name: "not valid", expectedErr: true},
		{name: "nope ", expectedErr: true},
		{name: " nope", expectedErr: true},
		{name: "not\rvalid", expectedErr: true},
		{name: "not\tvalid ", expectedErr: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewVar(tc.name, false)
			if err != nil {
				require.True(t, tc.expectedErr, "should have failed")
			} else {
				require.Equal(t, tc.name, r.Name(), "name should be the same")
			}
		})
	}
}
