package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/sgl-project/ome/pkg/constants"
)

/* NOTE TO AUTHORS:
 *
 * Only you can prevent ... the proliferation of useless "utility" classes.
 * Please add functional style container operations sparingly and intentionally.
 */

var gvResourcesCache map[string]*metav1.APIResourceList

func Filter(origin map[string]string, predicate func(string) bool) map[string]string {
	result := make(map[string]string)
	for k, v := range origin {
		if predicate(k) {
			result[k] = v
		}
	}
	return result
}

func Union(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func Includes(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func IncludesArg(slice []string, arg string) bool {
	for _, v := range slice {
		if v == arg || strings.HasPrefix(v, arg) {
			return true
		}
	}
	return false
}

func AppendVolumeIfNotExists(slice []v1.Volume, volume v1.Volume) []v1.Volume {
	for i := range slice {
		if slice[i].Name == volume.Name {
			return slice
		}
	}
	return append(slice, volume)
}

func IsGPUEnabled(requirements v1.ResourceRequirements) bool {
	_, ok := requirements.Limits[constants.NvidiaGPUResourceType]
	return ok
}

// FirstNonNilError returns the first non nil interface in the slice
func FirstNonNilError(objects []error) error {
	for _, object := range objects {
		if object != nil {
			return object
		}
	}
	return nil
}

// RemoveString Helper functions to remove string from a slice of strings.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// IsPrefixSupported Check if a given string contains one of the prefixes in the provided list.
func IsPrefixSupported(input string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

// MergeEnvs Merge a slice of EnvVars (`O`) into another slice of EnvVars (`B`), which does the following:
// 1. If an EnvVar is present in B but not in O, value remains unchanged in the result
// 2. If an EnvVar is present in `O` but not in `B`, appends to the result
// 3. If an EnvVar is present in both O and B, uses the value from O in the result
func MergeEnvs(baseEnvs []v1.EnvVar, overrideEnvs []v1.EnvVar) []v1.EnvVar {
	var extra []v1.EnvVar

	for _, override := range overrideEnvs {
		inBase := false

		for i, base := range baseEnvs {
			if override.Name == base.Name {
				inBase = true
				baseEnvs[i].Value = override.Value
				break
			}
		}

		if !inBase {
			extra = append(extra, override)
		}
	}

	return append(baseEnvs, extra...)
}

func AppendEnvVarIfNotExists(slice []v1.EnvVar, elems ...v1.EnvVar) []v1.EnvVar {
	for _, elem := range elems {
		isElemExists := false
		for _, item := range slice {
			if item.Name == elem.Name {
				isElemExists = true
				break
			}
		}
		if !isElemExists {
			slice = append(slice, elem)
		}
	}
	return slice
}

func AppendPortIfNotExists(slice []v1.ContainerPort, elems ...v1.ContainerPort) []v1.ContainerPort {
	for _, elem := range elems {
		isElemExists := false
		for _, item := range slice {
			if item.Name == elem.Name {
				isElemExists = true
				break
			}
		}
		if !isElemExists {
			slice = append(slice, elem)
		}
	}
	return slice
}

// IsCrdAvailable checks if a given CRD is present in the cluster by verifying the
// existence of its API.
func IsCrdAvailable(config *rest.Config, groupVersion, kind string) (bool, error) {
	gvResources, err := GetAvailableResourcesForApi(config, groupVersion)
	if err != nil {
		return false, err
	}

	found := false
	if gvResources != nil {
		for _, crd := range gvResources.APIResources {
			if crd.Kind == kind {
				found = true
				break
			}
		}
	}

	return found, nil
}

// GetAvailableResourcesForApi returns the list of discovered resources that belong
// to the API specified in groupVersion. The first query to a specifig groupVersion will
// query the cluster API server to discover the available resources and the discovered
// resources will be cached and returned to subsequent invocations to prevent additional
// queries to the API server.
func GetAvailableResourcesForApi(config *rest.Config, groupVersion string) (*metav1.APIResourceList, error) {
	var gvResources *metav1.APIResourceList
	var ok bool

	if gvResources, ok = gvResourcesCache[groupVersion]; !ok {
		discoveryClient, newClientErr := discovery.NewDiscoveryClientForConfig(config)
		if newClientErr != nil {
			return nil, newClientErr
		}

		var getGvResourcesErr error
		gvResources, getGvResourcesErr = discoveryClient.ServerResourcesForGroupVersion(groupVersion)
		if getGvResourcesErr != nil && !apierr.IsNotFound(getGvResourcesErr) {
			return nil, getGvResourcesErr
		}

		SetAvailableResourcesForApi(groupVersion, gvResources)
	}

	return gvResources, nil
}

// SetAvailableResourcesForApi stores the value fo resources argument in the global cache
// of discovered API resources. This function should never be called directly. It is exported
// for usage in tests.
func SetAvailableResourcesForApi(groupVersion string, resources *metav1.APIResourceList) {
	if gvResourcesCache == nil {
		gvResourcesCache = make(map[string]*metav1.APIResourceList)
	}

	gvResourcesCache[groupVersion] = resources
}

// IsStringEmptyOrWithWhitespaces checks if the string is empty or with whitespaces
var blankRegex = regexp.MustCompile(`\s`)

func IsStringEmptyOrWithWhitespaces(input string) bool {
	if blankRegex.MatchString(input) || input == "" {
		return true
	}
	return false
}

func Retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
	}

	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

// CreateSymbolicLink ensures that childPath is a symbolic link pointing to parentPath,
// using a relative link target computed from the directory of childPath to parentPath.
//
// Behavior:
// - Ensures the parent directory of childPath exists (creates it if necessary).
// - If childPath is an existing symlink with the same target, it is a no-op.
// - If childPath is an existing symlink with a different target, it is replaced.
// - If a non-symlink already exists at childPath, an error is returned.
//
// Note: The link target stored in the symlink is always a relative path.
func CreateSymbolicLink(childPath string, parentPath string) error {
	// Ensure the parent directory of childPath exists
	childDir := filepath.Dir(childPath)
	if err := os.MkdirAll(childDir, 0755); err != nil {
		return fmt.Errorf("failed to create child directory %s: %w", childDir, err)
	}

	// Compute relative path from childPath to parentPath
	relTarget, err := filepath.Rel(childDir, parentPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// Check if symlink already exists
	fileInfo, err := os.Lstat(childPath)
	if err == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, read its target
			currentTarget, err := os.Readlink(childPath)
			if err != nil {
				return fmt.Errorf("failed to read existing symlink: %w", err)
			}

			if currentTarget == relTarget {
				// Already points to the desired target, no-op
				return nil
			}

			// Remove existing symlink
			if err := os.Remove(childPath); err != nil {
				return fmt.Errorf("failed to remove existing symlink: %w", err)
			}
		} else {
			return fmt.Errorf("file exists and is not a symlink: %s", childPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat childPath: %w", err)
	}

	// Create the symlink
	if err := os.Symlink(relTarget, childPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// ContainsString reports whether values contains target.
// If isCaseSensitive is true, comparison is case-sensitive; otherwise it is case-insensitive.
// Non-string elements in values are ignored.
func ContainsString(values []interface{}, target string, isCaseSensitive bool) bool {
	for _, v := range values {
		if s, ok := v.(string); ok {
			var result bool
			if isCaseSensitive {
				result = s == target
			} else {
				result = strings.ToLower(s) == strings.ToLower(target)
			}
			if result {
				return result
			}
		}
	}
	return false
}

func HasSymlinkPointingToDir(searchDir, targetDir string) (bool, error) {
	searchDir, err := filepath.Abs(searchDir)
	if err != nil {
		return false, err
	}

	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return false, err
	}
	var errFound = errors.New("symlink pointing to target found")

	err = filepath.WalkDir(searchDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip evaluating targetDir itself, but DO NOT SkipDir
		if path == targetDir {
			return nil
		}

		// Only inspect symlinks
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}

		linkTarget, err := os.Readlink(path)
		if err != nil {
			return nil
		}

		// Resolve relative symlink
		if !filepath.IsAbs(linkTarget) {
			linkTarget = filepath.Join(filepath.Dir(path), linkTarget)
		}

		linkTarget = filepath.Clean(linkTarget)

		if linkTarget == targetDir {
			return errFound
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errFound) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}
