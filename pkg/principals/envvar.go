package principals

import (
	"errors"
	"fmt"
	"os"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
)

// EnvVar represents a configuration used to resolve the value for some
// environment variable.
type EnvVar struct {
	ValueByRegion env.StringByRegion `mapstructure:"value_by_region"`
	ValueByRealm  env.StringByRealm  `mapstructure:"value_by_realm"`
}

// Validate validates ev.
func (ev *EnvVar) Validate() error {
	if ev == nil {
		return nil
	}

	if len(ev.ValueByRegion) == 0 && len(ev.ValueByRealm) == 0 {
		return errors.New("either value_by_region or value_by_realm must be set")
	}

	if err := ev.ValueByRealm.Validate(); err != nil {
		return fmt.Errorf("invalid value_by_realm: %w", err)
	}
	if err := ev.ValueByRegion.Validate(); err != nil {
		return fmt.Errorf("invalid value_by_region: %w", err)
	}

	return nil
}

// osSetenv is used in tests.
var osSetenv = os.Setenv

// SetenvOrDefault calls os.Setenv on resolved key=value.
//
// If ev is nil, the defaultEV is used instead.
func (ev *EnvVar) SetenvOrDefault(envKey string, opts Opts, defaultEV *EnvVar) error {
	actualEV := ev.fallbackTo(defaultEV)
	if actualEV == nil {
		return nil
	}

	if err := actualEV.Validate(); err != nil {
		return fmt.Errorf("invalid EnvVar config for '%s': %w", envKey, err)
	}

	value, err := actualEV.resolveValue(opts.Env)
	if err != nil {
		return fmt.Errorf("resolving env var value for '%s': %w", envKey, err)
	}

	if value == "" {
		return nil
	}

	if err := osSetenv(envKey, value); err != nil {
		return fmt.Errorf("calling os.Setenv('%v', '%v'): %w", envKey, value, err)
	}

	opts.Log.
		WithField("env var key", envKey).
		WithField("env var value", value).
		Debug("set env var")
	return nil
}

// resolveValue resolves using value_by_realm first and then value_by_region second.
//
// NOTE: if value_by_realm has a valid "default" entry,
// then value_by_region won't be considered.
func (ev *EnvVar) resolveValue(e env.Interface) (string, error) {
	value, err := ev.ValueByRealm.Resolve(e)
	if err == nil {
		return value, nil
	}

	value, err = ev.ValueByRegion.Resolve(e)
	if err == nil {
		return value, nil
	}

	return "", err
}

func (ev *EnvVar) fallbackTo(defaultEV *EnvVar) *EnvVar {
	if ev != nil {
		return ev
	}

	return defaultEV
}
