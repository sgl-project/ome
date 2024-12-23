package vars

import (
	"fmt"

	"github.com/spf13/afero"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vibe"
)

var (
	VibeRegion           = MustNewVar("vibe_target_region", false)
	VibeRealm            = MustNewVar("vibe_target_realm", false)
	VibeAirportCode      = MustNewVar("vibe_target_airport", false)
	VibeSvcEnclaveSuffix = MustNewVar("vibe_service_enclave_suffix", false)
	VibeIdentityRealm    = MustNewVar("vibe_identity_realm", false)
	VibeLdapifiedRealm   = MustNewVar("vibe_target_realm_ldapify", false)
	VibeLdapifiedRegion  = MustNewVar("vibe_target_region_ldapify", false)
)

// VibeResolver represents a vibe-metadata var resolver.
type VibeResolver struct {
	provider vibe.Client // provider for vibe metadata.
}

// CanResolve returns a list of vars that this Resolver can resolve.
func (res VibeResolver) CanResolve() []Var {
	return []Var{
		VibeRealm,
		VibeRegion,
		VibeAirportCode,
		VibeSvcEnclaveSuffix,
		VibeIdentityRealm,
		VibeLdapifiedRealm,
		VibeLdapifiedRegion,
	}
}

// Resolve resolves the value of a given Var.
func (res VibeResolver) Resolve(v Var) (string, error) {
	switch v {
	case VibeRealm:
		return res.provider.GetRealm(), nil
	case VibeRegion:
		return res.provider.GetRegion(), nil
	case VibeAirportCode:
		return res.provider.GetAirportCode(), nil
	case VibeSvcEnclaveSuffix:
		return res.provider.GetSvcEnclaveSuffix(), nil
	case VibeIdentityRealm:
		return res.provider.GetIdentityRealm(), nil
	case VibeLdapifiedRealm:
		return res.provider.GetLdapifiedRealm(), nil
	case VibeLdapifiedRegion:
		return res.provider.GetLdapifiedRegion(), nil
	}

	return "", fmt.Errorf("VibeResolver can't resolve var: %v", v)
}

// NewVibeResolver returns vibe metadata resolver.
func NewVibeResolver(config vibe.Config, fs afero.Fs) (*VibeResolver, error) {
	provider, err := vibe.NewMetadataClient(config, fs)
	if err != nil {
		return nil, fmt.Errorf("constructing vibe provider: %w", err)
	}

	return &VibeResolver{
		provider: provider,
	}, nil
}
