package modelagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestMountedArtifactCacheKeyFromHuggingFaceIdentity(t *testing.T) {
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
	}

	key, ok := mountedArtifactCacheKeyFromHuggingFaceIdentity(identity)

	require.True(t, ok)
	assert.Equal(t, filepath.Join("openai", "gpt-oss-120b", "abcdef1234567890abcdef1234567890abcdef12"), key)
}

func TestModelArtifactCacheConfigNormalizesKeyRootAndMounts(t *testing.T) {
	config := ModelArtifactCacheConfig{
		Enabled: true,
		Mounts:  []string{" /cache-a ", "", "relative-cache", " /cache-b "},
		KeyRoot: "custom/root",
	}.normalized()

	assert.Equal(t, "custom/root", config.KeyRoot)
	assert.Equal(t, []string{"/cache-a", "/cache-b"}, config.Mounts)
	assert.Equal(t, defaultMountedArtifactCacheKeyRoot, ModelArtifactCacheConfig{KeyRoot: "../outside"}.normalized().KeyRoot)
	assert.Equal(t, defaultMountedArtifactCacheKeyRoot, ModelArtifactCacheConfig{KeyRoot: "/absolute"}.normalized().KeyRoot)
	assert.Equal(t, defaultMountedArtifactCacheKeyRoot, ModelArtifactCacheConfig{}.normalized().KeyRoot)
}

func TestTryReuseMountedArtifactCacheCopiesReadyArtifact(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{
		"config.json":                  `{"model_type":"gpt_oss"}`,
		"weights/model.safetensors":    "weights",
		"tokenizer/tokenizer.json":     "{}",
		"nested/dir/generation_config": "{}",
	})

	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{cacheMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	require.True(t, reused)
	assert.Equal(t, cachePath, source)
	assertFileContent(t, filepath.Join(destPath, "config.json"), `{"model_type":"gpt_oss"}`)
	assertFileContent(t, filepath.Join(destPath, "weights", "model.safetensors"), "weights")
	assertFileContent(t, filepath.Join(destPath, "tokenizer", "tokenizer.json"), "{}")
	assert.NoFileExists(t, destPath+".artifact-cache-staging")
}

func TestTryReuseMountedArtifactCacheRejectsSameSizeCorruption(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{
		"model.safetensors": "weights",
	})
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, "model.safetensors"), []byte("WEIGHTS"), 0644))
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled:        true,
			Mounts:         []string{cacheMount},
			KeyRoot:        defaultMountedArtifactCacheKeyRoot,
			SourceRequired: true,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.ErrorContains(t, err, "checksum mismatch")
	assert.False(t, reused)
	assert.Empty(t, source)
	assert.NoDirExists(t, destPath)
}

func TestTryReuseMountedArtifactCacheMissesWithoutReadyMarker(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	require.NoError(t, os.MkdirAll(cachePath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, mountedArtifactCacheManifestFile), []byte(`{"files":[]}`), 0644))
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{cacheMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	assert.False(t, reused)
	assert.Empty(t, source)
	assert.NoDirExists(t, destPath)
}

func TestTryReuseMountedArtifactCachePropagatesContextCancellation(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{"config.json": "cache"})
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{cacheMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reused, source, err := g.tryReuseMountedArtifactCache(ctx, identity, destPath)

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, reused)
	assert.Empty(t, source)
	assert.NoDirExists(t, destPath)
}

func TestTryReuseMountedArtifactCacheRejectsSizeMismatchAndTriesNextMount(t *testing.T) {
	badMount := t.TempDir()
	goodMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	relativeCacheKey := filepath.Join(defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	badCachePath := filepath.Join(badMount, relativeCacheKey)
	goodCachePath := filepath.Join(goodMount, relativeCacheKey)
	writeMountedArtifactCacheFixture(t, badCachePath, map[string]string{"config.json": "actual"})
	writeMountedArtifactCacheManifest(t, badCachePath, []mountedArtifactCacheManifestFileEntry{{Name: "config.json", Size: 999}})
	writeMountedArtifactCacheFixture(t, goodCachePath, map[string]string{"config.json": "good"})

	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{badMount, goodMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	require.True(t, reused)
	assert.Equal(t, goodCachePath, source)
	assertFileContent(t, filepath.Join(destPath, "config.json"), "good")
}

func TestTryReuseMountedArtifactCacheRejectsSymlinkedDirectoryComponent(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	externalRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(cachePath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(externalRoot, "model.safetensors"), []byte("outside"), 0644))
	require.NoError(t, os.Symlink(externalRoot, filepath.Join(cachePath, "weights")))
	writeMountedArtifactCacheManifest(t, cachePath, []mountedArtifactCacheManifestFileEntry{{
		Name: "weights/model.safetensors",
		Size: int64(len("outside")),
	}})
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, mountedArtifactCacheReadyMarkerFile), nil, 0644))

	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled:        true,
			Mounts:         []string{cacheMount},
			KeyRoot:        defaultMountedArtifactCacheKeyRoot,
			SourceRequired: true,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "symbolic link")
	assert.False(t, reused)
	assert.Empty(t, source)
	assert.NoDirExists(t, destPath)
}

func TestTryReuseMountedArtifactCacheSkipsExistingDestination(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	require.NoError(t, os.MkdirAll(destPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(destPath, "config.json"), []byte("existing"), 0644))
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{"config.json": "cache"})
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{cacheMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	assert.False(t, reused)
	assert.Empty(t, source)
	assertFileContent(t, filepath.Join(destPath, "config.json"), "existing")
}

func TestTryReuseMountedArtifactCacheIgnoresRelativeMount(t *testing.T) {
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{"relative-cache"},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	assert.False(t, reused)
	assert.Empty(t, source)
	assert.NoDirExists(t, destPath)
}

func TestTryReuseMountedArtifactCacheTreatsExistingDestinationAsMissWhenSourceRequired(t *testing.T) {
	cacheMount := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	require.NoError(t, os.MkdirAll(destPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(destPath, "config.json"), []byte("existing"), 0644))
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{"config.json": "cache"})
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled:        true,
			Mounts:         []string{cacheMount},
			KeyRoot:        defaultMountedArtifactCacheKeyRoot,
			SourceRequired: true,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	assert.False(t, reused)
	assert.Empty(t, source)
	assertFileContent(t, filepath.Join(destPath, "config.json"), "existing")
}

func TestTryReuseMountedArtifactCacheDoesNotRemoveExistingSiblingStagingDirectory(t *testing.T) {
	cacheMount := t.TempDir()
	destRoot := t.TempDir()
	destPath := filepath.Join(destRoot, "models", "gpt-oss-120b")
	legacyStagingPath := destPath + ".artifact-cache-staging"
	require.NoError(t, os.MkdirAll(legacyStagingPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyStagingPath, "keep"), []byte("do-not-touch"), 0644))
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   "openai/gpt-oss-120b",
		HFCommitSHA: "abcdef1234567890abcdef1234567890abcdef12",
	}
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", identity.HFCommitSHA)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{"config.json": "cache"})
	g := &Gopher{
		logger: zap.NewNop().Sugar(),
		modelArtifactCache: ModelArtifactCacheConfig{
			Enabled: true,
			Mounts:  []string{cacheMount},
			KeyRoot: defaultMountedArtifactCacheKeyRoot,
		},
	}

	reused, source, err := g.tryReuseMountedArtifactCache(context.Background(), identity, destPath)

	require.NoError(t, err)
	require.True(t, reused)
	assert.Equal(t, cachePath, source)
	assertFileContent(t, filepath.Join(destPath, "config.json"), "cache")
	assertFileContent(t, filepath.Join(legacyStagingPath, "keep"), "do-not-touch")
}

func TestCopyMountedArtifactCacheToDestinationRejectsDestinationCreatedBeforePublish(t *testing.T) {
	cachePath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, "config.json"), []byte("cache"), 0644))
	manifest := mountedArtifactCacheManifest{
		Files: []mountedArtifactCacheManifestFileEntry{{Name: "config.json", Size: 5, SHA256: testSHA256("cache")}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := copyMountedArtifactCacheToDestinationWithHook(ctx, cachePath, destPath, manifest, func() {
		require.NoError(t, os.MkdirAll(destPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(destPath, "config.json"), []byte("existing"), 0644))
	})

	require.ErrorIs(t, err, errMountedArtifactCacheDestinationExists)
	assertFileContent(t, filepath.Join(destPath, "config.json"), "existing")
}

func TestCopyMountedArtifactCacheToDestinationHonorsCanceledContextBeforePublishing(t *testing.T) {
	cachePath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := copyMountedArtifactCacheToDestination(ctx, cachePath, destPath, mountedArtifactCacheManifest{}, 4)

	require.ErrorIs(t, err, context.Canceled)
	assert.NoDirExists(t, destPath)
}

func TestMountedArtifactCacheCopyConcurrency(t *testing.T) {
	assert.Equal(t, 1, mountedArtifactCacheCopyConcurrency(0, 4))
	assert.Equal(t, 1, mountedArtifactCacheCopyConcurrency(4, 0))
	assert.Equal(t, 2, mountedArtifactCacheCopyConcurrency(4, 2))
	assert.Equal(t, 4, mountedArtifactCacheCopyConcurrency(4, 8))
}

func TestCopyMountedArtifactCacheToDestinationUsesConfiguredConcurrency(t *testing.T) {
	cachePath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	manifest := mountedArtifactCacheManifest{
		Files: []mountedArtifactCacheManifestFileEntry{
			{Name: "model-00001.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00002.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00003.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00004.safetensors", Size: 1, SHA256: testSHA256("")},
		},
	}

	var mu sync.Mutex
	active := 0
	maxActive := 0
	started := make(chan struct{}, len(manifest.Files))
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCopies := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseCopies)
	published := false
	copyFile := func(ctx context.Context, cacheRoot string, sourcePath string, targetPath string, expectedSize int64, expectedSHA256 string) error {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()

		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
		}

		mu.Lock()
		active--
		mu.Unlock()
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- copyMountedArtifactCacheToDestinationWithOptions(
			context.Background(),
			cachePath,
			destPath,
			manifest,
			4,
			func() {
				mu.Lock()
				defer mu.Unlock()
				assert.Zero(t, active)
				published = true
			},
			copyFile,
		)
	}()

	for range manifest.Files {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for concurrent cache copies")
		}
	}
	releaseCopies()

	require.NoError(t, <-errCh)
	assert.Equal(t, 4, maxActive)
	assert.True(t, published)
	assert.DirExists(t, destPath)
}

func TestCopyMountedArtifactCacheToDestinationDoesNotExceedConfiguredConcurrency(t *testing.T) {
	cachePath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	manifest := mountedArtifactCacheManifest{
		Files: []mountedArtifactCacheManifestFileEntry{
			{Name: "model-00001.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00002.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00003.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "model-00004.safetensors", Size: 1, SHA256: testSHA256("")},
		},
	}

	started := make(chan struct{}, len(manifest.Files))
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCopies := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseCopies)
	copyFile := func(ctx context.Context, cacheRoot string, sourcePath string, targetPath string, expectedSize int64, expectedSHA256 string) error {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- copyMountedArtifactCacheToDestinationWithOptions(
			context.Background(),
			cachePath,
			destPath,
			manifest,
			2,
			nil,
			copyFile,
		)
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for configured cache copy workers")
		}
	}
	select {
	case <-started:
		t.Fatal("cache copy exceeded configured concurrency")
	case <-time.After(100 * time.Millisecond):
	}

	releaseCopies()
	require.NoError(t, <-errCh)
	assert.DirExists(t, destPath)
}

func TestCopyMountedArtifactCacheToDestinationCancelsWorkersAfterFailure(t *testing.T) {
	cachePath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	manifest := mountedArtifactCacheManifest{
		Files: []mountedArtifactCacheManifestFileEntry{
			{Name: "fail.safetensors", Size: 1, SHA256: testSHA256("")},
			{Name: "blocked.safetensors", Size: 1, SHA256: testSHA256("")},
		},
	}
	copyFailure := errors.New("copy failed")
	blockedStarted := make(chan struct{})
	blockedCanceled := make(chan struct{})
	copyFile := func(ctx context.Context, cacheRoot string, sourcePath string, targetPath string, expectedSize int64, expectedSHA256 string) error {
		if filepath.Base(sourcePath) == "fail.safetensors" {
			select {
			case <-blockedStarted:
			case <-time.After(5 * time.Second):
				return errors.New("timed out waiting for blocked copy")
			}
			return copyFailure
		}
		close(blockedStarted)
		<-ctx.Done()
		close(blockedCanceled)
		return ctx.Err()
	}

	err := copyMountedArtifactCacheToDestinationWithOptions(
		context.Background(),
		cachePath,
		destPath,
		manifest,
		2,
		nil,
		copyFile,
	)

	require.ErrorIs(t, err, copyFailure)
	select {
	case <-blockedCanceled:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for blocked copy cancellation")
	}
	assert.NoDirExists(t, destPath)
	stagingPaths, globErr := filepath.Glob(destPath + ".artifact-cache-staging-*")
	require.NoError(t, globErr)
	assert.Empty(t, stagingPaths)
}

func TestReadMountedArtifactCacheManifestRejectsOversizedManifest(t *testing.T) {
	cachePath := t.TempDir()
	manifestPath := filepath.Join(cachePath, mountedArtifactCacheManifestFile)
	require.NoError(t, os.WriteFile(manifestPath, nil, 0644))
	require.NoError(t, os.Truncate(manifestPath, 64*1024*1024+1))

	_, err := readMountedArtifactCacheManifest(cachePath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestReadMountedArtifactCacheManifestRejectsSymlink(t *testing.T) {
	cachePath := t.TempDir()
	realManifestPath := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(realManifestPath, []byte(`{"files":[{"name":"config.json","size":5}]}`), 0644))
	if err := os.Symlink(realManifestPath, filepath.Join(cachePath, mountedArtifactCacheManifestFile)); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := readMountedArtifactCacheManifest(cachePath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestInspectMountedArtifactCacheRejectsSymlinkReadyMarker(t *testing.T) {
	cachePath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, "config.json"), []byte("cache"), 0644))
	writeMountedArtifactCacheManifest(t, cachePath, []mountedArtifactCacheManifestFileEntry{{Name: "config.json", Size: 5}})
	realReadyPath := filepath.Join(t.TempDir(), "_READY")
	require.NoError(t, os.WriteFile(realReadyPath, []byte("ready\n"), 0644))
	if err := os.Symlink(realReadyPath, filepath.Join(cachePath, mountedArtifactCacheReadyMarkerFile)); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, ready, err := inspectMountedArtifactCache(context.Background(), cachePath)

	require.Error(t, err)
	assert.False(t, ready)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestInspectMountedArtifactCacheRejectsDuplicateManifestPaths(t *testing.T) {
	cachePath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, "config.json"), []byte("cache"), 0644))
	writeMountedArtifactCacheManifest(t, cachePath, []mountedArtifactCacheManifestFileEntry{
		{Name: "config.json", Size: 5},
		{Path: "./config.json", Size: 5},
	})
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, mountedArtifactCacheReadyMarkerFile), []byte("ready\n"), 0644))

	_, ready, err := inspectMountedArtifactCache(context.Background(), cachePath)

	require.Error(t, err)
	assert.False(t, ready)
	assert.Contains(t, err.Error(), "listed more than once")
}

func TestProcessTaskReusesMountedArtifactCacheForImportedOCIModel(t *testing.T) {
	modelID := "openai/gpt-oss-120b"
	sha := "abcdef1234567890abcdef1234567890abcdef12"
	cacheMount := t.TempDir()
	modelRootDir := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", sha)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{
		"config.json": minimalModelConfigJSON,
	})
	reusePolicy := v1beta1.ReuseIfExists
	storageURI := "oci://n/ns/b/bucket/o/customer-imported-basemodels/openai/gpt-oss-120b/" + sha
	model := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpt-oss-120b",
			Namespace: "default",
			UID:       "gpt-oss-120b-uid",
			Annotations: map[string]string{
				HuggingFaceModelIDAnnotationKey: modelID,
				HuggingFaceSHAAnnotationKey:     sha,
			},
		},
		Spec: v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri:     &storageURI,
				Path:           &destPath,
				DownloadPolicy: &reusePolicy,
			},
		},
	}
	g := newGopherForArtifactReuseProcessTask(t, makeConfigMap("node-1", map[string]string{}), modelRootDir, map[string]string{
		"kubernetes.io/hostname": "node-1",
	})
	g.baseModelLister = &mockBaseModelLister{models: []*v1beta1.BaseModel{model}}
	g.modelArtifactCache = ModelArtifactCacheConfig{
		Enabled: true,
		Mounts:  []string{cacheMount},
		KeyRoot: defaultMountedArtifactCacheKeyRoot,
	}

	err := g.processTask(&GopherTask{
		TaskType:  Download,
		BaseModel: model,
	})

	require.NoError(t, err)
	assertFileContent(t, filepath.Join(destPath, "config.json"), minimalModelConfigJSON)
	cm, err := g.configMapReconciler.kubeClient.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	modelKey := g.configMapReconciler.getModelConfigMapKey(model, nil)
	var entry ModelEntry
	require.NoError(t, json.Unmarshal([]byte(cm.Data[modelKey]), &entry))
	assert.Equal(t, ModelStatusReady, entry.Status)
	require.NotNil(t, entry.Config)
	assert.Equal(t, sha, entry.Config.Artifact.Sha)
	require.NotNil(t, entry.Config.Artifact.Origin)
	assert.Equal(t, ArtifactOriginTypeHuggingFace, entry.Config.Artifact.Origin.Type)
	assert.Equal(t, modelID, entry.Config.Artifact.Origin.HFModelID)
	assert.Equal(t, sha, entry.Config.Artifact.Origin.HFCommitSHA)
	canonicalParentPath := canonicalHuggingFaceArtifactPath(destPath, ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   modelID,
		HFCommitSHA: sha,
	})
	parentKey := huggingFaceArtifactConfigMapKey(ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   modelID,
		HFCommitSHA: sha,
	})
	assert.Equal(t, canonicalParentPath, entry.Config.Artifact.ParentPath[parentKey])
	childInfo, err := os.Lstat(destPath)
	require.NoError(t, err)
	assert.NotZero(t, childInfo.Mode()&os.ModeSymlink)
	resolvedChild, err := filepath.EvalSymlinks(destPath)
	require.NoError(t, err)
	resolvedParent, err := filepath.EvalSymlinks(canonicalParentPath)
	require.NoError(t, err)
	assert.Equal(t, resolvedParent, resolvedChild)
}

func TestProcessTaskHydratesLegacyImportedOCIModelWithoutCanonicalReuse(t *testing.T) {
	modelID := "openai/gpt-oss-120b"
	sha := "abcdef1234567890abcdef1234567890abcdef12"
	cacheMount := t.TempDir()
	modelRootDir := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", sha)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{
		"config.json": minimalModelConfigJSON,
	})
	storageURI := "oci://n/ns/b/bucket/o/customer-imported-basemodels/openai/gpt-oss-120b/" + sha
	model := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpt-oss-120b",
			Namespace: "default",
			UID:       "gpt-oss-120b-uid",
			Annotations: map[string]string{
				HuggingFaceModelIDAnnotationKey: modelID,
				HuggingFaceSHAAnnotationKey:     sha,
			},
		},
		Spec: v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri: &storageURI,
				Path:       &destPath,
			},
		},
	}
	g := newGopherForArtifactReuseProcessTask(t, makeConfigMap("node-1", map[string]string{}), modelRootDir, map[string]string{
		"kubernetes.io/hostname": "node-1",
	})
	g.baseModelLister = &mockBaseModelLister{models: []*v1beta1.BaseModel{model}}
	g.modelArtifactCache = ModelArtifactCacheConfig{
		Enabled: true,
		Mounts:  []string{cacheMount},
		KeyRoot: defaultMountedArtifactCacheKeyRoot,
	}

	err := g.processTask(&GopherTask{
		TaskType:  Download,
		BaseModel: model,
	})

	require.NoError(t, err)
	assertFileContent(t, filepath.Join(destPath, "config.json"), minimalModelConfigJSON)
	info, err := os.Lstat(destPath)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink)
	canonicalParentPath := canonicalHuggingFaceArtifactPath(destPath, ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   modelID,
		HFCommitSHA: sha,
	})
	assert.NoDirExists(t, canonicalParentPath)
}

func TestProcessTaskRecordsFailureMetricForRequiredMountedArtifactCacheError(t *testing.T) {
	modelID := "openai/gpt-oss-120b"
	sha := "abcdef1234567890abcdef1234567890abcdef12"
	cacheMount := t.TempDir()
	modelRootDir := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "models", "gpt-oss-120b")
	cachePath := filepath.Join(cacheMount, defaultMountedArtifactCacheKeyRoot, "openai", "gpt-oss-120b", sha)
	writeMountedArtifactCacheFixture(t, cachePath, map[string]string{"config.json": minimalModelConfigJSON})
	writeMountedArtifactCacheManifest(t, cachePath, []mountedArtifactCacheManifestFileEntry{{Name: "config.json", Size: int64(len(minimalModelConfigJSON) + 1)}})
	reusePolicy := v1beta1.ReuseIfExists
	storageURI := "oci://n/ns/b/bucket/o/customer-imported-basemodels/openai/gpt-oss-120b/" + sha
	model := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpt-oss-120b",
			Namespace: "default",
			UID:       "gpt-oss-120b-uid",
			Annotations: map[string]string{
				HuggingFaceModelIDAnnotationKey: modelID,
				HuggingFaceSHAAnnotationKey:     sha,
			},
		},
		Spec: v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri:     &storageURI,
				Path:           &destPath,
				DownloadPolicy: &reusePolicy,
			},
		},
	}
	g := newGopherForArtifactReuseProcessTask(t, makeConfigMap("node-1", map[string]string{}), modelRootDir, map[string]string{
		"kubernetes.io/hostname": "node-1",
	})
	g.baseModelLister = &mockBaseModelLister{models: []*v1beta1.BaseModel{model}}
	g.modelArtifactCache = ModelArtifactCacheConfig{
		Enabled:        true,
		Mounts:         []string{cacheMount},
		KeyRoot:        defaultMountedArtifactCacheKeyRoot,
		SourceRequired: true,
	}

	err := g.processTask(&GopherTask{
		TaskType:  Download,
		BaseModel: model,
	})

	require.Error(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(g.metrics.modelDownloadsFailedTotal.WithLabelValues("BaseModel", "default", "gpt-oss-120b")))
}

const minimalModelConfigJSON = `{
  "architectures": ["LlamaForCausalLM"],
  "hidden_size": 16,
  "intermediate_size": 64,
  "max_position_embeddings": 128,
  "model_type": "llama",
  "num_attention_heads": 2,
  "num_hidden_layers": 2,
  "torch_dtype": "float16",
  "transformers_version": "4.44.0",
  "vocab_size": 128
}`

func writeMountedArtifactCacheFixture(t *testing.T, cachePath string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(cachePath, 0755))
	entries := make([]mountedArtifactCacheManifestFileEntry, 0, len(files))
	for name, content := range files {
		target := filepath.Join(cachePath, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0644))
		entries = append(entries, mountedArtifactCacheManifestFileEntry{Name: name, Size: int64(len(content)), SHA256: testSHA256(content)})
	}
	writeMountedArtifactCacheManifest(t, cachePath, entries)
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, mountedArtifactCacheReadyMarkerFile), []byte("ready\n"), 0644))
}

func writeMountedArtifactCacheManifest(t *testing.T, cachePath string, entries []mountedArtifactCacheManifestFileEntry) {
	t.Helper()
	for i := range entries {
		if entries[i].SHA256 != "" {
			continue
		}
		name := entries[i].Name
		if name == "" {
			name = entries[i].Path
		}
		data, err := os.ReadFile(filepath.Join(cachePath, filepath.FromSlash(name)))
		if err == nil {
			entries[i].SHA256 = testSHA256(string(data))
		}
	}
	manifest := mountedArtifactCacheManifest{Files: entries}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cachePath, mountedArtifactCacheManifestFile), data, 0644))
}

func testSHA256(content string) string {
	checksum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", checksum)
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
