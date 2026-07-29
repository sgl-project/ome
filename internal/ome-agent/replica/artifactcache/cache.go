package artifactcache

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	ManifestFileName               = "manifest.json"
	ReadyFileName                  = "_READY"
	ModeSeed                       = "seed"
	ModeFanOut                     = "fanout"
	ModeRepair                     = "repair"
	FanOutSourceUnavailableMessage = "model artifact cache source unavailable"
	maxManifestSize                = 64 << 20
	copyBufferSize                 = 4 << 20
	scratchStaleAge                = 24 * time.Hour

	scratchMetadataDirName = ".scratch-metadata"
	scratchLockDirName     = ".scratch-locks"
	entryLockDirName       = ".locks"
	stagingDirName         = ".staging"
	uploadsDirName         = ".uploads"
	scratchKindStaging     = "staging"
	scratchKindUpload      = "upload"
)

var fullCommitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var sha256Checksum = regexp.MustCompile(`^[0-9a-f]{64}$`)
var entryLockFileName = regexp.MustCompile(`^[0-9a-f]{32}\.lock$`)
var scratchNowFunc = time.Now
var ErrFanOutSourceUnavailable = errors.New(FanOutSourceUnavailableMessage)

type Config struct {
	Enabled    bool   `mapstructure:"enabled"`
	Mode       string `mapstructure:"mode"`
	Root       string `mapstructure:"root"`
	KeyRoot    string `mapstructure:"key_root"`
	HFModelID  string `mapstructure:"hf_model_id"`
	CommitSHA  string `mapstructure:"hf_commit_sha"`
	SourceRoot string `mapstructure:"source_root"`
}

type Manifest struct {
	Files []File `json:"files"`
}

type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type scratchMetadata struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	LockName  string    `json:"lockName"`
	CreatedAt time.Time `json:"createdAt"`
}

func (c Config) EntryPath(root string) (string, error) {
	if err := c.validate(root); err != nil {
		return "", err
	}
	entry := filepath.Join(root, filepath.FromSlash(c.KeyRoot), filepath.FromSlash(c.HFModelID), c.CommitSHA)
	if err := requirePathWithin(root, entry); err != nil {
		return "", err
	}
	return entry, nil
}

func (c Config) NewStagingDir() (string, error) {
	if _, err := c.EntryPath(c.Root); err != nil {
		return "", err
	}
	stagingRoot := filepath.Join(c.Root, stagingDirName)
	if err := rejectSymlinkComponents(c.Root, stagingRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return "", fmt.Errorf("create artifact cache staging root: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, stagingRoot); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(stagingRoot, "artifact-")
	if err != nil {
		return "", err
	}
	if err := c.writeScratchMetadata(scratchMetadata{
		Kind:      scratchKindStaging,
		Name:      filepath.Base(staging),
		LockName:  c.entryLockName(),
		CreatedAt: scratchNowFunc().UTC(),
	}); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}

func (c Config) WithEntryLock(operation func() error) error {
	if operation == nil {
		return fmt.Errorf("model artifact cache lock operation is nil")
	}
	if _, err := c.EntryPath(c.Root); err != nil {
		return err
	}
	lockRoot := filepath.Join(c.Root, entryLockDirName)
	if err := rejectSymlinkComponents(c.Root, lockRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		return fmt.Errorf("create artifact cache lock root: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, lockRoot); err != nil {
		return err
	}
	lockName := c.entryLockName()
	lockPath := filepath.Join(lockRoot, lockName)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open artifact cache lock: %w", err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("acquire artifact cache lock: %w", err)
	}
	defer func() {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
	}()
	if err := c.cleanupStaleScratch(lockName); err != nil {
		return fmt.Errorf("clean stale model artifact cache scratch: %w", err)
	}
	return operation()
}

func (c Config) Inspect(root string) (Manifest, bool, error) {
	entry, err := c.EntryPath(root)
	if err != nil {
		return Manifest{}, false, err
	}
	if err := rejectSymlinkComponents(root, entry); err != nil {
		return Manifest{}, false, err
	}
	return inspectEntry(entry)
}

// EnsureEntryReadable grants read and traversal access needed by read-only
// cache clients without granting them write access.
func (c Config) EnsureEntryReadable(root string) error {
	entry, err := c.EntryPath(root)
	if err != nil {
		return err
	}
	if err := rejectSymlinkComponents(root, entry); err != nil {
		return err
	}
	if err := makeArtifactTreeReadable(entry); err != nil {
		return fmt.Errorf("make artifact cache entry readable: %w", err)
	}
	return nil
}

// Verify performs the structural inspection and then validates every file
// checksum. Use this only when a full read is required; normal cache discovery
// intentionally remains metadata-only.
func (c Config) Verify(root string) (Manifest, bool, error) {
	entry, err := c.EntryPath(root)
	if err != nil {
		return Manifest{}, false, err
	}
	if err := rejectSymlinkComponents(root, entry); err != nil {
		return Manifest{}, false, err
	}
	return verifyEntry(entry)
}

// RemoveInvalidEntry removes an incomplete or corrupt entry so a caller that
// holds the entry lock can rebuild it. Valid entries are never removed.
func (c Config) RemoveInvalidEntry() (bool, error) {
	entry, err := c.EntryPath(c.Root)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(entry)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect invalid artifact cache entry: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("artifact cache entry is not a directory: %s", entry)
	}
	if err := rejectSymlinkComponents(c.Root, entry); err != nil {
		return false, err
	}
	_, hit, _ := verifyEntry(entry)
	if hit {
		return false, nil
	}
	if err := os.RemoveAll(entry); err != nil {
		return false, fmt.Errorf("remove invalid artifact cache entry: %w", err)
	}
	return true, nil
}

func (c Config) PublishStaging(staging string) (string, Manifest, bool, error) {
	entry, err := c.EntryPath(c.Root)
	if err != nil {
		return "", Manifest{}, false, err
	}
	if err := requireStrictPathWithin(filepath.Join(c.Root, stagingDirName), staging); err != nil {
		return "", Manifest{}, false, fmt.Errorf("invalid artifact staging path: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, staging); err != nil {
		return "", Manifest{}, false, err
	}
	if err := rejectSymlinkComponents(c.Root, entry); err != nil {
		return "", Manifest{}, false, err
	}

	existing, hit, err := verifyEntry(entry)
	if err != nil {
		return "", Manifest{}, false, err
	}
	if hit {
		if err := c.EnsureEntryReadable(c.Root); err != nil {
			return "", Manifest{}, false, err
		}
		_ = c.CleanupScratch(staging)
		return entry, existing, true, nil
	}

	manifest, err := buildManifest(staging)
	if err != nil {
		return "", Manifest{}, false, err
	}
	if err := writeManifestAndReady(staging, manifest); err != nil {
		return "", Manifest{}, false, err
	}
	if err := makeArtifactTreeReadable(staging); err != nil {
		return "", Manifest{}, false, fmt.Errorf("make artifact cache entry readable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		return "", Manifest{}, false, fmt.Errorf("create artifact cache identity directory: %w", err)
	}
	if err := os.Rename(staging, entry); err != nil {
		existing, hit, inspectErr := verifyEntry(entry)
		if inspectErr == nil && hit {
			if readableErr := c.EnsureEntryReadable(c.Root); readableErr != nil {
				return "", Manifest{}, false, readableErr
			}
			_ = c.CleanupScratch(staging)
			return entry, existing, true, nil
		}
		return "", Manifest{}, false, fmt.Errorf("publish artifact cache entry: %w", err)
	}
	_ = c.removeScratchMetadata(filepath.Base(staging))
	return entry, manifest, false, nil
}

func (c Config) FanOut() (string, bool, error) {
	var entry string
	var reused bool
	err := c.WithEntryLock(func() error {
		var fanOutErr error
		entry, reused, fanOutErr = c.fanOutLocked()
		return fanOutErr
	})
	return entry, reused, err
}

func (c Config) fanOutLocked() (string, bool, error) {
	if c.SourceRoot == "" {
		return "", false, fmt.Errorf("model artifact cache source_root is required for fanout")
	}
	sourceEntry, err := c.EntryPath(c.SourceRoot)
	if err != nil {
		return "", false, err
	}
	sourceManifest, hit, err := inspectEntry(sourceEntry)
	if err != nil {
		return "", false, fmt.Errorf("%w: inspect source artifact cache entry: %v", ErrFanOutSourceUnavailable, err)
	}
	if !hit {
		return "", false, fmt.Errorf("%w: source artifact cache entry is not ready: %s", ErrFanOutSourceUnavailable, sourceEntry)
	}

	targetEntry, err := c.EntryPath(c.Root)
	if err != nil {
		return "", false, err
	}
	_, hit, inspectErr := verifyEntry(targetEntry)
	if hit {
		if err := c.EnsureEntryReadable(c.Root); err != nil {
			return "", false, err
		}
		return targetEntry, true, nil
	}
	if _, err := c.RemoveInvalidEntry(); err != nil {
		if inspectErr != nil {
			return "", false, fmt.Errorf("repair target artifact cache entry after inspection failed (%v): %w", inspectErr, err)
		}
		return "", false, fmt.Errorf("repair incomplete target artifact cache entry: %w", err)
	}

	staging, err := c.NewStagingDir()
	if err != nil {
		return "", false, err
	}
	defer c.CleanupScratch(staging)
	if err := copyManifestFiles(sourceEntry, staging, sourceManifest); err != nil {
		return "", false, err
	}
	entry, _, reused, err := c.PublishStaging(staging)
	return entry, reused, err
}

func (c Config) NewUploadView(entry string, manifest Manifest, verifyChecksums bool) (string, func() error, error) {
	expectedEntry, err := c.EntryPath(c.Root)
	if err != nil {
		return "", nil, err
	}
	if filepath.Clean(entry) != expectedEntry {
		return "", nil, fmt.Errorf("artifact entry %q does not match configured identity path %q", entry, expectedEntry)
	}
	if err := rejectSymlinkComponents(c.Root, entry); err != nil {
		return "", nil, err
	}
	uploadsRoot := filepath.Join(c.Root, uploadsDirName)
	if err := rejectSymlinkComponents(c.Root, uploadsRoot); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(uploadsRoot, 0o755); err != nil {
		return "", nil, fmt.Errorf("create artifact upload view root: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, uploadsRoot); err != nil {
		return "", nil, err
	}
	view, err := os.MkdirTemp(uploadsRoot, "artifact-")
	if err != nil {
		return "", nil, fmt.Errorf("create artifact upload view: %w", err)
	}
	lockRoot := filepath.Join(c.Root, scratchLockDirName)
	if err := rejectSymlinkComponents(c.Root, lockRoot); err != nil {
		_ = os.RemoveAll(view)
		return "", nil, err
	}
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		_ = os.RemoveAll(view)
		return "", nil, fmt.Errorf("create artifact scratch lock root: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, lockRoot); err != nil {
		_ = os.RemoveAll(view)
		return "", nil, err
	}
	lockName := filepath.Base(view) + ".lock"
	lockPath := filepath.Join(lockRoot, lockName)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		_ = os.RemoveAll(view)
		return "", nil, fmt.Errorf("open artifact upload view lock: %w", err)
	}
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		lockFile.Close()
		_ = os.RemoveAll(view)
		return "", nil, fmt.Errorf("acquire artifact upload view lock: %w", err)
	}
	if err := c.writeScratchMetadata(scratchMetadata{
		Kind:      scratchKindUpload,
		Name:      filepath.Base(view),
		LockName:  lockName,
		CreatedAt: scratchNowFunc().UTC(),
	}); err != nil {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		lockFile.Close()
		_ = os.RemoveAll(view)
		_ = os.Remove(lockPath)
		return "", nil, err
	}
	var cleanupOnce sync.Once
	var cleanupErr error
	cleanup := func() error {
		cleanupOnce.Do(func() {
			removeErr := os.RemoveAll(view)
			metadataErr := c.removeScratchMetadata(filepath.Base(view))
			_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
			closeErr := lockFile.Close()
			lockErr := os.Remove(lockPath)
			for _, err := range []error{removeErr, metadataErr, closeErr, lockErr} {
				if err != nil && !os.IsNotExist(err) {
					cleanupErr = err
					return
				}
			}
		})
		return cleanupErr
	}
	for _, file := range manifest.Files {
		if err := validateArtifactFile(file); err != nil {
			_ = cleanup()
			return "", nil, err
		}
		source := filepath.Join(entry, filepath.FromSlash(file.Name))
		target := filepath.Join(view, filepath.FromSlash(file.Name))
		if err := validateRegularFileSize(source, file.Size); err != nil {
			_ = cleanup()
			return "", nil, err
		}
		if verifyChecksums {
			if err := verifyRegularFileChecksum(source, file.SHA256); err != nil {
				_ = cleanup()
				return "", nil, fmt.Errorf("verify upload view source file %q: %w", file.Name, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = cleanup()
			return "", nil, fmt.Errorf("create upload view directory: %w", err)
		}
		if err := os.Link(source, target); err != nil {
			if copyErr := copyRegularFile(source, target, file.Size, file.SHA256); copyErr != nil {
				_ = cleanup()
				return "", nil, fmt.Errorf("materialize upload view file %q: hard link: %v; copy: %w", file.Name, err, copyErr)
			}
		}
	}
	return view, cleanup, nil
}

func (c Config) CleanupScratch(path string) error {
	allowed := false
	for _, scratchRoot := range []string{
		filepath.Join(c.Root, stagingDirName),
		filepath.Join(c.Root, uploadsDirName),
	} {
		if requireStrictPathWithin(scratchRoot, path) == nil {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("path %q is not an artifact cache scratch path", path)
	}
	removeErr := os.RemoveAll(path)
	metadataErr := c.removeScratchMetadata(filepath.Base(path))
	if removeErr != nil {
		return removeErr
	}
	return metadataErr
}

func (c Config) entryLockName() string {
	sum := sha256.Sum256([]byte(c.HFModelID + "@" + c.CommitSHA))
	return fmt.Sprintf("%x.lock", sum[:16])
}

func (c Config) writeScratchMetadata(metadata scratchMetadata) error {
	metadataRoot := filepath.Join(c.Root, scratchMetadataDirName)
	if err := rejectSymlinkComponents(c.Root, metadataRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(metadataRoot, 0o755); err != nil {
		return fmt.Errorf("create artifact scratch metadata root: %w", err)
	}
	if err := rejectSymlinkComponents(c.Root, metadataRoot); err != nil {
		return err
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode artifact scratch metadata: %w", err)
	}
	path := filepath.Join(metadataRoot, metadata.Name+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write artifact scratch metadata: %w", err)
	}
	return nil
}

func (c Config) removeScratchMetadata(name string) error {
	if err := rejectSymlinkComponents(c.Root, filepath.Join(c.Root, scratchMetadataDirName)); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(c.Root, scratchMetadataDirName, name+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c Config) cleanupStaleScratch(currentEntryLockName string) error {
	metadataRoot := filepath.Join(c.Root, scratchMetadataDirName)
	if err := rejectSymlinkComponents(c.Root, metadataRoot); err != nil {
		return err
	}
	entries, err := os.ReadDir(metadataRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		metadataPath := filepath.Join(metadataRoot, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}
		var metadata scratchMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}
		if !validScratchName(metadata.Name) ||
			scratchNowFunc().Sub(metadata.CreatedAt) < scratchStaleAge {
			continue
		}
		var scratchRoot string
		var lockRoot string
		var lockPath string
		switch metadata.Kind {
		case scratchKindStaging:
			scratchRoot = filepath.Join(c.Root, stagingDirName)
			lockRoot = filepath.Join(c.Root, entryLockDirName)
			if !entryLockFileName.MatchString(metadata.LockName) {
				continue
			}
			lockPath = filepath.Join(lockRoot, metadata.LockName)
			if metadata.LockName == currentEntryLockName {
				scratchPath := filepath.Join(scratchRoot, metadata.Name)
				if requireStrictPathWithin(scratchRoot, scratchPath) != nil ||
					rejectSymlinkComponents(c.Root, scratchPath) != nil {
					continue
				}
				_ = os.RemoveAll(scratchPath)
				_ = os.Remove(metadataPath)
				continue
			}
		case scratchKindUpload:
			scratchRoot = filepath.Join(c.Root, uploadsDirName)
			lockRoot = filepath.Join(c.Root, scratchLockDirName)
			if metadata.LockName != metadata.Name+".lock" {
				continue
			}
			lockPath = filepath.Join(lockRoot, metadata.LockName)
		default:
			continue
		}
		scratchPath := filepath.Join(scratchRoot, metadata.Name)
		if requireStrictPathWithin(scratchRoot, scratchPath) != nil ||
			requireStrictPathWithin(lockRoot, lockPath) != nil ||
			rejectSymlinkComponents(c.Root, scratchPath) != nil ||
			rejectSymlinkComponents(c.Root, lockPath) != nil {
			continue
		}
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			continue
		}
		lockErr := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if lockErr == unix.EWOULDBLOCK || lockErr == unix.EAGAIN {
			lockFile.Close()
			continue
		}
		if lockErr != nil {
			lockFile.Close()
			continue
		}
		_ = os.RemoveAll(scratchPath)
		_ = os.Remove(metadataPath)
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		lockFile.Close()
		if metadata.Kind == scratchKindUpload {
			_ = os.Remove(lockPath)
		}
	}
	return nil
}

func validScratchName(name string) bool {
	return name != "" &&
		name != "." &&
		name != ".." &&
		filepath.Base(name) == name &&
		strings.HasPrefix(name, "artifact-") &&
		len(name) > len("artifact-")
}

func (c Config) validate(root string) error {
	if !c.Enabled {
		return fmt.Errorf("model artifact cache is disabled")
	}
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("model artifact cache root must be absolute: %q", root)
	}
	if filepath.Clean(root) != root {
		return fmt.Errorf("model artifact cache root must be a clean absolute path: %q", root)
	}
	if filepath.Dir(root) == root {
		return fmt.Errorf("model artifact cache root must not be the filesystem root: %q", root)
	}
	if err := validateRelativePath("key_root", c.KeyRoot); err != nil {
		return err
	}
	if isReservedCacheDirectory(c.KeyRoot) {
		return fmt.Errorf("key_root must not use a reserved cache directory: %q", c.KeyRoot)
	}
	if err := validateRelativePath("hf_model_id", c.HFModelID); err != nil {
		return err
	}
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	if mode == "" {
		mode = ModeSeed
	}
	if mode != ModeSeed && mode != ModeFanOut && mode != ModeRepair {
		return fmt.Errorf("unsupported model artifact cache mode %q", c.Mode)
	}
	commitSHA := strings.ToLower(strings.TrimSpace(c.CommitSHA))
	if !fullCommitSHA.MatchString(commitSHA) {
		return fmt.Errorf("hf_commit_sha must be a full lowercase commit SHA")
	}
	if commitSHA != c.CommitSHA {
		return fmt.Errorf("hf_commit_sha must be normalized to lowercase")
	}
	return nil
}

func isReservedCacheDirectory(value string) bool {
	topLevel := strings.Split(filepath.ToSlash(value), "/")[0]
	switch topLevel {
	case entryLockDirName, stagingDirName, uploadsDirName, scratchMetadataDirName, scratchLockDirName:
		return true
	default:
		return false
	}
}

func validateRelativePath(name string, value string) error {
	if value == "" || filepath.IsAbs(value) || filepath.Clean(value) != value || value == "." {
		return fmt.Errorf("%s must be a clean relative path: %q", name, value)
	}
	for _, component := range strings.Split(filepath.ToSlash(value), "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("%s contains an unsafe path component: %q", name, value)
		}
	}
	return nil
}

func inspectEntry(entry string) (Manifest, bool, error) {
	readyInfo, err := os.Lstat(filepath.Join(entry, ReadyFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, fmt.Errorf("inspect artifact ready marker: %w", err)
	}
	if !readyInfo.Mode().IsRegular() {
		return Manifest{}, false, fmt.Errorf("artifact ready marker is not a regular file")
	}

	manifestPath := filepath.Join(entry, ManifestFileName)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return Manifest{}, false, fmt.Errorf("inspect artifact manifest: %w", err)
	}
	if !manifestInfo.Mode().IsRegular() {
		return Manifest{}, false, fmt.Errorf("artifact manifest is not a regular file")
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return Manifest{}, false, err
	}
	if len(manifest.Files) == 0 {
		return Manifest{}, false, fmt.Errorf("artifact manifest has no files")
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	for _, file := range manifest.Files {
		if err := validateArtifactFile(file); err != nil {
			return Manifest{}, false, err
		}
		if _, ok := seen[file.Name]; ok {
			return Manifest{}, false, fmt.Errorf("duplicate artifact path in manifest: %q", file.Name)
		}
		seen[file.Name] = struct{}{}
		path := filepath.Join(entry, filepath.FromSlash(file.Name))
		if err := requirePathWithin(entry, path); err != nil {
			return Manifest{}, false, err
		}
		if err := rejectSymlinkComponents(entry, path); err != nil {
			return Manifest{}, false, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return Manifest{}, false, fmt.Errorf("inspect artifact file %q: %w", file.Name, err)
		}
		if !info.Mode().IsRegular() {
			return Manifest{}, false, fmt.Errorf("artifact file is not regular: %q", file.Name)
		}
		if info.Size() != file.Size {
			return Manifest{}, false, fmt.Errorf("artifact file size mismatch for %q: expected %d, got %d", file.Name, file.Size, info.Size())
		}
	}
	return manifest, true, nil
}

func verifyEntry(entry string) (Manifest, bool, error) {
	manifest, hit, err := inspectEntry(entry)
	if err != nil || !hit {
		return manifest, hit, err
	}
	for _, file := range manifest.Files {
		path := filepath.Join(entry, filepath.FromSlash(file.Name))
		if err := verifyRegularFileChecksum(path, file.SHA256); err != nil {
			return Manifest{}, false, fmt.Errorf("artifact file checksum mismatch for %q: %w", file.Name, err)
		}
	}
	return manifest, true, nil
}

func readManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open artifact manifest: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read artifact manifest: %w", err)
	}
	if len(data) > maxManifestSize {
		return Manifest{}, fmt.Errorf("artifact manifest exceeds %d bytes", maxManifestSize)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode artifact manifest: %w", err)
	}
	return manifest, nil
}

func buildManifest(root string) (Manifest, error) {
	manifest := Manifest{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if name == ManifestFileName || name == ReadyFileName {
			return fmt.Errorf("downloaded artifact uses reserved cache metadata path %q", name)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("downloaded artifact contains non-regular file %q", name)
		}
		checksum, err := calculateFileSHA256(path)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, File{Name: name, Size: info.Size(), SHA256: checksum})
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("build artifact manifest: %w", err)
	}
	if len(manifest.Files) == 0 {
		return Manifest{}, fmt.Errorf("downloaded artifact has no files")
	}
	return manifest, nil
}

func writeManifestAndReady(staging string, manifest Manifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode artifact manifest: %w", err)
	}
	tempManifest := filepath.Join(staging, "."+ManifestFileName+".tmp")
	if err := os.WriteFile(tempManifest, data, 0o644); err != nil {
		return fmt.Errorf("write artifact manifest: %w", err)
	}
	if err := os.Rename(tempManifest, filepath.Join(staging, ManifestFileName)); err != nil {
		return fmt.Errorf("publish artifact manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, ReadyFileName), nil, 0o644); err != nil {
		return fmt.Errorf("write artifact ready marker: %w", err)
	}
	return nil
}

func makeArtifactTreeReadable(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		permissions := info.Mode().Perm()
		switch {
		case info.IsDir():
			permissions |= 0o555
		case info.Mode().IsRegular():
			permissions |= 0o444
		default:
			return fmt.Errorf("artifact cache contains unsupported file %q", path)
		}
		if permissions == info.Mode().Perm() {
			return nil
		}
		if err := os.Chmod(path, permissions); err != nil {
			return fmt.Errorf("update permissions for %q: %w", path, err)
		}
		return nil
	})
}

func validateArtifactFile(file File) error {
	if err := validateRelativePath("artifact path", filepath.FromSlash(file.Name)); err != nil {
		return fmt.Errorf("unsafe artifact path %q: %w", file.Name, err)
	}
	if file.Name == ManifestFileName || file.Name == ReadyFileName {
		return fmt.Errorf("artifact manifest references reserved path %q", file.Name)
	}
	if file.Size < 0 {
		return fmt.Errorf("artifact file has negative size: %q", file.Name)
	}
	if !sha256Checksum.MatchString(file.SHA256) {
		return fmt.Errorf("artifact file has invalid SHA-256 checksum: %q", file.Name)
	}
	return nil
}

func requirePathWithin(root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %q escapes root %q", path, root)
	}
	return nil
}

func requireStrictPathWithin(root string, path string) error {
	if err := requirePathWithin(root, path); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." {
		return fmt.Errorf("path %q must be below root %q", path, root)
	}
	return nil
}

func rejectSymlinkComponents(root string, path string) error {
	if err := requirePathWithin(root, path); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
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
			return fmt.Errorf("inspect cache path component %q: %w", component, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cache path component is a symbolic link: %q", component)
		}
	}
	return nil
}

func copyManifestFiles(sourceRoot string, targetRoot string, manifest Manifest) error {
	for _, file := range manifest.Files {
		if err := validateArtifactFile(file); err != nil {
			return err
		}
		source := filepath.Join(sourceRoot, filepath.FromSlash(file.Name))
		target := filepath.Join(targetRoot, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create artifact target directory: %w", err)
		}
		if err := copyRegularFile(source, target, file.Size, file.SHA256); err != nil {
			return fmt.Errorf("copy artifact file %q: %w", file.Name, err)
		}
	}
	return nil
}

func validateRegularFileSize(path string, expectedSize int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect source file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file")
	}
	if info.Size() != expectedSize {
		return fmt.Errorf("source file size mismatch: expected %d, got %d", expectedSize, info.Size())
	}
	return nil
}

func copyRegularFile(source string, target string, expectedSize int64, expectedSHA256 string) error {
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("%w: inspect source file: %v", ErrFanOutSourceUnavailable, err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: source is not a regular file", ErrFanOutSourceUnavailable)
	}
	if sourceInfo.Size() != expectedSize {
		return fmt.Errorf("%w: source file size mismatch: expected %d, got %d", ErrFanOutSourceUnavailable, expectedSize, sourceInfo.Size())
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%w: open source file: %v", ErrFanOutSourceUnavailable, err)
	}
	defer sourceFile.Close()
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create target file: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(target)
		}
	}()
	written, actualSHA256, copyErr := copyAndHash(sourceFile, targetFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != expectedSize {
		return fmt.Errorf("%w: copied file size mismatch: expected %d, got %d", ErrFanOutSourceUnavailable, expectedSize, written)
	}
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("%w: copied file checksum mismatch: expected %s, got %s", ErrFanOutSourceUnavailable, expectedSHA256, actualSHA256)
	}
	complete = true
	return nil
}

func copyAndHash(source io.Reader, target io.Writer) (int64, string, error) {
	hasher := sha256.New()
	buffer := make([]byte, copyBufferSize)
	var written int64
	for {
		readCount, readErr := source.Read(buffer)
		if readCount > 0 {
			if _, err := hasher.Write(buffer[:readCount]); err != nil {
				return written, "", err
			}
			writeCount, writeErr := target.Write(buffer[:readCount])
			written += int64(writeCount)
			if writeErr != nil {
				return written, "", fmt.Errorf("write target file: %w", writeErr)
			}
			if writeCount != readCount {
				return written, "", io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, fmt.Sprintf("%x", hasher.Sum(nil)), nil
			}
			return written, "", fmt.Errorf("%w: read source file: %v", ErrFanOutSourceUnavailable, readErr)
		}
	}
}

func calculateFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.CopyBuffer(hasher, file, make([]byte, copyBufferSize)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func verifyRegularFileChecksum(path string, expectedSHA256 string) error {
	actualSHA256, err := calculateFileSHA256(path)
	if err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("expected %s, got %s", expectedSHA256, actualSHA256)
	}
	return nil
}
