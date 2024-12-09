// Package env provides an ability to get realm/region/ad information
// as well as other location variables as realmTLD/internalRealmTLD.
//
// Source of realm/region data:
// https://bitbucket.oci.oraclecorp.com/projects/COMMONS/repos/core-regions/browse/src/main/resources/region-data.json
package env

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/viper"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/imds"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

const ViperConfigKey = "env"

type ResolverConfig struct {
	// CanonicalRegionNames is a map from (short region name-> full region name).
	CanonicalRegionNames map[string]string `mapstructure:"canonical_region_names"`

	// RealmToRegions is a map (realm -> []region names).
	RealmConfigs map[string]*RealmConfig `mapstructure:"realm_configs"`

	// OverlayBastionHostclassRealmPrefixes specifies the set of prefixes that
	// are used in tandem with realm value to check if the instance is an overlay bastion.
	// Must be all lower-case.
	//
	// E.g. if the prefix is "overlay-bastion-", then the instance would be considered as an overlay bastion
	// if /etc/hostclass == "overlay-bastion-${realm}"
	OverlayBastionHostclassRealmPrefixes []string `mapstructure:"overlay_bastion_hostclass_prefixes"`

	// OverlayBastionHostclasses specifies the set of hostclasses
	// that are used to check if the instance is an overlay bastion.
	// Must be all lower-case.
	//
	// Note that this is a full string match, instead of prefix-based match
	// as in OverlayBastionHostclassRealmPrefixes.
	OverlayBastionHostclasses []string `mapstructure:"overlay_bastion_hostclasses"`

	// Set of realms to enforce touch on overlay bastions
	TouchEnforcedInRealms []string `mapstructure:"touch_enforced_in_realms"`

	// ResolveVarsWith specifies an ordered list of resolver kinds to use when resolving variables.
	// When resolving any variable, first one returning a value without an error wins.
	ResolveVarsWith []vars.ResolverKind `mapstructure:"resolve_vars_with"`

	// Local represents configuration for the local resolver.
	Local vars.LocalResolverConfig `mapstructure:"local"`

	// Env represents configuration for the env resolver.
	Env vars.EnvResolverConfig `mapstructure:"env"`

	// IMDS represents configuration for the IMDS resolver.
	IMDS imds.Config `mapstructure:"imds"`

	// Fallback represents configuration for the fallback resolver.
	Fallback vars.FallbackResolverConfig `mapstructure:"fallback"`

	// Set by WithResolverLogger
	logger logging.Interface

	// Set by WithResolverFs
	fs afero.Fs
}

func (c *ResolverConfig) ensureDefaults() {
	if c == nil {
		// Would be validated later
		return
	}

	if c.logger == nil {
		c.logger = logging.Discard()
	}
	if c.fs == nil {
		c.fs = afero.NewOsFs()
	}
}

type RealmConfig struct {
	// Regions is the set of regions that belong to this realm.
	Regions []string `mapstructure:"regions"`

	// IsGov is whether realm-resigner is on for this region.
	// This affects ${govExtension} variable.
	IsGov bool `mapstructure:"is_gov"`

	// IsONSR corresponds to `is_disconnected` value in region-data.json.
	IsONSR bool `mapstructure:"is_onsr"`
}

type ResolverOption func(*ResolverConfig) error

// newResolverConfig creates a new resolver config with the given options
func newResolverConfig(opts ...ResolverOption) (*ResolverConfig, error) {
	c := &ResolverConfig{}
	for _, o := range opts {
		if o == nil {
			continue
		}

		if err := o(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func WithResolverDefaults() ResolverOption {
	return func(c *ResolverConfig) error {
		c.ResolveVarsWith = []vars.ResolverKind{vars.IMDS}
		c.CanonicalRegionNames = DefaultCanonicalRegionNames()
		c.RealmConfigs = DefaultRealmConfigs()
		c.OverlayBastionHostclassRealmPrefixes = DefaultOverlayBastionHostclassRealmPrefixes()
		c.OverlayBastionHostclasses = DefaultOverlayBastionHostclasses()
		c.TouchEnforcedInRealms = DefaultTouchEnforcedInRealms()
		c.IMDS = imds.DefaultConfig()
		return nil
	}
}

func DefaultTouchEnforcedInRealms() []string {
	return []string{
		"oc2",
		"oc3",
		"oc6",
		"oc7",
		"oc11",
	}
}

func DefaultOverlayBastionHostclassRealmPrefixes() []string {
	return []string{
		"overlay-bastion-",
		"bastion-ob-",
	}
}

func DefaultOverlayBastionHostclasses() []string {
	return []string{
		"bastion-internal-host",
		"bastion-internal-host-test",
		"ztb-internal-host-test",
		"ztb-internal-host",
	}
}

func WithResolverLogger(logger logging.Interface) ResolverOption {
	return func(c *ResolverConfig) error {
		c.logger = logger
		return nil
	}
}

func WithResolverFs(fs afero.Fs) ResolverOption {
	return func(c *ResolverConfig) error {
		c.fs = fs
		return nil
	}
}

// WithResolverConfig sets the config directly to the given instance
func WithResolverConfig(config ResolverConfig) ResolverOption {
	return func(c *ResolverConfig) error {
		*c = config
		return nil
	}
}

// WithResolverFromViper applies the configuration using Viper. It assumes that Viper has
// already been configured to read from a config file, the environment, or flags.
func WithResolverFromViper(v *viper.Viper, configKey string) ResolverOption {
	if configKey == "" {
		configKey = ViperConfigKey
	}

	return func(c *ResolverConfig) error {
		if v == nil {
			return errors.New("nil viper")
		}

		if err := v.UnmarshalKey(configKey, c); err != nil {
			return err
		}

		return nil
	}
}

func (c *ResolverConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil config")
	}
	if err := c.validateRealmConfigs(); err != nil {
		return err
	}

	if err := c.validateOverlayBastionHostclassPrefixes(); err != nil {
		return fmt.Errorf("validating overlay_bastion_hostclass_prefixes: %w", err)
	}
	if err := c.validateOverlayBastionHostclasses(); err != nil {
		return fmt.Errorf("validating overlay_bastion_hostclasses: %w", err)
	}

	if err := c.validateResolvers(); err != nil {
		return err
	}

	return nil
}

func (c *ResolverConfig) validateResolvers() error {
	if len(c.ResolveVarsWith) == 0 {
		return fmt.Errorf("resolve_vars_with should contain at least 1 resolver")
	}

	kindsSeen := map[vars.ResolverKind]bool{}
	for _, kind := range c.ResolveVarsWith {
		if kindsSeen[kind] {
			return fmt.Errorf("duplicate resolver kind: %s", kind)
		}
		kindsSeen[kind] = true

		switch kind {
		case vars.Env:
			if err := c.Env.Validate(); err != nil {
				return fmt.Errorf("validating env config: %w", err)
			}
		case vars.Local:
		case vars.Fallback:
			if err := c.Fallback.Validate(); err != nil {
				return fmt.Errorf("validating fallback config: %w", err)
			}
		case vars.IMDS:
			if err := c.IMDS.Validate(); err != nil {
				return fmt.Errorf("validating imds config: %w", err)
			}
		default:
			return fmt.Errorf("unknown resolver kind: %s", kind)
		}
	}
	return nil
}

func (c *ResolverConfig) validateRealmConfigs() error {
	if len(c.RealmConfigs) != 0 {
		for realm, rConf := range c.RealmConfigs {
			if rConf == nil {
				return fmt.Errorf("realmConfigs[%s] is nil", realm)
			}
			if len(rConf.Regions) == 0 {
				return fmt.Errorf("realmConfigs[%s].regions is empty", realm)
			}
		}
	}
	return nil
}

func (c *ResolverConfig) validateOverlayBastionHostclassPrefixes() error {
	return validateOverlayBastionHostclassesMatchers(c.OverlayBastionHostclassRealmPrefixes)
}

func (c *ResolverConfig) validateOverlayBastionHostclasses() error {
	return validateOverlayBastionHostclassesMatchers(c.OverlayBastionHostclasses)
}

func validateOverlayBastionHostclassesMatchers(matchers []string) error {
	for _, value := range matchers {
		if value == "" {
			return errors.New("values cannot be empty strings")
		}

		if strings.ToLower(value) != value {
			return fmt.Errorf("values must be lowercase, but wasn't: %s", value)
		}
	}

	return nil
}
