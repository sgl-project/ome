package vars

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	PlaceholderStart = "${"
	PlaceholderEnd   = "}"
)

type Var struct {
	name   string
	isOcid bool
}

func (v Var) Name() string          { return v.name }
func (v Var) IsOcid() bool          { return v.isOcid }
func (v Var) IsHostClassName() bool { return v == Hostclass }

// String returns ${v.name}
func (v Var) String() string {
	return fmt.Sprintf("%s%s%s", PlaceholderStart, v.name, PlaceholderEnd)
}

var (
	validVarRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_\-]*$`)
)

// NewVar creates a new variable using a given name,
// or returns an error if the name is invalid.
func NewVar(name string, isOcid bool) (Var, error) {
	if err := validateVarName(name); err != nil {
		return Var{}, err
	}

	return Var{name: name, isOcid: isOcid}, nil
}

func validateVarName(name string) error {
	if !validVarRegex.MatchString(name) {
		return fmt.Errorf("var %s doesn't match regex '%s'", name, validVarRegex.String())
	}

	return nil
}

// MustNewVar creates a new variable or panics if the name is invalid.
func MustNewVar(name string, isOcid bool) Var {
	r, err := NewVar(name, isOcid)
	if err != nil {
		panic(err)
	}

	return r
}

var (
	Ad     = MustNewVar("ad", false)
	Realm  = MustNewVar("realm", false)
	Region = MustNewVar("region", false)

	// Resolves to the hostclass name in lower case
	Hostclass = MustNewVar("hostclass", false)

	// Resolves to .10x for gov regions and to empty string otherwise
	GovExtension = MustNewVar("govExtension", false)

	// RegionSE is the same as Region, but has all the hack-arounds to ensure it can be used directly to
	// construct ServiceEnclave endpoints (e.g. us-phoenix-1 -> r2, us-seattle-1 -> r1 etc.)
	RegionSE = MustNewVar("regionSE", false)

	// RealmTLD resolves to top-level public domain of OCI services (without preceding .)
	// Used by external-customer facing endpoints (e.g. ObjectStorage, Identity)
	RealmTLD = MustNewVar("realmTLD", false)

	// InternalRealmTLD resolves to top-level public domain of OCI services (without preceding .)
	// Used by OCI internal endpoints (e.g. SSH-CA)
	InternalRealmTLD = MustNewVar("internalRealmTLD", false)
)

type ResolverKind string

const (
	Env      ResolverKind = "env"
	IMDS     ResolverKind = "imds"
	Local    ResolverKind = "local"
	Fallback ResolverKind = "fallback"
)

type Resolver interface {
	// Resolve resolves the value of a given Var
	Resolve(v Var) (string, error)

	// CanResolve returns a list of vars that this Resolver can resolve
	CanResolve() []Var
}

// CanPotentiallyResolve returns whether a given resolver can potentially resolve a given variable.
func CanPotentiallyResolve(resolver Resolver, v Var) bool {
	for _, candidate := range resolver.CanResolve() {
		if candidate == v {
			return true
		}
	}

	return false
}

func escapeOCID(ocid string) string {
	if strings.Contains(ocid, ":") {
		return strings.ReplaceAll(ocid, ":", "-")
	}
	return strings.ReplaceAll(ocid, ".", "-")
}
