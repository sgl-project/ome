package principals

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
)

func TestEnvVar(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		var (
			invalidMap = map[string]string{"default": ""}
			validMap   = map[string]string{"default": "value"}
		)

		t.Run("should succeed if nil", func(t *testing.T) {
			var ev *EnvVar
			require.NoError(t, ev.Validate())
		})
		t.Run("should fail if value_by_region and value_by_realm are nil", func(t *testing.T) {
			ev := &EnvVar{}
			require.Error(t, ev.Validate())
		})
		t.Run("should fail on invalid value_by_realm", func(t *testing.T) {
			ev := &EnvVar{ValueByRealm: invalidMap}
			require.Error(t, ev.Validate())
		})
		t.Run("should fail on invalid value_by_region", func(t *testing.T) {
			ev := &EnvVar{ValueByRegion: invalidMap}
			require.Error(t, ev.Validate())
		})
		t.Run("should succeed on valid value_by_realm", func(t *testing.T) {
			ev := &EnvVar{ValueByRealm: validMap}
			require.NoError(t, ev.Validate())
		})
		t.Run("should succeed on valid value_by_region", func(t *testing.T) {
			ev := &EnvVar{ValueByRegion: validMap}
			require.NoError(t, ev.Validate())
		})
	})
	t.Run("SetenvOrDefault", func(t *testing.T) {
		opts := Opts{
			Log: logging.NewTestLogger(),
			Env: env.New(env.WithResolvedVars(env.Vars{
				vars.Region: "testRegion",
				vars.Realm:  "", // empty on purpose
			})),
		}

		defaultEV := &EnvVar{ValueByRealm: map[string]string{"default": "${region}"}}

		envSet := map[string]string{}
		osSetenv = func(key, value string) error {
			envSet[key] = value
			return nil
		}
		defer func() { osSetenv = os.Setenv }()

		t.Run("nil ev and nil defaultEV should do nothing", func(t *testing.T) {
			envSet = map[string]string{}

			var ev *EnvVar
			require.NoError(t, ev.SetenvOrDefault("TEST", opts, nil))
			require.Empty(t, envSet)
		})
		t.Run("nil ev and non-nil defaultEV sets using defaults", func(t *testing.T) {
			envSet = map[string]string{}

			var ev *EnvVar
			require.NoError(t, ev.SetenvOrDefault("TEST", opts, defaultEV))
			require.Equal(t, map[string]string{"TEST": "testRegion"}, envSet)
		})
		t.Run("empty resolved value doesn't set anything", func(t *testing.T) {
			envSet = map[string]string{}

			ev := &EnvVar{ValueByRealm: map[string]string{"default": "${realm}"}}
			require.NoError(t, ev.SetenvOrDefault("TEST", opts, defaultEV))
			require.Empty(t, envSet)
		})
	})
}
