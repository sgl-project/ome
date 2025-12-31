// Package distributor implements P2P model distribution using BitTorrent protocol.
// It provides BitTorrent client functionality for peer-to-peer file transfer.
// Lease coordination and HuggingFace integration are handled by the model-agent.
package distributor

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// ModelDistributor handles P2P model distribution via BitTorrent.
// It manages the torrent client, peer discovery, and seeding.
// Lease coordination is handled externally by the model-agent.
type ModelDistributor struct {
	torrentClient *torrent.Client
	dataDir       string
	podIP         string
	peersService  string // headless service DNS for peer discovery
	torrentPort   int
	metainfoPort  int
	logger        *zap.SugaredLogger

	// Active torrents for seeding
	activeTorrents map[string]*torrent.Torrent
	torrentsMu     sync.RWMutex

	// Metrics collector
	metrics *Metrics

	// HTTP client for metainfo fetching
	httpClient *http.Client
}

// New creates a new ModelDistributor with the given configuration.
func New(cfg Config, logger *zap.SugaredLogger) (*ModelDistributor, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	torrentCfg := torrent.NewDefaultClientConfig()
	torrentCfg.DataDir = cfg.DataDir
	torrentCfg.Seed = true
	torrentCfg.ListenPort = cfg.TorrentPort
	torrentCfg.NoDHT = true           // use k8s DNS for discovery
	torrentCfg.DisableTrackers = true // no external trackers

	// Enable header obfuscation for security if configured
	if cfg.EnableEncryption {
		torrentCfg.HeaderObfuscationPolicy.Preferred = true
		torrentCfg.HeaderObfuscationPolicy.RequirePreferred = cfg.RequireEncryption
	}

	// Rate limiting
	if cfg.MaxDownloadRate > 0 {
		torrentCfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(cfg.MaxDownloadRate), int(cfg.MaxDownloadRate))
	}
	if cfg.MaxUploadRate > 0 {
		torrentCfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(cfg.MaxUploadRate), int(cfg.MaxUploadRate))
	}

	client, err := torrent.NewClient(torrentCfg)
	if err != nil {
		return nil, fmt.Errorf("create torrent client: %w", err)
	}

	httpClient := &http.Client{
		// No overall timeout - we use per-request context timeouts instead
		// This allows large metainfo files (5MB+ for 1TB models) to transfer
		Transport: &http.Transport{
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       60 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, // Wait up to 30s for headers
		},
	}

	return &ModelDistributor{
		torrentClient:  client,
		dataDir:        cfg.DataDir,
		podIP:          cfg.PodIP,
		peersService:   cfg.PeersService,
		torrentPort:    cfg.TorrentPort,
		metainfoPort:   cfg.MetainfoPort,
		logger:         logger,
		activeTorrents: make(map[string]*torrent.Torrent),
		metrics:        NewMetrics(cfg.Namespace),
		httpClient:     httpClient,
	}, nil
}

// Close releases all resources held by the distributor.
func (d *ModelDistributor) Close() {
	d.logger.Info("Shutting down P2P distributor")

	d.torrentsMu.Lock()
	for hash, t := range d.activeTorrents {
		d.logger.Debugf("Dropping torrent for model %s", hash)
		t.Drop()
	}
	d.activeTorrents = make(map[string]*torrent.Torrent)
	d.torrentsMu.Unlock()

	if d.torrentClient != nil {
		d.torrentClient.Close()
	}

	d.logger.Info("P2P distributor shutdown complete")
}

// TryP2PDownload attempts to download a model from peers.
// destPath is the final destination where the model files should be placed.
// Returns nil if successful, error if P2P download is not available.
func (d *ModelDistributor) TryP2PDownload(ctx context.Context, modelHash, destPath string, timeout time.Duration) error {
	// Check context before starting - fail fast if already cancelled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	peers, err := d.discoverPeers(ctx)
	if err != nil || len(peers) == 0 {
		return fmt.Errorf("no peers available: %v", err)
	}

	d.logger.Debugf("Discovered %d peers for model %s", len(peers), modelHash)
	d.metrics.RecordPeersDiscovered(modelHash, len(peers))

	// Try to get metainfo from a peer
	mi, err := d.fetchMetainfoFromPeer(ctx, peers, modelHash)
	if err != nil {
		return fmt.Errorf("no peer has metainfo: %w", err)
	}

	t, err := d.torrentClient.AddTorrent(mi)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}

	// Wait for download with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-t.GotInfo():
		// Verify torrent has content before proceeding
		if t.Info().TotalLength() == 0 {
			t.Drop()
			return fmt.Errorf("torrent has 0 bytes, likely corrupt metainfo")
		}
		if t.NumPieces() == 0 {
			t.Drop()
			return fmt.Errorf("torrent has 0 pieces, likely corrupt metainfo")
		}
		d.logger.Debugf("Torrent info ready for %s: %d bytes, %d pieces", modelHash, t.Info().TotalLength(), t.NumPieces())
		// Add peers after torrent info is available (required for proper handshaking)
		peerInfos := make([]torrent.PeerInfo, len(peers))
		for i, p := range peers {
			peerInfos[i] = p
		}
		t.AddPeers(peerInfos)
		d.logger.Debugf("Added %d peers for model %s, starting download", len(peers), modelHash)

		// Register torrent immediately so we can share pieces while downloading
		// This enables true swarm behavior where all nodes contribute
		d.torrentsMu.Lock()
		d.activeTorrents[modelHash] = t
		d.torrentsMu.Unlock()

		t.DownloadAll()
		if !d.waitForComplete(ctx, t) {
			// Remove from active torrents on failure
			d.torrentsMu.Lock()
			delete(d.activeTorrents, modelHash)
			d.torrentsMu.Unlock()
			t.Drop()
			return fmt.Errorf("download incomplete within timeout")
		}

		// Downloaded files are at {dataDir}/{modelHash}
		// Move them to destPath and create symlink for continued seeding
		downloadPath := filepath.Join(d.dataDir, modelHash)

		// Ensure parent directory of destPath exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			t.Drop()
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Remove existing destPath if it exists (may be incomplete from previous download)
		// This is safe because we just completed a fresh P2P download with verified pieces
		if _, err := os.Stat(destPath); err == nil {
			d.logger.Infof("Removing existing (possibly incomplete) model at %s before replacing with P2P download", destPath)
			if err := os.RemoveAll(destPath); err != nil {
				t.Drop()
				return fmt.Errorf("failed to remove existing destination: %w", err)
			}
		}

		// Move downloaded files to destination
		if err := os.Rename(downloadPath, destPath); err != nil {
			t.Drop()
			return fmt.Errorf("failed to move downloaded files to destination: %w", err)
		}
		d.logger.Debugf("Moved downloaded model from %s to %s", downloadPath, destPath)

		// Create symlink from hash path to destination for continued seeding
		if err := os.Symlink(destPath, downloadPath); err != nil {
			d.logger.Warnf("Failed to create symlink for seeding: %v", err)
			// Don't fail the download, seeding is optional
		}

		// Torrent already registered in activeTorrents at start of download
		d.metrics.RecordSeeding(modelHash)
		return nil
	case <-ctx.Done():
		t.Drop()
		return ctx.Err()
	}
}

// SeedModel starts seeding an existing model directory.
func (d *ModelDistributor) SeedModel(path, modelHash string) error {
	d.torrentsMu.Lock()
	defer d.torrentsMu.Unlock()

	if _, exists := d.activeTorrents[modelHash]; exists {
		d.logger.Debugf("Already seeding model %s", modelHash)
		return nil
	}

	// Create symlink from {dataDir}/{modelHash} to actual model path
	// This allows the torrent client to find files at the expected location
	symlinkPath := filepath.Join(d.dataDir, modelHash)
	if _, err := os.Lstat(symlinkPath); err == nil {
		// Symlink or file already exists, remove it
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("failed to remove existing symlink: %w", err)
		}
	}
	if err := os.Symlink(path, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink for seeding: %w", err)
	}
	d.logger.Debugf("Created symlink %s -> %s for seeding", symlinkPath, path)

	d.logger.Infof("Creating metainfo for model %s at path %s (this may take several minutes for large models)...", modelHash, path)
	startTime := time.Now()

	mi, err := d.createMetainfo(path, modelHash)
	if err != nil {
		return fmt.Errorf("failed to create metainfo: %w", err)
	}

	metainfoTime := time.Since(startTime)
	d.logger.Infof("Metainfo created for model %s in %v", modelHash, metainfoTime.Round(time.Second))

	// Cache metainfo to disk for fast serving and persistence across restarts
	if err := d.saveMetainfoToFile(mi, modelHash); err != nil {
		d.logger.Warnf("Failed to cache metainfo to disk (will serve from memory): %v", err)
	}

	t, err := d.torrentClient.AddTorrent(mi)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}

	<-t.GotInfo()
	d.activeTorrents[modelHash] = t
	d.metrics.RecordSeeding(modelHash)

	totalTime := time.Since(startTime)
	d.logger.Infof("Started seeding model %s (total setup time: %v)", modelHash, totalTime.Round(time.Second))
	return nil
}

// StopSeeding stops seeding a model.
func (d *ModelDistributor) StopSeeding(modelHash string) {
	d.torrentsMu.Lock()
	defer d.torrentsMu.Unlock()

	if t, exists := d.activeTorrents[modelHash]; exists {
		t.Drop()
		delete(d.activeTorrents, modelHash)
		d.logger.Debugf("Stopped seeding model %s", modelHash)
	}
}

// HasPeers checks if there are any peers available for the model.
func (d *ModelDistributor) HasPeers(ctx context.Context, modelHash string) bool {
	// Check context before starting
	if ctx.Err() != nil {
		return false
	}

	peers, err := d.discoverPeers(ctx)
	if err != nil || len(peers) == 0 {
		return false
	}

	// Check if any peer has metainfo
	_, err = d.fetchMetainfoFromPeer(ctx, peers, modelHash)
	return err == nil
}

// GetMetainfo returns the metainfo for a model if it's being seeded.
func (d *ModelDistributor) GetMetainfo(modelHash string) (*metainfo.MetaInfo, bool) {
	d.torrentsMu.RLock()
	defer d.torrentsMu.RUnlock()

	t, exists := d.activeTorrents[modelHash]
	if !exists {
		return nil, false
	}

	// Use the torrent's Metainfo() method to get the correct info bytes
	// Re-marshaling t.Info() would produce different bytes (different info hash)
	mi := t.Metainfo()
	return &mi, true
}

// IsSeeding returns whether the distributor is seeding the given model.
func (d *ModelDistributor) IsSeeding(modelHash string) bool {
	d.torrentsMu.RLock()
	defer d.torrentsMu.RUnlock()
	_, exists := d.activeTorrents[modelHash]
	return exists
}

// GetStats returns current P2P statistics.
func (d *ModelDistributor) GetStats() Stats {
	d.torrentsMu.RLock()
	defer d.torrentsMu.RUnlock()

	stats := Stats{
		ActiveTorrents: len(d.activeTorrents),
	}

	for _, t := range d.activeTorrents {
		ts := t.Stats()
		stats.TotalBytesUploaded += ts.BytesWrittenData.Int64()
		stats.TotalBytesDownloaded += ts.BytesReadData.Int64()
		stats.ActivePeers += ts.ActivePeers
	}

	return stats
}

// Stats contains P2P distribution statistics.
type Stats struct {
	ActiveTorrents       int
	TotalBytesUploaded   int64
	TotalBytesDownloaded int64
	ActivePeers          int
}

// discoverPeers uses DNS to find other pods in the headless service.
func (d *ModelDistributor) discoverPeers(ctx context.Context) ([]torrent.PeerInfo, error) {
	if d.peersService == "" {
		return nil, fmt.Errorf("peers service not configured")
	}

	// Use context-aware DNS resolver to support cancellation
	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, d.peersService)
	if err != nil {
		// Check if cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	var peers []torrent.PeerInfo
	for _, ip := range ips {
		ipStr := ip.IP.String()
		if ipStr == d.podIP {
			continue // skip self
		}

		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			continue
		}

		addrPort := netip.AddrPortFrom(addr, uint16(d.torrentPort))
		peers = append(peers, torrent.PeerInfo{Addr: addrPort})
	}

	return peers, nil
}

// fetchMetainfoFromPeer tries each peer until one responds with metainfo.
func (d *ModelDistributor) fetchMetainfoFromPeer(ctx context.Context, peers []torrent.PeerInfo, modelHash string) (*metainfo.MetaInfo, error) {
	for _, peer := range peers {
		// Check context before each peer attempt - fail fast if cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Extract IP from peer address (format: ip:port)
		addrPort, ok := peer.Addr.(netip.AddrPort)
		if !ok {
			continue
		}
		url := fmt.Sprintf("http://%s:%d/metainfo/%s", addrPort.Addr().String(), d.metainfoPort, modelHash)

		// Use per-request timeout context (2 minutes per peer for large metainfo files)
		// A 1TB model has ~250K pieces, ~5MB metainfo file - needs time to transfer
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			continue
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			cancel()
			d.logger.Debugf("Failed to fetch metainfo from %s: %v", url, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			continue
		}

		mi, err := metainfo.Load(resp.Body)
		resp.Body.Close() // Close immediately after reading, not deferred in loop
		cancel()
		if err != nil {
			d.logger.Debugf("Failed to parse metainfo from %s: %v", url, err)
			continue
		}

		// Validate metainfo has actual content - protect against truncated/corrupt transfers
		info, err := mi.UnmarshalInfo()
		if err != nil {
			d.logger.Debugf("Failed to unmarshal metainfo info from %s: %v", url, err)
			continue
		}
		if info.TotalLength() == 0 {
			d.logger.Warnf("Received empty metainfo from %s (0 bytes), skipping", url)
			continue
		}
		if len(info.Pieces) == 0 {
			d.logger.Warnf("Received metainfo with no pieces from %s, skipping", url)
			continue
		}
		d.logger.Debugf("Fetched valid metainfo for %s from peer %s (size: %d bytes, pieces: %d)",
			modelHash, peer.Addr, info.TotalLength(), len(info.Pieces)/20)
		return mi, nil
	}

	return nil, fmt.Errorf("no peer has metainfo for %s", modelHash)
}

// waitForComplete waits until torrent download is complete using event-based waiting.
func (d *ModelDistributor) waitForComplete(ctx context.Context, t *torrent.Torrent) bool {
	// Get completion status - returns a struct with a channel that closes when complete
	completion := t.Complete()

	// If already complete, return immediately
	if completion.Bool() {
		return true
	}

	// Wait for completion or context cancellation using event-based waiting
	// This is more efficient than polling every second
	select {
	case <-ctx.Done():
		return false
	case <-completion.On():
		return true
	}
}

// createMetainfo builds a torrent metainfo for the given path using parallel piece hashing.
// This correctly handles piece boundaries across files while parallelizing I/O.
func (d *ModelDistributor) createMetainfo(path, name string) (*metainfo.MetaInfo, error) {
	const pieceLength int64 = 4 * 1024 * 1024 // 4MB pieces

	info, err := d.buildInfoParallel(path, name, pieceLength)
	if err != nil {
		return nil, err
	}

	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal info: %w", err)
	}

	return &metainfo.MetaInfo{InfoBytes: infoBytes}, nil
}

// metainfoFilePath returns the path to the cached .torrent file for a model.
func (d *ModelDistributor) metainfoFilePath(modelHash string) string {
	return filepath.Join(d.dataDir, modelHash+".torrent")
}

// saveMetainfoToFile caches the metainfo to a .torrent file for fast serving.
func (d *ModelDistributor) saveMetainfoToFile(mi *metainfo.MetaInfo, modelHash string) error {
	filePath := d.metainfoFilePath(modelHash)
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create metainfo file: %w", err)
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		os.Remove(filePath) // Clean up partial file
		return fmt.Errorf("failed to write metainfo file: %w", err)
	}

	d.logger.Infof("Cached metainfo to %s", filePath)
	return nil
}

// LoadMetainfoFromFile loads cached metainfo from disk.
// Returns nil, nil if file doesn't exist.
func (d *ModelDistributor) LoadMetainfoFromFile(modelHash string) (*metainfo.MetaInfo, error) {
	filePath := d.metainfoFilePath(modelHash)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not cached, not an error
		}
		return nil, fmt.Errorf("failed to open metainfo file: %w", err)
	}
	defer f.Close()

	mi, err := metainfo.Load(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metainfo file: %w", err)
	}

	d.logger.Debugf("Loaded cached metainfo from %s", filePath)
	return mi, nil
}

// fileSegment represents a file in the torrent with its position in the virtual stream.
type fileSegment struct {
	path        string // Full path to file
	relPath     string // Relative path for torrent
	size        int64  // File size
	startOffset int64  // Start offset in virtual stream
	endOffset   int64  // End offset in virtual stream (exclusive)
}

// buildInfoParallel builds torrent Info with parallel piece hashing.
// Unlike per-file parallel hashing, this correctly handles piece boundaries
// by treating all files as one continuous stream and hashing pieces in parallel.
func (d *ModelDistributor) buildInfoParallel(root, name string, pieceLength int64) (*metainfo.Info, error) {
	// Collect and sort files
	var files []fileSegment
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, fileSegment{
			path:    path,
			relPath: relPath,
			size:    fi.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Sort files for consistent ordering (same as standard BuildFromFilePath)
	sort.Slice(files, func(i, j int) bool {
		return files[i].relPath < files[j].relPath
	})

	// Calculate offsets in virtual stream
	var totalSize int64
	for i := range files {
		files[i].startOffset = totalSize
		totalSize += files[i].size
		files[i].endOffset = totalSize
	}

	if totalSize == 0 {
		return nil, fmt.Errorf("no files found in %s", root)
	}

	// Calculate number of pieces
	numPieces := (totalSize + pieceLength - 1) / pieceLength

	// Determine worker count
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16
	}
	if int64(numWorkers) > numPieces {
		numWorkers = int(numPieces)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	d.logger.Infof("Hashing %d files (%d GB) as %d pieces with %d parallel workers",
		len(files), totalSize/(1024*1024*1024), numPieces, numWorkers)

	// Create piece hash results array
	pieceHashes := make([][]byte, numPieces)
	var hashErr error
	var errOnce sync.Once

	// Work distribution
	var wg sync.WaitGroup
	pieceCh := make(chan int64, numPieces)

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each worker gets its own buffer to avoid allocation per piece
			buf := make([]byte, pieceLength)
			hasher := sha1.New()

			for pieceIdx := range pieceCh {
				hash, err := d.hashPiece(files, pieceIdx, pieceLength, totalSize, buf, hasher)
				if err != nil {
					errOnce.Do(func() {
						hashErr = fmt.Errorf("failed to hash piece %d: %w", pieceIdx, err)
					})
					continue
				}
				pieceHashes[pieceIdx] = hash
			}
		}()
	}

	// Send work
	for i := int64(0); i < numPieces; i++ {
		pieceCh <- i
	}
	close(pieceCh)

	wg.Wait()

	if hashErr != nil {
		return nil, hashErr
	}

	// Combine piece hashes
	pieces := make([]byte, 0, numPieces*sha1.Size)
	for _, h := range pieceHashes {
		pieces = append(pieces, h...)
	}

	// Build file list for multi-file torrent
	infoFiles := make([]metainfo.FileInfo, len(files))
	for i, f := range files {
		infoFiles[i] = metainfo.FileInfo{
			Path:   strings.Split(f.relPath, string(filepath.Separator)),
			Length: f.size,
		}
	}

	// Use directory name if no name provided
	if name == "" {
		name = filepath.Base(root)
	}

	return &metainfo.Info{
		Name:        name,
		PieceLength: pieceLength,
		Pieces:      pieces,
		Files:       infoFiles,
	}, nil
}

// hashPiece hashes a single piece, correctly reading across file boundaries.
func (d *ModelDistributor) hashPiece(files []fileSegment, pieceIdx, pieceLength, totalSize int64, buf []byte, hasher hash.Hash) ([]byte, error) {
	pieceStart := pieceIdx * pieceLength
	pieceEnd := pieceStart + pieceLength
	if pieceEnd > totalSize {
		pieceEnd = totalSize
	}

	hasher.Reset()
	remaining := pieceEnd - pieceStart
	currentOffset := pieceStart

	for remaining > 0 {
		// Find the file containing currentOffset
		fileIdx := d.findFileForOffset(files, currentOffset)
		if fileIdx < 0 {
			return nil, fmt.Errorf("no file found for offset %d", currentOffset)
		}

		f := files[fileIdx]
		// Calculate read position within this file
		fileOffset := currentOffset - f.startOffset
		// Calculate how much we can read from this file
		canRead := f.size - fileOffset
		if canRead > remaining {
			canRead = remaining
		}

		// Read from file
		data, err := d.readFileAt(f.path, fileOffset, canRead, buf[:canRead])
		if err != nil {
			return nil, fmt.Errorf("failed to read %s at offset %d: %w", f.path, fileOffset, err)
		}

		hasher.Write(data)
		currentOffset += canRead
		remaining -= canRead
	}

	return hasher.Sum(nil), nil
}

// findFileForOffset finds the file index containing the given virtual offset.
func (d *ModelDistributor) findFileForOffset(files []fileSegment, offset int64) int {
	// Binary search for efficiency with many files
	lo, hi := 0, len(files)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if offset < files[mid].startOffset {
			hi = mid - 1
		} else if offset >= files[mid].endOffset {
			lo = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// readFileAt reads data from a file at a specific offset.
func (d *ModelDistributor) readFileAt(path string, offset, length int64, buf []byte) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, err
	}

	n, err := io.ReadFull(f, buf[:length])
	if err != nil {
		// Treat ALL errors as failures, including io.ErrUnexpectedEOF.
		// Partial reads would produce incorrect hashes.
		return nil, err
	}

	return buf[:n], nil
}

// GetDataDir returns the data directory path.
func (d *ModelDistributor) GetDataDir() string {
	return d.dataDir
}

// GetMetrics returns the metrics collector.
func (d *ModelDistributor) GetMetrics() *Metrics {
	return d.metrics
}
