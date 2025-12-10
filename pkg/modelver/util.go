package modelver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	numbers  string = "0123456789"
	alphas          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"
	alphanum        = alphas + numbers
)

var (
	// acceptedMajorVersionPrefixes contains the accepted prefixes for major version
	// Currently only "v" is supported, but keep as a slice for future extensibility
	acceptedMajorVersionPrefixes = []string{"v"}
)

// Version represents a semver compatible version
type Version struct {
	Major       uint64
	MajorPrefix string
	Minor       uint64
	Patch       uint64
	Pre         []string
	Build       []string //No Precedence
	Dev         []string
	Precision   int // Number of version parts specified (1=major, 2=major.minor, 3=major.minor.patch)
}

// Parse parses version strings like:
//   - 4.51.3-SAM-HQ-preview
//   - 4.43.0.dev0
//   - 4.43.0+build
//   - 0.6.0
//   - v0.8.0 (only lowercase "v" is supported as major version prefix)
//   - 1 (single major version)
//   - v1
//   - 1.12 (two-part version)
//   - v1.12

func Parse(s string) (Version, error) {
	if len(s) == 0 {
		return Version{}, errors.New("Version string empty")
	}

	// Split into major.minor.(patch+pr+meta)
	parts := strings.SplitN(s, ".", 3)
	precision := len(parts)

	// Check if parts[0] starts with an accepted prefix
	majorStr := parts[0]
	majorPrefix := ""
	for _, prefix := range acceptedMajorVersionPrefixes {
		if strings.HasPrefix(majorStr, prefix) {
			majorPrefix = prefix
			majorStr = strings.TrimPrefix(majorStr, prefix)
			break
		}
	}

	// Major
	major, err := parseNumeric(majorStr, "major")
	if err != nil {
		return Version{}, err
	}

	// Minor
	minorStr := "0"
	if precision > 1 {
		minorStr = parts[1]
	}
	minor, err := parseNumeric(minorStr, "minor")
	if err != nil {
		return Version{}, err
	}

	v := Version{Major: major, MajorPrefix: majorPrefix, Minor: minor, Precision: precision}

	// Patch
	var (
		build, prerelease, dev []string
	)
	patchStr := "0"
	if precision > 2 {
		patchStr = parts[2]
	}

	// Extract +build
	if buildIndex := strings.IndexRune(patchStr, '+'); buildIndex != -1 {
		build = strings.Split(patchStr[buildIndex+1:], ".")
		patchStr = patchStr[:buildIndex]
	}

	// Extract -preview
	if preIndex := strings.IndexRune(patchStr, '-'); preIndex != -1 {
		prerelease = strings.Split(patchStr[preIndex+1:], ".")
		patchStr = patchStr[:preIndex]
	}

	// Extract .dev
	if devIndex := strings.IndexRune(patchStr, '.'); devIndex != -1 {
		dev = strings.Split(patchStr[devIndex+1:], ".")
		patchStr = patchStr[:devIndex]
	}

	patch, err := parseNumeric(patchStr, "patch")
	if err != nil {
		return Version{}, err
	}

	v.Patch = patch

	// Prerelease
	for _, str := range prerelease {
		if len(str) == 0 {
			return Version{}, errors.New("Prerelease meta data is empty")
		}
		v.Pre = append(v.Pre, str)
	}

	// Build meta data
	for _, str := range build {
		if len(str) == 0 {
			return Version{}, errors.New("Build meta data is empty")
		}
		v.Build = append(v.Build, str)
	}

	// DEV
	for _, str := range dev {
		if len(str) == 0 {
			return Version{}, errors.New("Dev meta data is empty")
		}
		v.Dev = append(v.Dev, str)
	}

	return v, nil
}

func containsOnly(s string, set string) bool {
	for _, c := range s {
		if !strings.ContainsRune(set, c) {
			return false
		}
	}
	return true
}

func hasLeadingZeroes(s string) bool {
	return len(s) > 1 && s[0] == '0'
}

func ContainsUnofficialVersion(v Version) bool {
	return len(v.Pre) > 0 || len(v.Build) > 0 || len(v.Dev) > 0
}

func Equal(v Version, o Version) bool {
	return CompareVersion(v, o) == 0
}

func GreaterThan(v Version, o Version) bool {
	return CompareVersion(v, o) == 1
}

func GreaterThanOrEqual(v Version, o Version) bool {
	return CompareVersion(v, o) >= 0
}

// Compare compares Versions v to o:
// -1 == v is less than o
// 0 == v is equal to o
// 1 == v is greater than o
func CompareVersion(v, o Version) int {
	if cmp := compareUint64(v.Major, o.Major); cmp != 0 {
		return cmp
	}
	if cmp := compareUint64(v.Minor, o.Minor); cmp != 0 {
		return cmp
	}
	if cmp := compareUint64(v.Patch, o.Patch); cmp != 0 {
		return cmp
	}

	if cmp := compareStringSlices(v.Pre, o.Pre); cmp != 0 {
		return cmp
	}
	if cmp := compareStringSlices(v.Build, o.Build); cmp != 0 {
		return cmp
	}
	if cmp := compareStringSlices(v.Dev, o.Dev); cmp != 0 {
		return cmp
	}

	return 0
}

func compareUint64(a, b uint64) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	default:
		return 0
	}
}

func compareStringSlices(a, b []string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return strings.Compare(a[i], b[i])
		}
	}
	return compareUint64(uint64(len(a)), uint64(len(b)))
}

func parseNumeric(s, field string) (uint64, error) {
	if !containsOnly(s, numbers) {
		return 0, fmt.Errorf("invalid character(s) in %s number %q", field, s)
	}
	if hasLeadingZeroes(s) {
		return 0, fmt.Errorf("%s must not have leading zeroes: %q", field, s)
	}
	return strconv.ParseUint(s, 10, 64)
}
