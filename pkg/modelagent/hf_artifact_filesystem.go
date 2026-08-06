package modelagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils"
)

type artifactFileSystem interface {
	ParentExists(path string) bool
	HasReadyMarker(path string) bool
	ReadyMarkerMatches(path, reservationToken string) bool
	WriteReadyMarker(path, reservationToken string) error
	RemoveReadyMarker(path string) error
	ChildPointsTo(childPath, parentPath string) bool
	LinkChild(childPath, parentPath string) error
	RemoveChild(path string) error
	RemoveParent(path string) error
	HasOtherChild(parentPath, searchRoot string) (bool, error)
}

type osArtifactFileSystem struct{}

func newOSArtifactFileSystem() artifactFileSystem {
	return osArtifactFileSystem{}
}

func (osArtifactFileSystem) ParentExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (osArtifactFileSystem) HasReadyMarker(path string) bool {
	info, err := os.Stat(filepath.Join(path, constants.HfArtifactReadyMarkerFileName))
	return err == nil && !info.IsDir()
}

func (osArtifactFileSystem) ReadyMarkerMatches(path, reservationToken string) bool {
	if strings.TrimSpace(reservationToken) == "" {
		return false
	}
	content, err := os.ReadFile(filepath.Join(path, constants.HfArtifactReadyMarkerFileName))
	return err == nil && strings.TrimSpace(string(content)) == reservationToken
}

func (osArtifactFileSystem) WriteReadyMarker(path, reservationToken string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(path, constants.HfArtifactReadyMarkerFileName)
	body := strings.TrimSpace(reservationToken)
	if body == "" {
		body = constants.ArtifactCompleteMarkerBody
	}
	return os.WriteFile(marker, []byte(body), 0o644)
}

func (osArtifactFileSystem) RemoveReadyMarker(path string) error {
	err := os.Remove(filepath.Join(path, constants.HfArtifactReadyMarkerFileName))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (osArtifactFileSystem) ChildPointsTo(childPath, parentPath string) bool {
	childTarget, err := filepath.EvalSymlinks(childPath)
	if err != nil {
		return false
	}
	parentTarget, err := filepath.EvalSymlinks(parentPath)
	return err == nil && filepath.Clean(childTarget) == filepath.Clean(parentTarget)
}

func (osArtifactFileSystem) LinkChild(childPath, parentPath string) error {
	return utils.CreateSymbolicLink(childPath, parentPath)
}

func (osArtifactFileSystem) RemoveChild(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("shared artifact child path %s is not a symbolic link", path)
	}
	return os.Remove(path)
}

func (osArtifactFileSystem) RemoveParent(path string) error {
	return os.RemoveAll(path)
}

func (osArtifactFileSystem) HasOtherChild(parentPath, searchRoot string) (bool, error) {
	return utils.HasSymlinkPointingToDir(searchRoot, parentPath)
}

func hasHfArtifactReadyMarker(path string) bool {
	return osArtifactFileSystem{}.HasReadyMarker(path)
}

func writeHfArtifactReadyMarker(path string, reservationToken ...string) error {
	token := ""
	if len(reservationToken) > 0 {
		token = reservationToken[0]
	}
	return osArtifactFileSystem{}.WriteReadyMarker(path, token)
}

func removeHfArtifactReadyMarker(path string) error {
	return osArtifactFileSystem{}.RemoveReadyMarker(path)
}
