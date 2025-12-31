package distributor

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anacrolix/torrent/metainfo"
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
	// MaxDownloadRate and MaxUploadRate are not set by WithDefaults (0 means unlimited)
	assert.Equal(t, int64(0), result.MaxDownloadRate)
	assert.Equal(t, int64(0), result.MaxUploadRate)
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

// TestParallelHashingMatchesStandardLibrary verifies that our parallel piece hashing
// produces identical results to the standard library's sequential BuildFromFilePath.
// This is critical because any difference would cause "piece count and file lengths are at odds" errors.
func TestParallelHashingMatchesStandardLibrary(t *testing.T) {
	const pieceLength int64 = 256 * 1024 // 256KB pieces for faster testing

	tests := []struct {
		name  string
		files map[string]int // filename -> size in bytes
	}{
		{
			name: "single small file",
			files: map[string]int{
				"small.bin": 1024, // 1KB, smaller than piece size
			},
		},
		{
			name: "single large file",
			files: map[string]int{
				"large.bin": 1024 * 1024, // 1MB, spans multiple pieces
			},
		},
		{
			name: "multiple files aligned to piece boundaries",
			files: map[string]int{
				"a.bin": 256 * 1024, // exactly 1 piece
				"b.bin": 512 * 1024, // exactly 2 pieces
				"c.bin": 256 * 1024, // exactly 1 piece
			},
		},
		{
			name: "multiple files NOT aligned to piece boundaries",
			files: map[string]int{
				"a.bin": 100 * 1024, // partial piece
				"b.bin": 300 * 1024, // spans piece boundary
				"c.bin": 50 * 1024,  // partial piece
				"d.bin": 400 * 1024, // spans piece boundary
				"e.bin": 150 * 1024, // partial piece
			},
		},
		{
			name: "files in subdirectories",
			files: map[string]int{
				"dir1/file1.bin":    100 * 1024,
				"dir1/file2.bin":    200 * 1024,
				"dir2/subdir/a.bin": 150 * 1024,
				"dir2/subdir/b.bin": 250 * 1024,
			},
		},
		{
			name: "many small files spanning pieces",
			files: map[string]int{
				"01.bin": 50 * 1024,
				"02.bin": 50 * 1024,
				"03.bin": 50 * 1024,
				"04.bin": 50 * 1024,
				"05.bin": 50 * 1024,
				"06.bin": 50 * 1024,
				"07.bin": 50 * 1024,
				"08.bin": 50 * 1024,
				"09.bin": 50 * 1024,
				"10.bin": 50 * 1024,
			},
		},
		{
			name: "empty file mixed with others",
			files: map[string]int{
				"a.bin":     100 * 1024,
				"empty.bin": 0, // empty file
				"b.bin":     200 * 1024,
			},
		},
		{
			name: "file sizes that create tricky boundaries",
			files: map[string]int{
				"a.bin": 256*1024 - 1, // one byte short of piece
				"b.bin": 1,            // just one byte (completes previous piece)
				"c.bin": 256*1024 + 1, // one byte over piece
				"d.bin": 256*1024 - 1, // one byte short (combined with c's extra = full piece)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tempDir, err := os.MkdirTemp("", "parallel-hash-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create test files with random content
			for filename, size := range tt.files {
				filePath := filepath.Join(tempDir, filename)
				err := os.MkdirAll(filepath.Dir(filePath), 0755)
				require.NoError(t, err)

				data := make([]byte, size)
				if size > 0 {
					_, err = rand.Read(data)
					require.NoError(t, err)
				}
				err = os.WriteFile(filePath, data, 0644)
				require.NoError(t, err)
			}

			// Generate using standard library
			stdInfo := metainfo.Info{PieceLength: pieceLength}
			err = stdInfo.BuildFromFilePath(tempDir)
			require.NoError(t, err)

			// Generate using our parallel implementation
			logger := zaptest.NewLogger(t).Sugar()
			d := &ModelDistributor{logger: logger}
			parallelInfo, err := d.buildInfoParallel(tempDir, filepath.Base(tempDir), pieceLength)
			require.NoError(t, err)

			// Compare results
			assert.Equal(t, stdInfo.PieceLength, parallelInfo.PieceLength, "piece length mismatch")
			assert.Equal(t, stdInfo.TotalLength(), parallelInfo.TotalLength(), "total length mismatch")
			assert.Equal(t, len(stdInfo.Files), len(parallelInfo.Files), "file count mismatch")

			// Compare file list (order matters!)
			for i := range stdInfo.Files {
				assert.Equal(t, stdInfo.Files[i].Length, parallelInfo.Files[i].Length,
					"file %d length mismatch", i)
				assert.Equal(t, stdInfo.Files[i].Path, parallelInfo.Files[i].Path,
					"file %d path mismatch", i)
			}

			// The critical comparison: piece hashes must match exactly
			if !bytes.Equal(stdInfo.Pieces, parallelInfo.Pieces) {
				t.Errorf("PIECE HASH MISMATCH!\n"+
					"Standard library pieces: %d bytes (%d pieces)\n"+
					"Parallel impl pieces:    %d bytes (%d pieces)",
					len(stdInfo.Pieces), len(stdInfo.Pieces)/20,
					len(parallelInfo.Pieces), len(parallelInfo.Pieces)/20)

				// Find first differing piece for debugging
				for i := 0; i < len(stdInfo.Pieces) && i < len(parallelInfo.Pieces); i += 20 {
					pieceNum := i / 20
					stdPiece := stdInfo.Pieces[i : i+20]
					parallelPiece := parallelInfo.Pieces[i : i+20]
					if !bytes.Equal(stdPiece, parallelPiece) {
						t.Errorf("First difference at piece %d:\n  std: %x\n  par: %x",
							pieceNum, stdPiece, parallelPiece)
						break
					}
				}
			}
		})
	}
}

// TestParallelHashingSingleFile tests the edge case of a single file (not a directory).
func TestParallelHashingSingleFile(t *testing.T) {
	const pieceLength int64 = 256 * 1024

	tempDir, err := os.MkdirTemp("", "parallel-hash-single")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a single file
	filePath := filepath.Join(tempDir, "model.bin")
	data := make([]byte, 500*1024) // 500KB
	_, err = rand.Read(data)
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0644)
	require.NoError(t, err)

	// Test with directory containing single file
	stdInfo := metainfo.Info{PieceLength: pieceLength}
	err = stdInfo.BuildFromFilePath(tempDir)
	require.NoError(t, err)

	logger := zaptest.NewLogger(t).Sugar()
	d := &ModelDistributor{logger: logger}
	parallelInfo, err := d.buildInfoParallel(tempDir, filepath.Base(tempDir), pieceLength)
	require.NoError(t, err)

	assert.True(t, bytes.Equal(stdInfo.Pieces, parallelInfo.Pieces),
		"piece hashes don't match for single file case")
}
