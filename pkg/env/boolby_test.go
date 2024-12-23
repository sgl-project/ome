package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoolByRegion(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		require.Error(t, BoolByRegion{}.Validate())
		require.Error(t, BoolByRegion{"": false}.Validate())
		require.Error(t, BoolByRegion{"us-phoenix-1": false}.Validate())
		require.NoError(t, BoolByRegion{"default": false}.Validate())
		require.NoError(t, BoolByRegion{"default": false, "us-phoenix-1": false}.Validate())
	})
	t.Run("happy case", func(t *testing.T) {
		bbr := BoolByRegion{
			"us-phoenix-1": true,
			"default":      false,
		}

		value, err := bbr.Resolve(newFakeEnv("oc1", "us-phoenix-1"))
		require.NoError(t, err)
		require.True(t, value)

		value, err = bbr.Resolve(newFakeEnv("oc2", "us-langley-1"))
		require.NoError(t, err)
		require.False(t, value)
	})
}

func TestBoolByRealm(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		require.Error(t, BoolByRealm{}.Validate())
		require.Error(t, BoolByRealm{"": false}.Validate())
		require.Error(t, BoolByRealm{"oc1": false}.Validate())
		require.NoError(t, BoolByRealm{"default": false}.Validate())
		require.NoError(t, BoolByRealm{"default": false, "oc1": true}.Validate())
	})
	t.Run("happy case", func(t *testing.T) {
		bbr := BoolByRealm{
			"oc1":     true,
			"default": false,
		}

		value, err := bbr.Resolve(newFakeEnv("oc1", "us-phoenix-1"))
		require.NoError(t, err)
		require.True(t, value)

		value, err = bbr.Resolve(newFakeEnv("oc2", "us-langley-1"))
		require.NoError(t, err)
		require.False(t, value)
	})
}
