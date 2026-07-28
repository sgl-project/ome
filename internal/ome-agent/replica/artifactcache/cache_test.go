package artifactcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"
)

const testCommitSHA = "cdbee75f17c01a7cc42f958dc650907174af0554"
const testFileSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"

func TestConfigEntryPathUsesImmutableHuggingFaceIdentity(t *testing.T) {
	root := t.TempDir()
	config := Config{
		Enabled:   true,
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: testCommitSHA,
	}

	path, err := config.EntryPath(root)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "_artifacts", "Qwen", "Qwen3-4B-Instruct-2507", testCommitSHA), path)
}

func TestConfigEntryPathRejectsUnsafeCacheNamespace(t *testing.T) {
	base := Config{
		Enabled:   true,
		Mode:      "seed",
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: testCommitSHA,
	}

	rootConfig := base
	rootConfig.Root = "/"
	_, err := rootConfig.EntryPath(rootConfig.Root)
	require.ErrorContains(t, err, "must not be the filesystem root")

	reservedConfig := base
	reservedConfig.Root = t.TempDir()
	reservedConfig.KeyRoot = ".staging"
	_, err = reservedConfig.EntryPath(reservedConfig.Root)
	require.ErrorContains(t, err, "reserved cache directory")
}

func TestPublishStagingCreatesValidImmutableEntry(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(staging, "weights"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "config.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "weights", "model.safetensors"), []byte("weights"), 0o644))

	entry, manifest, reused, err := config.PublishStaging(staging)

	require.NoError(t, err)
	require.False(t, reused)
	require.Len(t, manifest.Files, 2)
	require.FileExists(t, filepath.Join(entry, ReadyFileName))
	require.FileExists(t, filepath.Join(entry, ManifestFileName))
	inspected, hit, err := config.Inspect(config.Root)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, manifest, inspected)
}

func TestInspectAvoidsFullFileChecksumAndVerifyRejectsSameSizeArtifactCorruption(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))

	entry, manifest, _, err := config.PublishStaging(staging)
	require.NoError(t, err)
	require.Len(t, manifest.Files, 1)
	require.Len(t, manifest.Files[0].SHA256, 64)
	require.NoError(t, os.WriteFile(filepath.Join(entry, "model.safetensors"), []byte("WEIGHTS"), 0o644))

	_, hit, err := config.Inspect(root)

	require.NoError(t, err)
	require.True(t, hit)

	_, hit, err = config.Verify(root)

	require.ErrorContains(t, err, "checksum mismatch")
	require.False(t, hit)
}

func TestInspectRejectsManifestPathTraversal(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	entry, err := config.EntryPath(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(entry, 0o755))
	manifest := Manifest{Files: []File{{Name: "../outside", Size: 1, SHA256: testFileSHA256}}}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entry, ManifestFileName), data, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(entry, ReadyFileName), nil, 0o644))

	_, hit, err := config.Inspect(root)

	require.ErrorContains(t, err, "unsafe artifact path")
	require.False(t, hit)
}

func TestInspectRejectsSymlinkedReadyMarker(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	entry, _, _, err := config.PublishStaging(staging)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(entry, ReadyFileName)))
	outsideReady := filepath.Join(t.TempDir(), "ready")
	require.NoError(t, os.WriteFile(outsideReady, nil, 0o644))
	require.NoError(t, os.Symlink(outsideReady, filepath.Join(entry, ReadyFileName)))

	_, hit, err := config.Inspect(root)

	require.ErrorContains(t, err, "ready marker is not a regular file")
	require.False(t, hit)
}

func TestInspectRejectsSymlinkedIdentityDirectory(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	outsideKeyRoot := t.TempDir()
	outsideEntry := filepath.Join(
		outsideKeyRoot,
		filepath.FromSlash(config.HFModelID),
		config.CommitSHA,
	)
	require.NoError(t, os.MkdirAll(outsideEntry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outsideEntry, "model.safetensors"), []byte("weights"), 0o644))
	manifest := Manifest{Files: []File{{Name: "model.safetensors", Size: 7, SHA256: testFileSHA256}}}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(outsideEntry, ManifestFileName), data, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outsideEntry, ReadyFileName), nil, 0o644))
	require.NoError(t, os.Symlink(outsideKeyRoot, filepath.Join(root, config.KeyRoot)))

	_, hit, err := config.Inspect(root)

	require.ErrorContains(t, err, "symbolic link")
	require.False(t, hit)
}

func TestFanOutCopiesCompleteEntryAndReusesTargetHit(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceConfig := testConfig(sourceRoot)
	staging, err := sourceConfig.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	_, _, _, err = sourceConfig.PublishStaging(staging)
	require.NoError(t, err)

	targetConfig := testConfig(targetRoot)
	targetConfig.SourceRoot = sourceRoot
	entry, reused, err := targetConfig.FanOut()

	require.NoError(t, err)
	require.False(t, reused)
	require.FileExists(t, filepath.Join(entry, "model.safetensors"))

	entry, reused, err = targetConfig.FanOut()
	require.NoError(t, err)
	require.True(t, reused)
	require.FileExists(t, filepath.Join(entry, "model.safetensors"))
}

func TestFanOutClassifiesCorruptSourceAsUnavailable(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceConfig := testConfig(sourceRoot)
	staging, err := sourceConfig.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	sourceEntry, _, _, err := sourceConfig.PublishStaging(staging)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(sourceEntry, "model.safetensors"), []byte("WEIGHTS"), 0o644))

	targetConfig := testConfig(targetRoot)
	targetConfig.SourceRoot = sourceRoot
	_, _, err = targetConfig.FanOut()

	require.ErrorIs(t, err, ErrFanOutSourceUnavailable)
	require.ErrorContains(t, err, FanOutSourceUnavailableMessage)
}

func TestNewUploadViewExcludesCacheMetadata(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	entry, manifest, _, err := config.PublishStaging(staging)
	require.NoError(t, err)

	view, cleanup, err := config.NewUploadView(entry, manifest, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })

	require.FileExists(t, filepath.Join(view, "model.safetensors"))
	require.NoFileExists(t, filepath.Join(view, ManifestFileName))
	require.NoFileExists(t, filepath.Join(view, ReadyFileName))
	require.NoError(t, cleanup())
	_, err = os.Stat(view)
	require.True(t, os.IsNotExist(err))
}

func TestNewUploadViewVerifiesReusedCacheBeforeUpload(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	entry, manifest, _, err := config.PublishStaging(staging)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entry, "model.safetensors"), []byte("WEIGHTS"), 0o644))

	_, _, err = config.NewUploadView(entry, manifest, true)

	require.ErrorContains(t, err, "verify upload view source file")
	require.ErrorContains(t, err, "expected")
}

func TestEntryLockReclaimsStaleAbandonedStagingAndPreservesFreshStaging(t *testing.T) {
	originalNow := scratchNowFunc
	t.Cleanup(func() { scratchNowFunc = originalNow })
	currentTime := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	config := testConfig(t.TempDir())

	scratchNowFunc = func() time.Time { return currentTime.Add(-48 * time.Hour) }
	staleStaging, err := config.NewStagingDir()
	require.NoError(t, err)
	scratchNowFunc = func() time.Time { return currentTime }
	freshStaging, err := config.NewStagingDir()
	require.NoError(t, err)

	require.NoError(t, config.WithEntryLock(func() error { return nil }))

	require.NoDirExists(t, staleStaging)
	require.DirExists(t, freshStaging)
}

func TestEntryLockDoesNotReclaimActiveUploadView(t *testing.T) {
	originalNow := scratchNowFunc
	t.Cleanup(func() { scratchNowFunc = originalNow })
	currentTime := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	config := testConfig(t.TempDir())
	staging, err := config.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	entry, manifest, _, err := config.PublishStaging(staging)
	require.NoError(t, err)

	scratchNowFunc = func() time.Time { return currentTime.Add(-48 * time.Hour) }
	view, cleanup, err := config.NewUploadView(entry, manifest, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })
	scratchNowFunc = func() time.Time { return currentTime }

	require.NoError(t, config.WithEntryLock(func() error { return nil }))

	require.DirExists(t, view)
}

func TestCleanupScratchRejectsCacheRoot(t *testing.T) {
	root := t.TempDir()
	config := testConfig(root)

	err := config.CleanupScratch(root)

	require.ErrorContains(t, err, "not an artifact cache scratch path")
	require.DirExists(t, root)
}

func TestEntryLockIgnoresScratchMetadataThatTargetsParentDirectory(t *testing.T) {
	originalNow := scratchNowFunc
	t.Cleanup(func() { scratchNowFunc = originalNow })
	currentTime := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	scratchNowFunc = func() time.Time { return currentTime }
	config := testConfig(t.TempDir())
	sentinel := filepath.Join(config.Root, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o644))
	metadataRoot := filepath.Join(config.Root, scratchMetadataDirName)
	require.NoError(t, os.MkdirAll(metadataRoot, 0o755))
	metadata := scratchMetadata{
		Kind:      scratchKindStaging,
		Name:      "..",
		LockName:  config.entryLockName(),
		CreatedAt: currentTime.Add(-48 * time.Hour),
	}
	data, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(metadataRoot, "malicious.json"), data, 0o600))

	require.NoError(t, config.WithEntryLock(func() error { return nil }))

	require.FileExists(t, sentinel)
}

func TestEntryLockIgnoresScratchMetadataWithUnsafeLockName(t *testing.T) {
	originalNow := scratchNowFunc
	t.Cleanup(func() { scratchNowFunc = originalNow })
	currentTime := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	scratchNowFunc = func() time.Time { return currentTime }
	config := testConfig(t.TempDir())
	metadataRoot := filepath.Join(config.Root, scratchMetadataDirName)
	require.NoError(t, os.MkdirAll(metadataRoot, 0o755))
	metadata := scratchMetadata{
		Kind:      scratchKindStaging,
		Name:      "artifact-malicious",
		LockName:  "../../outside.lock",
		CreatedAt: currentTime.Add(-48 * time.Hour),
	}
	data, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(metadataRoot, "malicious-lock.json"), data, 0o600))
	outsideLock := filepath.Join(filepath.Dir(config.Root), "outside.lock")

	require.NoError(t, config.WithEntryLock(func() error { return nil }))

	require.NoFileExists(t, outsideLock)
}

func TestRemoveInvalidEntryRepairsCorruptPublishedEntry(t *testing.T) {
	root := t.TempDir()
	config := Config{
		Enabled:   true,
		Mode:      "seed",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: testCommitSHA,
	}
	entry, err := config.EntryPath(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(entry, 0o755))
	manifestData, err := json.Marshal(Manifest{Files: []File{{
		Name:   "model.safetensors",
		Size:   7,
		SHA256: testFileSHA256,
	}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entry, ManifestFileName), manifestData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(entry, ReadyFileName), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(entry, "model.safetensors"), []byte("bad"), 0o644))

	_, hit, inspectErr := config.Inspect(root)
	require.ErrorContains(t, inspectErr, "size mismatch")
	require.False(t, hit)

	removed, err := config.RemoveInvalidEntry()

	require.NoError(t, err)
	require.True(t, removed)
	require.NoDirExists(t, entry)
}

func TestCopyManifestFilesRejectsSourceSizeMismatch(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "model.safetensors"), []byte("short"), 0o644))

	err := copyManifestFiles(source, target, Manifest{Files: []File{{
		Name:   "model.safetensors",
		Size:   100,
		SHA256: testFileSHA256,
	}}})

	require.ErrorContains(t, err, "size mismatch")
	require.NoFileExists(t, filepath.Join(target, "model.safetensors"))
}

func TestCopyAndHashClassifiesReadFailureAsUnavailableSource(t *testing.T) {
	_, _, err := copyAndHash(iotest.ErrReader(errors.New("nfs read failed")), &discardWriter{})

	require.ErrorIs(t, err, ErrFanOutSourceUnavailable)
	require.ErrorContains(t, err, "nfs read failed")
}

func TestCopyAndHashDoesNotClassifyWriteFailureAsUnavailableSource(t *testing.T) {
	writeErr := errors.New("target disk full")
	_, _, err := copyAndHash(strings.NewReader("weights"), errWriter{err: writeErr})

	require.ErrorIs(t, err, writeErr)
	require.NotErrorIs(t, err, ErrFanOutSourceUnavailable)
}

type discardWriter struct{}

func (*discardWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func testConfig(root string) Config {
	return Config{
		Enabled:   true,
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: testCommitSHA,
	}
}
