package env

import (
	"errors"
	"fmt"
)

const (
	DefaultKey = "default"
)

// StringSliceByRegion is a set of string slices that can be resolved by environment.
// Keys are regions (or "default") and each string in the slice could contain various ${.} placeholders
// that will be resolved with the values from the environment.
type StringSliceByRegion map[string][]string

func (ebr StringSliceByRegion) Validate() error { return validateSliceNoEmptyValues(ebr) }
func (ebr StringSliceByRegion) Resolve(env Interface) ([]string, error) {
	return resolveSlice(env, ebr, env.Region)
}
func (ebr StringSliceByRegion) ResolveIfExists(env Interface) ([]string, error) {
	return resolveSliceIfExists(env, ebr, env.Region)
}

// StringSliceByRealm is a set of string slices that can be resolved by environment.
// Keys are realms (or "default") and each string in the slice could contain various ${.} placeholders
// that will be resolved with the values from the environment.
type StringSliceByRealm map[string][]string

func (ebr StringSliceByRealm) Validate() error { return validateSliceNoEmptyValues(ebr) }
func (ebr StringSliceByRealm) Resolve(env Interface) ([]string, error) {
	return resolveSlice(env, ebr, env.Realm)
}
func (ebr StringSliceByRealm) ResolveIfExists(env Interface) ([]string, error) {
	return resolveSliceIfExists(env, ebr, env.Realm)
}

// StringSlice is a slice that can be resolved by environment.
type StringSlice []string

func (ss StringSlice) Validate() error                         { return validateSlice(ss) }
func (ss StringSlice) Resolve(env Interface) ([]string, error) { return resolveSliceWithEnv(env, ss) }

func validateSliceNoEmptyValues(ebl map[string][]string) error {
	for k, v := range ebl {
		if len(v) == 0 {
			return fmt.Errorf("empty slice for key '%v'", k)
		}

		err := validateSlice(v)
		if err != nil {
			return fmt.Errorf("invalid slice under key '%v': %w", k, err)
		}
	}

	return nil
}

func validateSlice(slice []string) error {
	for _, v := range slice {
		if v == "" {
			return errors.New("empty string in slice")
		}
	}
	return nil
}

func resolveSlice(env Interface, ebl map[string][]string, getLocation func() (string, bool)) ([]string, error) {
	candidates := candidatesFor(getLocation)
	for _, candidate := range candidates {
		endpoints, ok := ebl[candidate]
		if !ok {
			continue
		}

		resolvedEndpoints, err := resolveSliceWithEnv(env, endpoints)
		if err != nil {
			continue
		}
		return resolvedEndpoints, nil
	}

	return nil, fmt.Errorf("unable to resolve endpoint using these candidates: %q", candidates)
}

// candidatesFor constructs an ordered list of keys to probe the String*By* map types.
// It returns whatever locationFn returned (e.g. env.Realm, env.Region)
// and a "default" case (as a fallback).
func candidatesFor(locationFn func() (string, bool)) []string {
	var candidates []string
	if loc, ok := locationFn(); ok {
		candidates = append(candidates, loc)
	}

	return append(candidates, DefaultKey)
}

func resolveSliceWithEnv(env Interface, endpoints []string) ([]string, error) {
	var result []string
	for _, e := range endpoints {
		s, err := env.Resolve(e)
		if err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	return result, nil
}

func resolveSliceIfExists(env Interface, ebl map[string][]string, location locationFn) ([]string, error) {
	loc, ok := location()
	if !ok {
		return nil, nil
	}

	endpoints, ok := ebl[loc]
	if !ok {
		return nil, nil
	}

	resolvedEndpoints, err := resolveSliceWithEnv(env, endpoints)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve endpoint for location: %q", loc)
	}

	return resolvedEndpoints, nil
}
