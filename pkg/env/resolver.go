package env

import (
	"fmt"
	"github.com/spf13/afero"
	"os"
	"sort"
	"strings"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
	"github.com/hashicorp/go-multierror"
)

type Resolver interface {
	Resolve() (*Environment, error)
}

// resolver is responsible for automatic resolution of the environment that the host is running on.
//
// Use New() to construct a custom environment.
type resolver struct {
	config       *ResolverConfig
	varResolvers []vars.Resolver

	regionToRealm map[string]string
}

var _ Resolver = &resolver{}

// newResolver constructs a new resolver.
func newResolver(c *ResolverConfig) (*resolver, error) {
	c.ensureDefaults()

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	varResolvers, err := makeResolvers(c, c.fs)
	if err != nil {
		return nil, fmt.Errorf("creating var resolvers from config: %w", err)
	}

	return newResolverUsing(c, varResolvers)
}

// newResolverUsing constructs a new resolver using given varResolvers.
//
// Config validation should have already happened in newResolver.
func newResolverUsing(c *ResolverConfig, varResolvers []vars.Resolver) (*resolver, error) {
	er := resolver{
		config:       c,
		varResolvers: varResolvers,
	}
	er.makeRegionToRealmMap()

	return &er, nil
}

func makeResolvers(config *ResolverConfig, fs afero.Fs) ([]vars.Resolver, error) {
	if err := config.validateResolvers(); err != nil {
		return nil, err
	}

	var varResolvers []vars.Resolver
	for _, resolverKind := range config.ResolveVarsWith {
		var (
			resolver vars.Resolver
			err      error
		)
		switch resolverKind {
		case vars.Local:
			resolver, err = vars.NewLocalResolver(config.Local, fs)
			if err != nil {
				return nil, fmt.Errorf("local resolver: %w", err)
			}
		case vars.Env:
			resolver, err = vars.NewEnvResolver(config.Env)
			if err != nil {
				return nil, fmt.Errorf("env resolver: %w", err)
			}
		case vars.IMDS:
			resolver, err = vars.NewIMDSResolver(config.IMDS, config.logger)
			if err != nil {
				return nil, fmt.Errorf("imds resolver: %w", err)
			}
		case vars.Fallback:
			resolver, err = vars.NewFallbackResolver(config.Fallback)
			if err != nil {
				return nil, fmt.Errorf("fallback resolver: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown resolver kind: %s", resolverKind)
		}

		varResolvers = append(varResolvers, resolver)
	}

	return varResolvers, nil
}

func (e *resolver) Resolve() (*Environment, error) {
	e.config.logger.Info("Resolving environment...")

	allVars, err := e.collectVars()
	if err != nil {
		return nil, fmt.Errorf("collecting vars: %w", err)
	}

	var (
		region = e.resolveRegion()
		ad     = e.resolveAd()
		realm  = e.resolveRealm(region)

		// existence of key means it's resolved (even if value is empty string).
		// env.Resolve will fail if the key is not found
		resolved = make(map[vars.Var]string, len(allVars))
	)

	resolved[vars.Region] = region
	resolved[vars.Ad] = ad
	resolved[vars.Realm] = realm.Name()

	resolved[vars.GovExtension] = e.resolveGovExtension(realm)
	resolved[vars.RealmTLD] = e.resolveRealmTLD(realm)
	resolved[vars.InternalRealmTLD] = e.resolveInternalRealmTLD(realm)

	for v := range allVars {
		if e.isBuiltinVar(v) {
			e.config.logger.WithField("var", v).Debug("Not overriding builtin var")
			continue
		}

		value, err := e.resolveVar(v)
		if err != nil {
			e.config.logger.WithField("var", v).WithError(err).Debug("Can't resolve var")
			continue
		}

		resolved[v] = value
	}

	e.config.logger.WithField("values", mapToString(resolved)).Info("Resolved environment")

	return New(
		WithResolvedVars(resolved),
		WithIsGov(e.isGovRealm(realm)),
		WithIsONSR(e.isOnsrRealm(realm)),
		WithIsOverlayBastion(e.isOverlayBastion(resolved, realm)),
		WithIsTouchEnforcedForRealm(e.isTouchEnforcedForRealm(realm)),
	), nil
}

func (e *resolver) isBuiltinVar(v vars.Var) bool {
	return v == vars.Realm || v == vars.Region || v == vars.Ad || v == vars.GovExtension
}

func (e *resolver) resolveVar(v vars.Var) (string, error) {
	var resultErr *multierror.Error
	for _, varResolver := range e.varResolvers {
		if !vars.CanPotentiallyResolve(varResolver, v) {
			continue
		}

		vv, err := varResolver.Resolve(v)
		if err != nil {
			resultErr = multierror.Append(resultErr, err)
			continue
		}

		return vv, nil
	}

	return "", fmt.Errorf("resolving var '%v': %w", v, resultErr.ErrorOrNil())
}

func (e *resolver) resolveAd() string {
	ad, err := e.resolveVar(vars.Ad)
	if err != nil {
		e.config.logger.WithError(err).Warn("ad couldn't be resolved")
		return "<ad not resolved>"
	}

	if strings.HasPrefix(ad, "pop") {
		return "ad" + strings.TrimPrefix(ad, "pop")
	}
	return ad
}

func (e *resolver) resolveRegion() string {
	region, err := e.resolveVar(vars.Region)
	if err != nil {
		e.config.logger.WithError(err).Warn("region couldn't be resolved from region file, falling back to env var")

		region, exist := os.LookupEnv("REGION")
		if exist {
			return region
		} else {
			return "<region not resolved>"
		}
	}

	actualRegion, ok := e.canonicalRegion(region)
	if ok {
		region = actualRegion
	}

	return region
}

func (e *resolver) resolveRealm(region string) Realm {
	realm, err := e.resolveVar(vars.Realm)
	if err != nil {
		e.config.logger.WithError(err).Debug("realm couldn't be resolved, falling back to region->realm lookup")

		var ok bool
		if realm, ok = e.regionToRealm[region]; !ok {
			e.config.logger.WithError(fmt.Errorf("regionToRealm[%s] not found", region)).Warn("realm couldn't be resolved")
			return "<realm not resolved>"
		}
	}

	return MakeRealm(realm)
}

func (e *resolver) resolveGovExtension(realm Realm) string {
	isGov := e.isGovRealm(realm)

	govExtension := ""
	if isGov {
		govExtension = ".10x"
	}
	return govExtension
}

// resolveInternalRealmTLD resolves internal top-level domain of that realm (without preceding .)
func (e *resolver) resolveInternalRealmTLD(realm Realm) string {
	return e.resolveTLD(vars.InternalRealmTLD, realm.InternalTLD())
}

// resolveRealmTLD resolves top-level domain of that realm (without preceding .)
func (e *resolver) resolveRealmTLD(realm Realm) string {
	return e.resolveTLD(vars.RealmTLD, realm.TLD())
}

// resolveTLD is a helper function for resolving realmTLD/internalRealmTLD.
func (e *resolver) resolveTLD(v vars.Var, fallbackValue string) string {
	if resolved, err := e.resolveVar(v); err == nil {
		return resolved
	}

	if fallbackValue != "" { // fallback to cover where IMDS is not reachable or if an error is thrown
		return fallbackValue
	}

	return fmt.Sprintf("<%s not resolved>", v.Name())
}

func (e *resolver) makeRegionToRealmMap() {
	e.regionToRealm = make(map[string]string)

	for realm, realmConfig := range e.config.RealmConfigs {
		for _, region := range realmConfig.Regions {
			e.regionToRealm[region] = realm
			if canonicalRegion, ok := e.canonicalRegion(region); ok {
				e.regionToRealm[canonicalRegion] = realm
			}
		}
	}
}

func (e *resolver) canonicalRegion(region string) (string, bool) {
	actualRegion, ok := e.config.CanonicalRegionNames[region]
	return actualRegion, ok
}

func (e *resolver) isGovRealm(realm Realm) bool {
	if rc, ok := e.config.RealmConfigs[realm.Name()]; ok {
		return rc.IsGov
	}

	return false
}

func (e *resolver) isOnsrRealm(realm Realm) bool {
	if rc, ok := e.config.RealmConfigs[realm.Name()]; ok {
		return rc.IsONSR
	}

	return false
}

func (e *resolver) isOverlayBastion(resolved map[vars.Var]string, realm Realm) bool {
	hc, ok := resolved[vars.Hostclass]
	if hc == "" || !ok {
		// hostclass is unknown, defaulting to normal overlay instance
		return false
	}

	// Keep resolved[vars.Hostclass] at whatever casing it was,
	// but for this method it's easier to check this way
	hc = strings.ToLower(hc)

	// New hostclasses that don't have realm as suffix
	for _, candidate := range e.config.OverlayBastionHostclasses {
		if hc == candidate {
			return true
		}
	}

	// Legacy OB3 bastions have a ${realm} suffix
	for _, prefix := range e.config.OverlayBastionHostclassRealmPrefixes {
		candidate := prefix + realm.Name()
		if hc == candidate {
			return true
		}
	}

	return false
}

func (e *resolver) isTouchEnforcedForRealm(realm Realm) bool {
	for _, candidate := range e.config.TouchEnforcedInRealms {
		if strings.ToLower(candidate) == realm.Name() {
			return true
		}
	}

	return false
}

// collectVars collects all variables defined by resolvers
func (e *resolver) collectVars() (map[vars.Var]bool, error) {
	resultMap := make(map[vars.Var]bool)
	for _, resolver := range e.varResolvers {
		resolverVars := resolver.CanResolve()
		for _, v := range resolverVars {
			resultMap[v] = true
		}
	}

	return resultMap, nil
}

func mapToString(m map[vars.Var]string) string {
	// this needs to be made deterministic otherwise the test fails
	// to do that lets sort the keys
	var sb strings.Builder
	keys := make([]vars.Var, 0)
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Name() < keys[j].Name() })

	for i, k := range keys {
		if i != 0 {
			_, _ = sb.WriteString(", ")
		}

		_, _ = fmt.Fprintf(&sb, "%s: %s", k.Name(), m[k])
	}
	return sb.String()
}
