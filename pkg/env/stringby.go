package env

import "fmt"

// StringByRegion is a set of strings that can be resolved by environment.
// Keys are regions (or "default") and each string value could contain various ${.} placeholders
// that will be resolved with the values from the environment.
type StringByRegion map[string]string

func (ebr StringByRegion) Validate() error { return validateNoEmptyValues(ebr) }
func (ebr StringByRegion) Resolve(env Interface) (string, error) {
	return resolve(env, ebr, env.Region)
}
func (ebr StringByRegion) ResolveIfExists(env Interface) (string, error) {
	return resolveIfExists(env, ebr, env.Region)
}

// StringByRealm is a set of strings that can be resolved by environment.
// Keys are realms (or "default") and each string value could contain various ${.} placeholders
// that will be resolved with the values from the environment.
type StringByRealm map[string]string

func (ebr StringByRealm) Validate() error                       { return validateNoEmptyValues(ebr) }
func (ebr StringByRealm) Resolve(env Interface) (string, error) { return resolve(env, ebr, env.Realm) }
func (ebr StringByRealm) ResolveIfExists(env Interface) (string, error) {
	return resolveIfExists(env, ebr, env.Realm)
}

func validateNoEmptyValues(ebl map[string]string) error {
	for k, v := range ebl {
		if v == "" {
			return fmt.Errorf("empty value for key '%v'", k)
		}
	}

	return nil
}

func resolve(env Interface, ebl map[string]string, locationFn func() (string, bool)) (string, error) {
	candidates := candidatesFor(locationFn)
	for _, candidate := range candidates {
		endpoint, ok := ebl[candidate]
		if !ok {
			continue
		}
		resolvedEndpoint, err := env.Resolve(endpoint)
		if err != nil {
			continue
		}
		return resolvedEndpoint, nil
	}

	return "", fmt.Errorf("unable to resolve endpoint using these candidates: %q", candidates)
}

type locationFn func() (string, bool)

func resolveIfExists(env Interface, ebl map[string]string, location locationFn) (string, error) {
	loc, ok := location()
	if !ok {
		return "", nil
	}

	endpoint, ok := ebl[loc]
	if !ok {
		return "", nil
	}

	resolvedEndpoint, err := env.Resolve(endpoint)
	if err != nil {
		return "", fmt.Errorf("unable to resolve endpoint for location: %q", loc)
	}
	return resolvedEndpoint, nil
}
