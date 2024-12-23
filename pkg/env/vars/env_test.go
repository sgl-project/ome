package vars

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	envRegion = "REGION"
)

func TestEnv_Resolve(t *testing.T) {
	resolver := &EnvResolver{}

	t.Run("empty var", func(t *testing.T) {
		defer setEnv(Region, "")()

		region, err := resolver.Resolve(Region)
		assert.Equal(t, "", region)
		assert.Error(t, err)
	})

	t.Run("unset var", func(t *testing.T) {
		oldValue, hasOldValue := os.LookupEnv(envRegion)
		_ = os.Unsetenv(envRegion)
		defer func() {
			if hasOldValue {
				_ = os.Setenv(envRegion, oldValue)
			}
		}()

		region, err := resolver.Resolve(Region)
		assert.Equal(t, "", region)
		assert.Error(t, err)
	})

	t.Run("unknown env var", func(t *testing.T) {
		unknownVar := MustNewVar("REGION-X", false)
		defer setEnvRaw("REGION-X", "foobarRegion")()

		region, err := resolver.Resolve(unknownVar)
		assert.Equal(t, "", region)
		assert.Error(t, err)
	})

	t.Run("resolve everything that canResolve returns", func(t *testing.T) {
		canResolve := resolver.CanResolve()
		for _, v := range canResolve {
			t.Run(v.String()+"/success", func(t *testing.T) {
				defer setEnv(v, "foobar"+v.name)()

				result, err := resolver.Resolve(v)
				require.NoError(t, err)
				require.Equal(t, "foobar"+v.name, result)
			})

			t.Run(v.String()+"/failure", func(t *testing.T) {
				defer setEnv(v, "")()

				_, err := resolver.Resolve(v)
				require.Error(t, err)
			})
		}
	})

	t.Run("config validation", func(t *testing.T) {
		t.Run("happy case", func(t *testing.T) {
			c := EnvResolverConfig{
				AdditionalVars: nil, /* empty vars is a valid case */
			}

			require.NoError(t, c.Validate())
		})

		t.Run("happy case", func(t *testing.T) {
			c := EnvResolverConfig{AdditionalVars: []string{"HELLO"}}
			require.NoError(t, c.Validate())
		})
		t.Run("not matching regex", func(t *testing.T) {
			c := EnvResolverConfig{AdditionalVars: []string{"123"}}
			require.Error(t, c.Validate())
		})
	})

	t.Run("additional vars", func(t *testing.T) {
		resolver, err := NewEnvResolver(EnvResolverConfig{
			AdditionalVars: []string{"HELLO"},
		})
		require.NoError(t, err)

		helloVar := MustNewVar("HELLO", false)
		t.Run("canResolve", func(t *testing.T) {
			potentialVars := resolver.CanResolve()
			require.Contains(t, potentialVars, helloVar)
		})

		t.Run("resolve happy case", func(t *testing.T) {
			defer setEnvRaw("HELLO", "THERE")()

			result, err := resolver.Resolve(helloVar)
			require.NoError(t, err)
			require.Equal(t, "THERE", result)
		})

		t.Run("resolve empty value", func(t *testing.T) {
			defer setEnvRaw("HELLO", "")()

			result, err := resolver.Resolve(helloVar)
			require.Error(t, err)
			require.Empty(t, result)
		})
	})
}

// returns a cleanup function.
func setEnv(v Var, value string) func() {
	variable := varToEnv[v]
	return setEnvRaw(variable, value)
}

func setEnvRaw(variable string, value string) func() {
	oldValue, hasOldValue := os.LookupEnv(variable)

	if err := os.Setenv(variable, value); err != nil {
		panic(err)
	}

	return func() {
		if hasOldValue {
			_ = os.Unsetenv(variable)
		} else {
			_ = os.Setenv(variable, oldValue)
		}
	}
}
