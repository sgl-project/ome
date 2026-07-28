package modelagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func (c ModelArtifactCacheConfig) normalized() ModelArtifactCacheConfig {
	c.KeyRoot = normalizeMountedArtifactCacheKeyRoot(c.KeyRoot)
	mounts := make([]string, 0, len(c.Mounts))
	for _, mount := range c.Mounts {
		mount = strings.TrimSpace(mount)
		if mount != "" && filepath.IsAbs(mount) {
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
		if err := rejectMountedArtifactCacheSymlinkComponents(mount, cachePath); err != nil {
			lastErr = err
			s.logger.Warnf("Mounted artifact cache source %s is not usable: %v", cachePath, err)
			continue
		}
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
		copyConcurrency := mountedArtifactCacheCopyConcurrency(s.concurrency, len(manifest.Files))
		s.logger.Infof("Copying mounted artifact cache source %s to %s with %d file workers", cachePath, destPath, copyConcurrency)
		if err := copyMountedArtifactCacheToDestination(ctx, cachePath, destPath, manifest, copyConcurrency); err != nil {
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
		if err := rejectMountedArtifactCacheSymlinkComponents(cachePath, sourcePath); err != nil {
			return mountedArtifactCacheManifest{}, false, err
		}
		info, err := os.Lstat(sourcePath)
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

func copyMountedArtifactCacheToDestination(
	ctx context.Context,
	cachePath string,
	destPath string,
	manifest mountedArtifactCacheManifest,
	concurrency int,
) error {
	return copyMountedArtifactCacheToDestinationWithOptions(
		ctx,
		cachePath,
		destPath,
		manifest,
		concurrency,
		nil,
		copyMountedArtifactCacheFile,
	)
}

func copyMountedArtifactCacheToDestinationWithHook(ctx context.Context, cachePath string, destPath string, manifest mountedArtifactCacheManifest, beforePublish func()) error {
	return copyMountedArtifactCacheToDestinationWithOptions(
		ctx,
		cachePath,
		destPath,
		manifest,
		1,
		beforePublish,
		copyMountedArtifactCacheFile,
	)
}

type mountedArtifactCacheFileCopier func(
	ctx context.Context,
	cachePath string,
	sourcePath string,
	targetPath string,
	expectedSize int64,
	expectedSHA256 string,
) error

func mountedArtifactCacheCopyConcurrency(configured int, fileCount int) int {
	if fileCount < 1 {
		return 1
	}
	if configured < 1 {
		configured = 1
	}
	if configured > fileCount {
		return fileCount
	}
	return configured
}

func copyMountedArtifactCacheToDestinationWithOptions(
	ctx context.Context,
	cachePath string,
	destPath string,
	manifest mountedArtifactCacheManifest,
	concurrency int,
	beforePublish func(),
	copyFile mountedArtifactCacheFileCopier,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if copyFile == nil {
		return fmt.Errorf("mounted artifact cache file copier is nil")
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

	copyCtx, cancelCopies := context.WithCancel(ctx)
	defer cancelCopies()

	workerCount := mountedArtifactCacheCopyConcurrency(concurrency, len(manifest.Files))
	files := make(chan mountedArtifactCacheManifestFileEntry)
	var workers sync.WaitGroup
	var firstErr error
	var firstErrOnce sync.Once
	recordError := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			cancelCopies()
		})
	}

	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-copyCtx.Done():
					return
				case file, ok := <-files:
					if !ok {
						return
					}
					relativeName, err := file.safeRelativeName()
					if err != nil {
						recordError(err)
						return
					}
					sourcePath := filepath.Join(cachePath, relativeName)
					targetPath := filepath.Join(stagingPath, relativeName)
					if err := copyFile(copyCtx, cachePath, sourcePath, targetPath, file.Size, file.SHA256); err != nil {
						recordError(err)
						return
					}
				}
			}
		}()
	}

enqueue:
	for _, file := range manifest.Files {
		select {
		case <-copyCtx.Done():
			break enqueue
		case files <- file:
		}
	}
	close(files)
	workers.Wait()

	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if beforePublish != nil {
		beforePublish()
	}
	if _, err := os.Lstat(destPath); err == nil {
		return fmt.Errorf("%w: %s", errMountedArtifactCacheDestinationExists, destPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stagingPath, destPath); err != nil {
		return err
	}
	return nil
}

func copyMountedArtifactCacheFile(ctx context.Context, cachePath string, sourcePath string, targetPath string, expectedSize int64, expectedSHA256 string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := rejectMountedArtifactCacheSymlinkComponents(cachePath, sourcePath); err != nil {
		return err
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
	hasher := sha256.New()
	_, copyErr := copyWithContext(ctx, io.MultiWriter(target, hasher), source)
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
	actualSHA256 := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("%s checksum mismatch after copy: expected %s, got %s", targetPath, expectedSHA256, actualSHA256)
	}
	return nil
}

func rejectMountedArtifactCacheSymlinkComponents(root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("mounted artifact cache path %q escapes root %q", path, root)
	}

	current := filepath.Clean(root)
	components := []string{current}
	if relative != "." {
		for _, component := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			components = append(components, current)
		}
	}
	for _, component := range components {
		info, err := os.Lstat(component)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect mounted artifact cache path component %q: %w", component, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("mounted artifact cache path component is a symbolic link: %q", component)
		}
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
	if len(f.SHA256) != sha256.Size*2 {
		return "", fmt.Errorf("manifest file entry %q has invalid SHA-256 checksum", name)
	}
	for _, char := range f.SHA256 {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return "", fmt.Errorf("manifest file entry %q has invalid SHA-256 checksum", name)
		}
	}
	return clean, nil
}
