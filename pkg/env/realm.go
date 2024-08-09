package env

import (
	"fmt"
	"strings"
)

// Realm is a lower-case realm name. Use MakeRealm to parse it.
type Realm string

func MakeRealm(realm string) Realm {
	return Realm(strings.ToLower(realm))
}

const (
	// realmTLD/internalRealmTLD templates are used for realms
	// that follow standard naming convention.
	//
	// Other realms are exceptional and their values are hardcoded
	// in realmTLDs/internalRealmTLDs maps.

	realmTldTemplate         = "oraclecloud%s.com"
	internalRealmTldTemplate = "oraclerealm%s.com"
)

const (
	Region1 Realm = "region1"
	OC0     Realm = "oc0"
	OC1     Realm = "oc1"
	OC2     Realm = "oc2"
	OC3     Realm = "oc3"
	OC4     Realm = "oc4"
	OC5     Realm = "oc5"
	OC6     Realm = "oc6"
	OC7     Realm = "oc7"
	OC8     Realm = "oc8"
	OC9     Realm = "oc9"
	OC10    Realm = "oc10"
	OC11    Realm = "oc11"
	OC12    Realm = "oc12"
	OC14    Realm = "oc14"
	OC16    Realm = "oc16"
	OC17    Realm = "oc17"
	OC18    Realm = "oc18"
	OC19    Realm = "oc19"
	OC20    Realm = "oc20"
)

var (
	// corresponds to publicDomainName
	realmTLDs = map[Realm]string{
		Region1: "oracleiaas.com",
		OC1:     "oraclecloud.com",
		OC2:     "oraclegovcloud.com",
		OC3:     "oraclegovcloud.com",
		OC4:     "oraclegovcloud.uk",
		OC6:     "oraclecloud.ic.gov",
		OC7:     "oc.ic.gov",
		OC11:    "oraclecloud.smil.mil",
		OC12:    "oracledodcloud.ic.gov",
		OC19:    "oraclecloud.eu",
	}

	// corresponds to iaasDomainName
	internalRealmTLDs = map[Realm]string{
		Region1: "oracleiaas.com",
		OC1:     "oracleiaas.com",
		OC2:     "oraclegoviaas.com",
		OC3:     "oraclegoviaas.com",
		OC4:     "oraclegoviaas.uk",
		OC6:     "oraclerealm.ic.gov",
		OC7:     "oci.ic.gov",
		OC11:    "oraclerealm.smil.mil",
		OC12:    "oracledodrealm.ic.gov",
		OC19:    "oraclerealm.eu",
	}
)

// Name is the official (lowercase) name of the realm
func (r Realm) Name() string { return string(r) }

// TLD is the official top-level domain for customer-facing endpoints (no preceding .)
// E.g.: ObjectStorage, Identity
func (r Realm) TLD() string {
	if result, ok := realmTLDs[r]; ok {
		return result
	}

	return tryDefaultNamingConvention(r, realmTldTemplate)
}

// InternalTLD is the official top-level domain for internal OCI-facing endpoints (no preceding .)
// E.g.: SSH-CA
func (r Realm) InternalTLD() string {
	if result, ok := internalRealmTLDs[r]; ok {
		return result
	}

	return tryDefaultNamingConvention(r, internalRealmTldTemplate)
}

func tryDefaultNamingConvention(r Realm, template string) string {
	realmStr := strings.ToLower(string(r))
	if strings.HasPrefix(realmStr, "oc") {
		// most likely follows the oraclecloudXX.com convention
		return fmt.Sprintf(template, realmStr[2:])
	}

	return ""
}
