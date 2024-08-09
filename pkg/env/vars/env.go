package vars

import (
	"fmt"
	"os"
	"strings"
)

var (
	varToEnv = map[Var]string{
		Ad:                    "AD",
		Region:                "REGION",
		Realm:                 "REALM",
		InstanceCompartmentId: "INSTANCE_COMPARTMENT_ID",
		ResourceCompartmentId: "RESOURCE_COMPARTMENT_ID",
		TenancyId:             "TENANCY_ID",
		Hostclass:             "HOSTCLASS",
		RealmTLD:              "REALM_TOP_LEVEL_DOMAIN",
		InternalRealmTLD:      "INTERNAL_REALM_TOP_LEVEL_DOMAIN",
	}
)

// EnvResolverConfig provides additional configuration for EnvResolver.
type EnvResolverConfig struct {
	// AdditionalVars allow for env resolver to resolve additional
	// environment variables defined in configs.
	AdditionalVars []string `mapstructure:"additional_vars"`
}

// Validate validates the env resolver config.
func (c EnvResolverConfig) Validate() error {
	for _, varName := range c.AdditionalVars {
		if err := validateVarName(varName); err != nil {
			return fmt.Errorf("invalid additional var '%s': %w", varName, err)
		}
	}

	return nil
}

// EnvResolver represents a var resolver that uses env variables.
type EnvResolver struct {
	additionalVars map[Var]string
}

// NewEnvResolver constructs a new EnvResolver.
func NewEnvResolver(config EnvResolverConfig) (*EnvResolver, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid env resolver config: %w", err)
	}

	addlEnvVars, err := constructAddlEnvVars(config)
	if err != nil {
		return nil, fmt.Errorf("constructing additional env vars: %w", err)
	}

	return &EnvResolver{
		additionalVars: addlEnvVars,
	}, nil
}

func constructAddlEnvVars(c EnvResolverConfig) (map[Var]string, error) {
	res := make(map[Var]string, len(c.AdditionalVars))

	for _, varName := range c.AdditionalVars {
		// TODO(achebatu): remove isOcid flag from Var type,
		//   as it was a bad decision based on the assumption that is _no longer_ valid:
		//     all vars with OCID values must be escaped.
		//   Instead, the escaping should be done where it's required
		//   (e.g. by access-updater when checking the entitlements)
		v, err := NewVar(varName, false /* don't escape */)
		if err != nil {
			return nil, err
		}

		res[v] = varName
	}

	return res, nil
}

func (o EnvResolver) Resolve(v Var) (string, error) {
	envVar, ok := varToEnv[v]
	if !ok {
		envVar, ok = o.additionalVars[v]
	}
	if !ok {
		return "", fmt.Errorf("env can't resolve var: %v", v)
	}

	result, err := o.resolve(envVar)
	if err != nil {
		return "", err
	}

	if v.IsOcid() {
		return escapeOCID(result), nil
	}

	if v.IsHostClassName() {
		return strings.ToLower(result), nil
	}

	return result, nil
}

func (o EnvResolver) CanResolve() []Var {
	result := []Var{
		Region,
		Realm,
		Ad,
		InstanceCompartmentId,
		ResourceCompartmentId,
		TenancyId,
		Hostclass,
		RealmTLD,
	}

	for v := range o.additionalVars {
		result = append(result, v)
	}

	return result
}

func (o EnvResolver) resolve(variable string) (string, error) {
	result, ok := os.LookupEnv(variable)
	if !ok {
		return "", fmt.Errorf("env variable '%s' not found", variable)
	}

	if result == "" {
		return "", fmt.Errorf("env variable '%s' is empty", variable)
	}

	return result, nil
}

var _ Resolver = EnvResolver{}
