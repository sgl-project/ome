package env

import (
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
)

type Interface interface {
	Realm() (string, bool)
	Region() (string, bool)
	Resolve(string) (string, error)
}

var _ Interface = &Environment{}

// Environment is responsible for supplying environment specific information.
type Environment struct {
	isGov                  bool
	isONSR                 bool
	isOverlayBastion       bool
	isTouchEnabledForRealm bool

	resolved Vars
}

func (e *Environment) Realm() (string, bool) {
	result, ok := e.resolved[vars.Realm]
	return result, ok
}

func (e *Environment) Region() (string, bool) {
	result, ok := e.resolved[vars.Region]
	return result, ok
}

func (e *Environment) Ad() (string, bool) {
	result, ok := e.resolved[vars.Ad]
	return result, ok
}

// Resolve takes a string literal with ${var} tokens inside
// it to turn those tokens into the resolved values.
//
// e.g. "${hostclass}" => "WEBADM-JIT".
func (e *Environment) Resolve(s string) (string, error) {
	varsToResolve, err := e.parseVars(s)
	if err != nil {
		return "", fmt.Errorf("parsing vars: %w", err)
	}

	ss := s
	for placeholder, resolvedValue := range varsToResolve {
		n, err := e.resolveVar(resolvedValue)
		if err != nil {
			return "", err
		}

		ss = strings.ReplaceAll(ss, placeholder, n)
	}

	return ss, nil
}

func (e *Environment) resolveVar(variable vars.Var) (string, error) {
	if resolvedValue, ok := e.resolved[variable]; ok {
		return resolvedValue, nil
	}
	if computedFn, ok := computedVars[variable]; ok {
		return computedFn(e)
	}

	return "", fmt.Errorf("(shouldn't happen) can't resolve placeholder %v: %q", variable.String(), variable)
}

const (
	// see comment in parseVars method about this.
	nonVarPlaceholder = "!OK!"
)

// ParseVars parses the string and return a map of variables that are used in it.
// e.g. ${region} -> Region, ...
func (e *Environment) parseVars(s string) (map[string]vars.Var, error) {
	varsToResolve := make(map[string]vars.Var)
	for v := range e.resolved {
		s = substituteVarPlaceholder(s, v, varsToResolve)
	}
	for v := range computedVars {
		s = substituteVarPlaceholder(s, v, varsToResolve)
	}

	if strings.Contains(s, vars.PlaceholderStart) {
		return nil, fmt.Errorf("value still contains unresolved vars: %s", s)
	}

	return varsToResolve, nil
}

func substituteVarPlaceholder(s string, v vars.Var, varsToResolve map[string]vars.Var) string {
	placeholder := v.String()
	if !strings.Contains(s, placeholder) {
		return s
	}

	varsToResolve[placeholder] = v

	// nonVarPlaceholder needs to follow some rules for this quick & dirty implementation
	// to work deterministically: we gotta be careful to ensure we return an error
	// for strings like '${a${b}}' or '${a${b}c}'.
	//
	// Current algorithm uses strings.ReplaceAll() to enumerate all variables,
	// and if ${b} is processed first, it could lead to expansion of a variable
	// that's different from any of input variables (and not fail when it should):
	//
	//   '${a${b}c}' -> [resolve b as abc] ->
	//   '${aabcc}' -> [resolve aabcc] -> succeeds, but should have failed :(
	//
	// Any placeholder should be fine, as long as:
	// - it doesn't contain ${ or }
	// - it is a valid name for vars.Var
	//
	// A proper implementation (e.g. using strings.IndexOf("${")) wouldn't need
	// this, but I'm too lazy/sleepy to rewrite this right now
	return strings.ReplaceAll(s, placeholder, nonVarPlaceholder)
}

func (e *Environment) IsGov() bool  { return e.isGov }
func (e *Environment) IsONSR() bool { return e.isONSR }

// IsOverlayBastion returns whether the host we're running on is an overlay bastion.
func (e *Environment) IsOverlayBastion() bool { return e.isOverlayBastion }

// IsTouchEnforced returns whether the Yubikey touch should be enforced.
// Always returns false if the instance is not an overlay bastion.
//
// Deprecated: use IsTouchEnforcedOnOverlayBastions instead.
func (e *Environment) IsTouchEnforced() bool { return e.IsTouchEnforcedOnOverlayBastions() }

// IsTouchEnforcedOnOverlayBastions returns whether the Yubikey touch should be enforced.
// Always returns false if the instance is not an overlay bastion.
func (e *Environment) IsTouchEnforcedOnOverlayBastions() bool {
	return e.isTouchEnabledForRealm && e.isOverlayBastion
}

// IsTouchEnforcedForRealm returns whether the Yubikey touch should be enforced.
//
// Used by ldap-updater.
func (e *Environment) IsTouchEnforcedForRealm() bool { return e.isTouchEnabledForRealm }

// TryResolve tries resolving a string with placeholders.
// Returns a resulting string with all variables substituted or an empty string if the resolution has failed.
func TryResolve(e Interface, v string) string {
	result, _ := e.Resolve(v)
	return result
}

// Option is function that configures the resulting Environment.
type Option func(e *Environment)

// Vars is a shorthand for the underlying type.
type Vars = map[vars.Var]string

func WithResolvedVars(resolved Vars) Option {
	return func(e *Environment) {
		e.resolved = resolved
	}
}

func WithIsGov(value bool) Option {
	return func(e *Environment) {
		e.isGov = value
	}
}

func WithIsONSR(value bool) Option {
	return func(e *Environment) {
		e.isONSR = value
	}
}

func WithIsOverlayBastion(value bool) Option {
	return func(e *Environment) {
		e.isOverlayBastion = value
	}
}

func WithIsTouchEnforcedForRealm(value bool) Option {
	return func(e *Environment) {
		e.isTouchEnabledForRealm = value
	}
}

// New creates a new environment using given set of options.
func New(opts ...Option) *Environment {
	e := &Environment{}
	for _, o := range opts {
		o(e)
	}

	return e
}

// FromResolver creates a new environment using given set of resolver config options.
func FromResolver(opts ...ResolverOption) (*Environment, error) {
	conf, err := newResolverConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	resolver, err := newResolver(conf)
	if err != nil {
		return nil, fmt.Errorf("constructing env resolver: %w", err)
	}

	return resolver.Resolve()
}
