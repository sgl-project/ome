package secrets

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"errors"
	"fmt"
)

// EnvVar represents a configuration used to resolve the value for some environment variable.
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

// ResolveValue resolves using value_by_realm first and then value_by_region second.
//
// NOTE: if value_by_realm has a valid "default" entry,
// then value_by_region won't be considered.
func (ev *EnvVar) ResolveValue(e env.Interface) (string, error) {
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

// EnsureValid ensures EnvVars are valid.
// It will panic during at `go test ./...` if they're not
func EnsureValid(ev *EnvVar) *EnvVar {
	if err := ev.Validate(); err != nil {
		panic("invalid default EnvVar: " + err.Error())
	}

	return ev
}
