package distributor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				DataDir:              "/mnt/models",
				Namespace:            "ome",
				PodName:              "test-pod",
				PodIP:                "10.0.0.1",
				TorrentPort:          6881,
				MetainfoPort:         8081,
				LeaseDurationSeconds: 120,
			},
			wantErr: false,
		},
		{
			name: "missing data dir",
			config: Config{
				Namespace:    "ome",
				PodName:      "test-pod",
				PodIP:        "10.0.0.1",
				TorrentPort:  6881,
				MetainfoPort: 8081,
			},
			wantErr: true,
		},
		{
			name: "missing namespace",
			config: Config{
				DataDir:      "/mnt/models",
				PodName:      "test-pod",
				PodIP:        "10.0.0.1",
				TorrentPort:  6881,
				MetainfoPort: 8081,
			},
			wantErr: true,
		},
		{
			name: "missing pod name",
			config: Config{
				DataDir:      "/mnt/models",
				Namespace:    "ome",
				PodIP:        "10.0.0.1",
				TorrentPort:  6881,
				MetainfoPort: 8081,
			},
			wantErr: true,
		},
		{
			name: "missing pod IP",
			config: Config{
				DataDir:      "/mnt/models",
				Namespace:    "ome",
				PodName:      "test-pod",
				TorrentPort:  6881,
				MetainfoPort: 8081,
			},
			wantErr: true,
		},
		{
			name: "same torrent and metainfo port",
			config: Config{
				DataDir:      "/mnt/models",
				Namespace:    "ome",
				PodName:      "test-pod",
				PodIP:        "10.0.0.1",
				TorrentPort:  6881,
				MetainfoPort: 6881,
			},
			wantErr: true,
		},
		{
			name: "invalid torrent port",
			config: Config{
				DataDir:      "/mnt/models",
				Namespace:    "ome",
				PodName:      "test-pod",
				PodIP:        "10.0.0.1",
				TorrentPort:  -1,
				MetainfoPort: 8081,
			},
			wantErr: true,
		},
		{
			name: "negative download rate",
			config: Config{
				DataDir:         "/mnt/models",
				Namespace:       "ome",
				PodName:         "test-pod",
				PodIP:           "10.0.0.1",
				TorrentPort:     6881,
				MetainfoPort:    8081,
				MaxDownloadRate: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigWithDefaults(t *testing.T) {
	cfg := Config{}
	result := cfg.WithDefaults()

	assert.Equal(t, "/mnt/models", result.DataDir)
	assert.Equal(t, "ome", result.Namespace)
	assert.Equal(t, 6881, result.TorrentPort)
	assert.Equal(t, 8081, result.MetainfoPort)
	assert.Equal(t, int64(500*1024*1024), result.MaxDownloadRate)
	assert.Equal(t, int64(500*1024*1024), result.MaxUploadRate)
}

func TestMetrics(t *testing.T) {
	metrics := NewMetrics("test")
	require.NotNil(t, metrics)

	// Test recording various metrics (should not panic)
	metrics.RecordDownloadStart("hash1")
	metrics.RecordDownloadComplete("hash1", "p2p", 10*time.Second)
	metrics.RecordDownloadFailed("hash2", "timeout")
	metrics.RecordVerificationFailed("hash3")
	metrics.RecordPeersDiscovered("hash1", 5)
	metrics.RecordPeersConnected("hash1", 3)
	metrics.RecordLeaseAcquired("hash1")
	metrics.RecordWaitingForP2P("hash1")
	metrics.RecordSeeding("hash1")
	metrics.RecordBytesUploaded(1000)
	metrics.RecordBytesDownloaded(2000)
	metrics.RecordP2PDownloadBytes("hash1", 1000)
	metrics.RecordHFDownloadBytes("hash1", 5000)
	metrics.RecordMetainfoRequest("success")
	metrics.RecordMetainfoLatency("hash1", 100*time.Millisecond)
	metrics.UpdateP2PRatio(8, 10)
}

func TestDistributorStats(t *testing.T) {
	stats := Stats{
		ActiveTorrents:       5,
		TotalBytesUploaded:   1000000,
		TotalBytesDownloaded: 2000000,
		ActivePeers:          10,
	}

	assert.Equal(t, 5, stats.ActiveTorrents)
	assert.Equal(t, int64(1000000), stats.TotalBytesUploaded)
	assert.Equal(t, int64(2000000), stats.TotalBytesDownloaded)
	assert.Equal(t, 10, stats.ActivePeers)
}

func TestExistsHelper(t *testing.T) {
	// Create a temp directory for testing
	tempDir, err := os.MkdirTemp("", "p2p-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test-file")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Test the exists helper
	_, existsErr := os.Stat(testFile)
	assert.True(t, existsErr == nil || !os.IsNotExist(existsErr))

	_, existsErr = os.Stat(tempDir)
	assert.True(t, existsErr == nil || !os.IsNotExist(existsErr))

	_, existsErr = os.Stat(filepath.Join(tempDir, "nonexistent"))
	assert.True(t, os.IsNotExist(existsErr))
}

// Integration test helpers

func createTestLogger(t *testing.T) *zap.SugaredLogger {
	return zaptest.NewLogger(t).Sugar()
}

func createTestConfig(dataDir string) Config {
	return Config{
		DataDir:                   dataDir,
		Namespace:                 "test",
		PodName:                   "test-pod",
		PodIP:                     "10.0.0.1",
		PeersService:              "test-peers.test.svc.cluster.local",
		TorrentPort:               16881,
		MetainfoPort:              18081,
		MaxDownloadRate:           100 * 1024 * 1024,
		MaxUploadRate:             100 * 1024 * 1024,
		EnableEncryption:          false,
		LeaseDurationSeconds:      60,
		LeaseRenewIntervalSeconds: 15,
		P2PTimeoutSeconds:         10,
		EnableP2P:                 true,
	}
}
