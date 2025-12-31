// Package distributor implements P2P model distribution using BitTorrent protocol.
// It provides BitTorrent client functionality for peer-to-peer file transfer.
// Lease coordination and HuggingFace integration are handled by the model-agent.
package distributor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
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
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
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

	d.logger.Infof("Discovered %d peers for model %s", len(peers), modelHash)
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
		// Add peers after torrent info is available (required for proper handshaking)
		peerInfos := make([]torrent.PeerInfo, len(peers))
		for i, p := range peers {
			peerInfos[i] = p
		}
		t.AddPeers(peerInfos)
		d.logger.Infof("Added %d peers for model %s, starting download", len(peers), modelHash)

		t.DownloadAll()
		if !d.waitForComplete(ctx, t) {
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

		// Move downloaded files to destination
		if err := os.Rename(downloadPath, destPath); err != nil {
			t.Drop()
			return fmt.Errorf("failed to move downloaded files to destination: %w", err)
		}
		d.logger.Infof("Moved downloaded model from %s to %s", downloadPath, destPath)

		// Create symlink from hash path to destination for continued seeding
		if err := os.Symlink(destPath, downloadPath); err != nil {
			d.logger.Warnf("Failed to create symlink for seeding: %v", err)
			// Don't fail the download, seeding is optional
		}

		// Store for seeding
		d.torrentsMu.Lock()
		d.activeTorrents[modelHash] = t
		d.torrentsMu.Unlock()

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
		d.logger.Infof("Stopped seeding model %s", modelHash)
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

		// Use per-request timeout context (10s per peer)
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
			continue
		}

		d.logger.Infof("Fetched metainfo for %s from peer %s", modelHash, peer.Addr)
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

// createMetainfo builds a torrent metainfo for the given path.
func (d *ModelDistributor) createMetainfo(path, name string) (*metainfo.MetaInfo, error) {
	info := metainfo.Info{
		PieceLength: 4 * 1024 * 1024, // 4MB pieces
	}

	if err := info.BuildFromFilePath(path); err != nil {
		return nil, fmt.Errorf("failed to build info: %w", err)
	}

	// Set Name after BuildFromFilePath because BuildFromFilePath overwrites it
	// with filepath.Base(path). We need the modelHash as the name so the torrent
	// client looks for files at {dataDir}/{modelHash}/ where we create the symlink.
	info.Name = name

	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal info: %w", err)
	}

	return &metainfo.MetaInfo{InfoBytes: infoBytes}, nil
}

// GetDataDir returns the data directory path.
func (d *ModelDistributor) GetDataDir() string {
	return d.dataDir
}

// GetMetrics returns the metrics collector.
func (d *ModelDistributor) GetMetrics() *Metrics {
	return d.metrics
}
