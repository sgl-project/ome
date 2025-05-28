package ociobjectstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadOptions(t *testing.T) {
	t.Run("WithSizeThreshold", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithSizeThreshold(200))
		require.NoError(t, err)
		assert.Equal(t, 200, opts.SizeThresholdInMB)
	})

	t.Run("WithChunkSize", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithChunkSize(16))
		require.NoError(t, err)
		assert.Equal(t, 16, opts.ChunkSizeInMB)
	})

	t.Run("WithThreads", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithThreads(50))
		require.NoError(t, err)
		assert.Equal(t, 50, opts.Threads)
	})

	t.Run("WithForceStandard", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithForceStandard(true))
		require.NoError(t, err)
		assert.True(t, opts.ForceStandard)
	})

	t.Run("WithForceMultipart", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithForceMultipart(true))
		require.NoError(t, err)
		assert.True(t, opts.ForceMultipart)
	})

	t.Run("WithOverrideEnabled", func(t *testing.T) {
		// Test enabling override (should disable DisableOverride)
		opts, err := applyDownloadOptions(WithOverrideEnabled(true))
		require.NoError(t, err)
		assert.False(t, opts.DisableOverride)

		// Test disabling override (should enable DisableOverride)
		opts, err = applyDownloadOptions(WithOverrideEnabled(false))
		require.NoError(t, err)
		assert.True(t, opts.DisableOverride)
	})

	t.Run("WithExcludePatterns", func(t *testing.T) {
		patterns := []string{"*.tmp", "*.log"}
		opts, err := applyDownloadOptions(WithExcludePatterns(patterns))
		require.NoError(t, err)
		assert.Equal(t, patterns, opts.ExcludePatterns)
	})

	t.Run("WithStripPrefix", func(t *testing.T) {
		prefix := "models/v1/"
		opts, err := applyDownloadOptions(WithStripPrefix(prefix))
		require.NoError(t, err)
		assert.True(t, opts.StripPrefix)
		assert.Equal(t, prefix, opts.PrefixToStrip)
	})

	t.Run("WithBaseNameOnly", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithBaseNameOnly(true))
		require.NoError(t, err)
		assert.True(t, opts.UseBaseNameOnly)
	})

	t.Run("WithTailOverlap", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithTailOverlap(true))
		require.NoError(t, err)
		assert.True(t, opts.JoinWithTailOverlap)
	})
}

func TestDownloadOptionsChaining(t *testing.T) {
	opts, err := applyDownloadOptions(
		WithSizeThreshold(150),
		WithChunkSize(32),
		WithThreads(25),
		WithForceStandard(false),
		WithOverrideEnabled(true),
		WithExcludePatterns([]string{"*.bin"}),
		WithStripPrefix("data/"),
		WithBaseNameOnly(false),
		WithTailOverlap(true),
	)
	require.NoError(t, err)

	assert.Equal(t, 150, opts.SizeThresholdInMB)
	assert.Equal(t, 32, opts.ChunkSizeInMB)
	assert.Equal(t, 25, opts.Threads)
	assert.False(t, opts.ForceStandard)
	assert.False(t, opts.DisableOverride) // Override enabled means DisableOverride is false
	assert.Equal(t, []string{"*.bin"}, opts.ExcludePatterns)
	assert.True(t, opts.StripPrefix)
	assert.Equal(t, "data/", opts.PrefixToStrip)
	assert.False(t, opts.UseBaseNameOnly)
	assert.True(t, opts.JoinWithTailOverlap)
}

func TestDefaultDownloadOptions(t *testing.T) {
	// Test that applying no options returns defaults
	opts, err := applyDownloadOptions()
	require.NoError(t, err)

	defaults := DefaultDownloadOptions()
	assert.Equal(t, defaults.SizeThresholdInMB, opts.SizeThresholdInMB)
	assert.Equal(t, defaults.ChunkSizeInMB, opts.ChunkSizeInMB)
	assert.Equal(t, defaults.Threads, opts.Threads)
	assert.Equal(t, defaults.ForceStandard, opts.ForceStandard)
	assert.Equal(t, defaults.ForceMultipart, opts.ForceMultipart)
	assert.Equal(t, defaults.DisableOverride, opts.DisableOverride)
	assert.Equal(t, defaults.ExcludePatterns, opts.ExcludePatterns)
	assert.Equal(t, defaults.StripPrefix, opts.StripPrefix)
	assert.Equal(t, defaults.UseBaseNameOnly, opts.UseBaseNameOnly)
	assert.Equal(t, defaults.JoinWithTailOverlap, opts.JoinWithTailOverlap)
}

func TestDownloadOptionsWithNilOptions(t *testing.T) {
	// Test that nil options are safely ignored
	opts, err := applyDownloadOptions(
		WithThreads(10),
		nil, // This should be safely ignored
		WithChunkSize(8),
	)
	require.NoError(t, err)

	assert.Equal(t, 10, opts.Threads)
	assert.Equal(t, 8, opts.ChunkSizeInMB)
}
