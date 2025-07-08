package utils

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

// Version represents a semver compatible version
type Version struct {
	Major uint64
	Minor uint64
	Patch uint64
	Pre   []string
	Build []string //No Precedence
	Dev   []string
}

// we will only support these three type of version
//4.51.3-SAM-HQ-preview
//4.43.0.dev0
//4.43.0+build
// for safesensor 0.6.0

func Parse(s string) (Version, error) {
	if len(s) == 0 {
		return Version{}, errors.New("Version string empty")
	}

	// Split into major.minor.(patch+pr+meta)
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Version{}, errors.New("No Major.Minor.Patch elements found")
	}

	// Major
	if !containsOnly(parts[0], numbers) {
		return Version{}, fmt.Errorf("Invalid character(s) found in major number %q", parts[0])
	}
	if hasLeadingZeroes(parts[0]) {
		return Version{}, fmt.Errorf("Major number must not contain leading zeroes %q", parts[0])
	}
	major, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return Version{}, err
	}

	// Minor
	if !containsOnly(parts[1], numbers) {
		return Version{}, fmt.Errorf("Invalid character(s) found in minor number %q", parts[1])
	}
	if hasLeadingZeroes(parts[1]) {
		return Version{}, fmt.Errorf("Minor number must not contain leading zeroes %q", parts[1])
	}
	minor, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return Version{}, err
	}

	v := Version{}
	v.Major = major
	v.Minor = minor

	var build, prerelease, dev []string
	patchStr := parts[2]

	if buildIndex := strings.IndexRune(patchStr, '+'); buildIndex != -1 {
		build = strings.Split(patchStr[buildIndex+1:], ".")
		patchStr = patchStr[:buildIndex]
	}

	if preIndex := strings.IndexRune(patchStr, '-'); preIndex != -1 {
		prerelease = strings.Split(patchStr[preIndex+1:], ".")
		patchStr = patchStr[:preIndex]
	}

	if devIndex := strings.IndexRune(patchStr, '.'); devIndex != -1 {
		dev = strings.Split(patchStr[devIndex+1:], ".")
		patchStr = patchStr[:devIndex]
	}

	if !containsOnly(patchStr, numbers) {
		return Version{}, fmt.Errorf("Invalid character(s) found in patch number %q", patchStr)
	}
	if hasLeadingZeroes(patchStr) {
		return Version{}, fmt.Errorf("Patch number must not contain leading zeroes %q", patchStr)
	}
	patch, err := strconv.ParseUint(patchStr, 10, 64)
	if err != nil {
		return Version{}, err
	}

	v.Patch = patch

	// Prerelease
	for _, prstr := range prerelease {
		if len(prstr) == 0 {
			return Version{}, errors.New("Prerelease meta data is empty")
		}
		v.Pre = append(v.Pre, prstr)
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

func containsUnofficialVersion(v Version) bool {
	return len(v.Pre) > 0 || len(v.Build) > 0 || len(v.Dev) > 0
}

func Equal(v Version, o Version) bool {
	return CompareVersion(v, o) == 0
}

func GreaterThan(v Version, o Version) bool {
	return CompareVersion(v, o) == 1
}

// Compare compares Versions v to o:
// -1 == v is less than o
// 0 == v is equal to o
// 1 == v is greater than o
func CompareVersion(v Version, o Version) int {
	if v.Major != o.Major {
		if v.Major > o.Major {
			return 1
		}
		return -1
	}
	if v.Minor != o.Minor {
		if v.Minor > o.Minor {
			return 1
		}
		return -1
	}
	if v.Patch != o.Patch {
		if v.Patch > o.Patch {
			return 1
		}
		return -1
	}

	// quick comparison if a version has no prerelease versions
	if len(v.Pre) == 0 && len(o.Pre) == 0 {
		return 0
	}
	if len(v.Pre) == 0 && len(o.Pre) > 0 {
		return 1
	}
	if len(v.Pre) > 0 && len(o.Pre) == 0 {
		return -1
	}

	// compare all pre-release versions
	i := 0
	for ; i < len(v.Pre) && i < len(o.Pre); i++ {
		if v.Pre[i] != o.Pre[i] {
			return strings.Compare(v.Pre[i], o.Pre[i])
		}
	}

	// quick comparison if a version has no build metadata
	if len(v.Build) == 0 && len(o.Build) == 0 {
		return 0
	}
	if len(v.Build) == 0 && len(o.Build) > 0 {
		return 1
	}
	if len(v.Build) > 0 && len(o.Build) == 0 {
		return -1
	}

	// compare all build metadata
	for i := range v.Build {
		if v.Build[i] != o.Build[i] {
			return strings.Compare(v.Build[i], o.Build[i])
		}
	}

	// quick comparison if a version has no dev metadata
	if len(v.Dev) == 0 && len(o.Dev) == 0 {
		return 0
	}
	if len(v.Dev) == 0 && len(o.Dev) > 0 {
		return 1
	}
	if len(v.Dev) > 0 && len(o.Dev) == 0 {
		return -1
	}

	// compare all dev metadata
	for i := range v.Dev {
		if v.Dev[i] != o.Dev[i] {
			return strings.Compare(v.Dev[i], o.Dev[i])
		}
	}

	return 0
}
