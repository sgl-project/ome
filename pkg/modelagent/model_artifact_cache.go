package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultMountedArtifactCacheKeyRoot is the default relative namespace under each mounted cache root.
	DefaultMountedArtifactCacheKeyRoot = "_artifacts"
	defaultMountedArtifactCacheKeyRoot = DefaultMountedArtifactCacheKeyRoot

	mountedArtifactCacheManifestFile     = "manifest.json"
	mountedArtifactCacheReadyMarkerFile  = "_READY"
	maxMountedArtifactCacheManifestBytes = 64 * 1024 * 1024
)

var errMountedArtifactCacheDestinationExists = errors.New("mounted artifact cache destination exists")

type ModelArtifactCacheConfig struct {
	Enabled        bool
	Mounts         []string
	KeyRoot        string
	SourceRequired bool
}

type mountedArtifactCacheManifest struct {
	Files []mountedArtifactCacheManifestFileEntry `json:"files"`
}

type mountedArtifactCacheManifestFileEntry struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	Size int64  `json:"size"`
}

func (c ModelArtifactCacheConfig) normalized() ModelArtifactCacheConfig {
	c.KeyRoot = normalizeMountedArtifactCacheKeyRoot(c.KeyRoot)
	mounts := make([]string, 0, len(c.Mounts))
	for _, mount := range c.Mounts {
		mount = strings.TrimSpace(mount)
		if mount != "" {
			mounts = append(mounts, mount)
		}
	}
	c.Mounts = mounts
	return c
}

func (c ModelArtifactCacheConfig) enabled() bool {
	return c.Enabled && len(c.Mounts) > 0
}

func mountedArtifactCacheKeyFromHuggingFaceIdentity(identity ArtifactIdentity) (string, bool) {
	if !identity.isValid() {
		return "", false
	}
	return filepath.Join(filepath.FromSlash(strings.Trim(identity.HFModelID, "/")), strings.ToLower(identity.HFCommitSHA)), true
}

func normalizeMountedArtifactCacheKeyRoot(keyRoot string) string {
	keyRoot = strings.TrimSpace(keyRoot)
	if keyRoot == "" {
		return defaultMountedArtifactCacheKeyRoot
	}
	clean := filepath.Clean(filepath.FromSlash(keyRoot))
	if filepath.IsAbs(clean) ||
		clean == "." ||
		clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return defaultMountedArtifactCacheKeyRoot
	}
	return clean
}

func (s *Gopher) tryReuseMountedArtifactCache(ctx context.Context, identity ArtifactIdentity, destPath string) (bool, string, error) {
	config := s.modelArtifactCache.normalized()
	if !config.enabled() {
		return false, "", nil
	}
	cacheKey, ok := mountedArtifactCacheKeyFromHuggingFaceIdentity(identity)
	if !ok {
		return false, "", nil
	}
	if strings.TrimSpace(destPath) == "" {
		return false, "", fmt.Errorf("destination path is empty")
	}

	var lastErr error
	for _, mount := range config.Mounts {
		mount = strings.TrimSpace(mount)
		if mount == "" {
			continue
		}
		cachePath := filepath.Join(mount, config.KeyRoot, cacheKey)
		manifest, ready, err := inspectMountedArtifactCache(ctx, cachePath)
		if err != nil {
			if isMountedArtifactCacheContextError(err) {
				return false, "", err
			}
			lastErr = err
			s.logger.Warnf("Mounted artifact cache source %s is not usable: %v", cachePath, err)
			continue
		}
		if !ready {
			continue
		}
		if err := copyMountedArtifactCacheToDestination(ctx, cachePath, destPath, manifest); err != nil {
			if isMountedArtifactCacheContextError(err) {
				return false, "", err
			}
			if errors.Is(err, errMountedArtifactCacheDestinationExists) {
				s.logger.Infof("Skipping mounted artifact cache source %s because destination path %s already exists", cachePath, destPath)
				return false, "", nil
			}
			lastErr = err
			s.logger.Warnf("Failed to copy mounted artifact cache source %s to %s: %v", cachePath, destPath, err)
			continue
		}
		return true, cachePath, nil
	}

	if config.SourceRequired && lastErr != nil {
		return false, "", lastErr
	}
	return false, "", nil
}

func isMountedArtifactCacheContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func inspectMountedArtifactCache(ctx context.Context, cachePath string) (mountedArtifactCacheManifest, bool, error) {
	if err := ctx.Err(); err != nil {
		return mountedArtifactCacheManifest{}, false, err
	}
	readyPath := filepath.Join(cachePath, mountedArtifactCacheReadyMarkerFile)
	readyInfo, err := os.Lstat(readyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return mountedArtifactCacheManifest{}, false, nil
		}
		return mountedArtifactCacheManifest{}, false, err
	}
	if !readyInfo.Mode().IsRegular() {
		return mountedArtifactCacheManifest{}, false, fmt.Errorf("%s is not a regular file", readyPath)
	}

	manifest, err := readMountedArtifactCacheManifest(cachePath)
	if err != nil {
		return mountedArtifactCacheManifest{}, false, err
	}
	if len(manifest.Files) == 0 {
		return mountedArtifactCacheManifest{}, false, fmt.Errorf("%s does not list any files", filepath.Join(cachePath, mountedArtifactCacheManifestFile))
	}

	seenFiles := map[string]struct{}{}
	for _, file := range manifest.Files {
		if err := ctx.Err(); err != nil {
			return mountedArtifactCacheManifest{}, false, err
		}
		relativeName, err := file.safeRelativeName()
		if err != nil {
			return mountedArtifactCacheManifest{}, false, err
		}
		if _, ok := seenFiles[relativeName]; ok {
			return mountedArtifactCacheManifest{}, false, fmt.Errorf("manifest file entry %q is listed more than once", relativeName)
		}
		seenFiles[relativeName] = struct{}{}
		sourcePath := filepath.Join(cachePath, relativeName)
		info, err := os.Stat(sourcePath)
		if err != nil {
			return mountedArtifactCacheManifest{}, false, err
		}
		if !info.Mode().IsRegular() {
			return mountedArtifactCacheManifest{}, false, fmt.Errorf("%s is not a regular file", sourcePath)
		}
		if info.Size() != file.Size {
			return mountedArtifactCacheManifest{}, false, fmt.Errorf("%s size mismatch: expected %d, got %d", sourcePath, file.Size, info.Size())
		}
	}

	return manifest, true, nil
}

func readMountedArtifactCacheManifest(cachePath string) (mountedArtifactCacheManifest, error) {
	manifestPath := filepath.Join(cachePath, mountedArtifactCacheManifestFile)
	info, err := os.Lstat(manifestPath)
	if err != nil {
		return mountedArtifactCacheManifest{}, err
	}
	if !info.Mode().IsRegular() {
		return mountedArtifactCacheManifest{}, fmt.Errorf("%s is not a regular file", manifestPath)
	}
	if info.Size() > maxMountedArtifactCacheManifestBytes {
		return mountedArtifactCacheManifest{}, fmt.Errorf("%s is too large: %d bytes exceeds %d bytes", manifestPath, info.Size(), maxMountedArtifactCacheManifestBytes)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return mountedArtifactCacheManifest{}, err
	}
	var manifest mountedArtifactCacheManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return mountedArtifactCacheManifest{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	return manifest, nil
}

func copyMountedArtifactCacheToDestination(ctx context.Context, cachePath string, destPath string, manifest mountedArtifactCacheManifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(destPath); err == nil {
		return fmt.Errorf("%w: %s", errMountedArtifactCacheDestinationExists, destPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	destParent := filepath.Dir(destPath)
	if err := os.MkdirAll(destParent, 0755); err != nil {
		return err
	}
	stagingPath, err := os.MkdirTemp(destParent, filepath.Base(destPath)+".artifact-cache-staging-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(stagingPath)
	}()

	for _, file := range manifest.Files {
		relativeName, err := file.safeRelativeName()
		if err != nil {
			return err
		}
		sourcePath := filepath.Join(cachePath, relativeName)
		targetPath := filepath.Join(stagingPath, relativeName)
		if err := copyMountedArtifactCacheFile(ctx, sourcePath, targetPath, file.Size); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(stagingPath, destPath); err != nil {
		return err
	}
	return nil
}

func copyMountedArtifactCacheFile(ctx context.Context, sourcePath string, targetPath string, expectedSize int64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", sourcePath)
	}
	if sourceInfo.Size() != expectedSize {
		return fmt.Errorf("%s size mismatch before copy: expected %d, got %d", sourcePath, expectedSize, sourceInfo.Size())
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := copyWithContext(ctx, target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	if targetInfo.Size() != expectedSize {
		return fmt.Errorf("%s size mismatch after copy: expected %d, got %d", targetPath, expectedSize, targetInfo.Size())
	}
	return nil
}

func copyWithContext(ctx context.Context, writer io.Writer, reader io.Reader) (int64, error) {
	buffer := make([]byte, 1024*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, readErr := reader.Read(buffer)
		if nr > 0 {
			nw, writeErr := writer.Write(buffer[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func (f mountedArtifactCacheManifestFileEntry) safeRelativeName() (string, error) {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		name = strings.TrimSpace(f.Path)
	}
	if name == "" {
		return "", fmt.Errorf("manifest file entry has empty name")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("manifest file entry %q is absolute", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("manifest file entry %q escapes artifact root", name)
	}
	if f.Size < 0 {
		return "", fmt.Errorf("manifest file entry %q has negative size %d", name, f.Size)
	}
	return clean, nil
}
