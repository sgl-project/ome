package env

import (
	"errors"
	"fmt"
)

// BoolByRealm is a set of booleans that can be resolved by environment.
// Keys are realms (or "default").
type BoolByRealm map[string]bool

func (bbr BoolByRealm) Validate() error                     { return validateStringBoolMap(bbr) }
func (bbr BoolByRealm) Resolve(env Interface) (bool, error) { return resolveBool(bbr, env.Realm) }

// BoolByRegion is a set of booleans that can be resolved by environment.
// Keys are regions (or "default").
type BoolByRegion map[string]bool

func (bbr BoolByRegion) Validate() error                     { return validateStringBoolMap(bbr) }
func (bbr BoolByRegion) Resolve(env Interface) (bool, error) { return resolveBool(bbr, env.Region) }

func validateStringBoolMap(bbr map[string]bool) error {
	if len(bbr) == 0 {
		return errors.New("no keys")
	}

	for k := range bbr {
		if k == "" {
			return errors.New("empty key")
		}
	}

	if _, ok := bbr[DefaultKey]; !ok {
		return fmt.Errorf("no '%s' key defined", DefaultKey)
	}

	return nil
}

func resolveBool(bbr map[string]bool, keyFn func() (string, bool)) (bool, error) {
	candidates := candidatesFor(keyFn)
	for _, candidate := range candidates {
		value, ok := bbr[candidate]
		if !ok {
			continue
		}

		return value, nil
	}

	return false, fmt.Errorf("unable to resolve endpoint using these candidates: %q", candidates)
}
