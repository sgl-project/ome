package controllerconfig

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// The start-up form reads the API server once, the cached form shares one
// GET within the TTL, and both hand the ConfigMap to the block's parser.
func TestLoadBlock_PlainAndCachedForms_OneGetEachThenParse(t *testing.T) {
	clientset, gets := countingClientset(t, map[string]string{"block": `{"n": 1}`})
	parse := func(configMap *v1.ConfigMap) (string, error) { return configMap.Data["block"], nil }

	got, err := loadBlock(clientset, parse)
	require.NoError(t, err)
	assert.Equal(t, `{"n": 1}`, got)
	assert.Equal(t, int64(1), atomic.LoadInt64(gets))

	cache := NewConfigCache(time.Minute)
	for range 2 {
		got, err = loadBlockCached(cache, clientset, parse)
		require.NoError(t, err)
		assert.Equal(t, `{"n": 1}`, got)
	}
	assert.Equal(t, int64(2), atomic.LoadInt64(gets), "the cached form shares one GET within the TTL")
}

// A missing ConfigMap and a parser's own error both come back unchanged, with
// no partial block beside them.
func TestLoadBlock_MissingConfigMapOrParseError_ErrorWithNoBlock(t *testing.T) {
	parseErr := errors.New("block is malformed")
	failing := func(*v1.ConfigMap) (*RolloutConfig, error) { return nil, parseErr }

	cfg, err := loadBlock(fake.NewSimpleClientset(), failing)
	require.ErrorContains(t, err, `configmaps "inferenceservice-config" not found`)
	assert.Nil(t, cfg)

	clientset, _ := countingClientset(t, map[string]string{})
	cfg, err = loadBlock(clientset, failing)
	require.ErrorIs(t, err, parseErr)
	assert.Nil(t, cfg)

	cfg, err = loadBlockCached(nil, clientset, failing)
	require.ErrorIs(t, err, parseErr)
	assert.Nil(t, cfg)
}

// One rule for a duration the operator may leave out: a positive duration
// reads as itself, anything else as zero.
func TestPositiveDurationOrZero_AnyString_PositiveReadsItselfElseZero(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"15m":   15 * time.Minute,
		"250ms": 250 * time.Millisecond,
		"":      0,
		"nope":  0,
		"30":    0,
		"0s":    0,
		"-5m":   0,
	} {
		assert.Equal(t, want, positiveDurationOrZero(value), "value %q", value)
	}
}

func TestReadPositiveDuration_BadValue_ErrorNamesKeyAndValue(t *testing.T) {
	d, err := readPositiveDuration("block.interval", "5s")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, d)

	for value, wantErr := range map[string]string{
		"many": `invalid block.interval "many": time: invalid duration "many"`,
		"":     `invalid block.interval "": time: invalid duration ""`,
		"0s":   "invalid block.interval: must be > 0, got 0s",
		"-1s":  "invalid block.interval: must be > 0, got -1s",
	} {
		d, err := readPositiveDuration("block.interval", value)
		assert.EqualError(t, err, wantErr)
		assert.Zero(t, d)
	}
}

func TestReadPositiveCount_NonPositive_ErrorNamesKeyAndValue(t *testing.T) {
	n, err := readPositiveCount("block.count", int32(400))
	require.NoError(t, err)
	assert.Equal(t, int32(400), n)

	for value, wantErr := range map[int64]string{
		0:  "invalid block.count: must be > 0, got 0",
		-1: "invalid block.count: must be > 0, got -1",
	} {
		n, err := readPositiveCount("block.count", value)
		assert.EqualError(t, err, wantErr)
		assert.Zero(t, n)
	}
}
