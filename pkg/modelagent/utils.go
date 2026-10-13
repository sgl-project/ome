package modelagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// canonicalModelPath resolves directory aliases but leaves the final component
// unresolved: deleting a model symlink must remove the link, not its target.
func canonicalModelPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("model path is empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for ancestor := filepath.Dir(path); ; ancestor = filepath.Dir(ancestor) {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			relative, err := filepath.Rel(ancestor, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, relative), nil
		}
		if !os.IsNotExist(err) || ancestor == filepath.Dir(ancestor) {
			return "", err
		}
	}
}

func modelPathWithin(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func sameStringPtr(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func sameStringMapPtr(left *map[string]string, right *map[string]string) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(*left) != len(*right) {
		return false
	}
	for key, leftValue := range *left {
		if rightValue, ok := (*right)[key]; !ok || rightValue != leftValue {
			return false
		}
	}
	return true
}
