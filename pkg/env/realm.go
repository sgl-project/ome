package env

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

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
	// Region1 is an alias for REGION1 (backward-compatibility).
	//
	// Deprecated: use REGION1 instead.
	Region1 = REGION1
)

var (
	// realmPrefixPrecedence allows us to apply our own custom sorting:
	// regionX -> ocY -> rbZ.
	realmPrefixPrecedence = map[string]int{
		"region": 0,
		"oc":     1,
		"rb":     2,
	}
)

// Realm is a lower-case realm name. Use MakeRealm to parse it.
type Realm string

// MakeRealm creates a new realm.
func MakeRealm(realm string) Realm {
	return Realm(strings.ToLower(realm))
}

// Name is the official (lowercase) name of the realm.
func (r Realm) Name() string { return string(r) }

// TLD is the official top-level domain for customer-facing endpoints (no preceding .)
// E.g.: ObjectStorage, Identity.
func (r Realm) TLD() string {
	if result, ok := realmTLDs[r]; ok {
		return result
	}

	return tryDefaultNamingConvention(r, realmTldTemplate)
}

// InternalTLD is the official top-level domain for internal OCI-facing endpoints (no preceding .)
// E.g.: SSH-CA.
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

// Less returns whether this realm should be earlier
// in the list than the other during sorting.
func (r Realm) Less(other Realm) bool {
	return IsRealmLess(r.Name(), other.Name())
}

// IsRealmLess returns whether this realm should be earlier
// in the list than the other during sorting.
func IsRealmLess(left, right string) bool {
	if left == right {
		return false
	}

	leftFirstDigit := strings.IndexFunc(left, unicode.IsDigit)
	if leftFirstDigit == -1 {
		leftFirstDigit = len(left)
	}

	rightFirstDigit := strings.IndexFunc(right, unicode.IsDigit)
	if rightFirstDigit == -1 {
		rightFirstDigit = len(right)
	}

	leftPrefix := left[:leftFirstDigit]
	rightPrefix := right[:rightFirstDigit]

	leftRealmPrecedence := realmPrefixPrecedence[leftPrefix]
	rightRealmPrecedence := realmPrefixPrecedence[rightPrefix]
	if leftRealmPrecedence != rightRealmPrecedence {
		return leftRealmPrecedence < rightRealmPrecedence
	}

	leftSuffix := left[leftFirstDigit:]
	rightSuffix := right[rightFirstDigit:]

	leftNum, err := strconv.Atoi(leftSuffix)
	if err != nil {
		return strings.Compare(leftSuffix, rightSuffix) < 0
	}
	rightNum, err := strconv.Atoi(rightSuffix)
	if err != nil {
		return strings.Compare(leftSuffix, rightSuffix) < 0
	}

	return leftNum < rightNum
}
