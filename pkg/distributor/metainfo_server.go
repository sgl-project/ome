package distributor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"go.uber.org/zap"
)

// MetainfoServer serves torrent metainfo files to peers for P2P model discovery.
// Each pod runs this server to share information about models it's seeding.
type MetainfoServer struct {
	dataDir     string
	port        int
	distributor *ModelDistributor
	logger      *zap.SugaredLogger
	server      *http.Server
}

// NewMetainfoServer creates a new MetainfoServer.
func NewMetainfoServer(dataDir string, port int, distributor *ModelDistributor, logger *zap.SugaredLogger) *MetainfoServer {
	return &MetainfoServer{
		dataDir:     dataDir,
		port:        port,
		distributor: distributor,
		logger:      logger,
	}
}

// Start begins serving metainfo requests.
func (s *MetainfoServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metainfo/", s.handleMetainfo)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/models", s.handleListModels)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Infof("Starting metainfo server on port %d", s.port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *MetainfoServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.logger.Info("Shutting down metainfo server")
	return s.server.Shutdown(ctx)
}

// handleMetainfo serves torrent metainfo for a specific model.
// GET /metainfo/{modelHash}
func (s *MetainfoServer) handleMetainfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract model hash from path
	modelHash := strings.TrimPrefix(r.URL.Path, "/metainfo/")
	modelHash = filepath.Clean(modelHash)

	if modelHash == "" || modelHash == "." {
		http.Error(w, "Model hash required", http.StatusBadRequest)
		return
	}

	s.logger.Debugf("Metainfo request for model: %s", modelHash)

	// First check if we're actively seeding this model (has metainfo cached)
	if s.distributor != nil {
		mi, found := s.distributor.GetMetainfo(modelHash)
		if found {
			s.serveMetainfo(w, mi, modelHash)
			return
		}
	}

	// Fall back to generating metainfo from the file system
	// Validate path to prevent directory traversal attacks
	absBase, err := filepath.Abs(s.dataDir)
	if err != nil {
		s.logger.Errorf("Failed to resolve data directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	modelPath, err := filepath.Abs(filepath.Join(absBase, modelHash))
	if err != nil {
		s.logger.Errorf("Failed to resolve model path for %s: %v", modelHash, err)
		http.Error(w, "Invalid model path", http.StatusBadRequest)
		return
	}

	// Ensure the resolved path is within the data directory to prevent path traversal
	// Use filepath.Rel to get the relative path and verify it doesn't escape
	relPath, err := filepath.Rel(absBase, modelPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		s.logger.Warnf("Rejected metainfo request with path traversal attempt: %s", modelHash)
		http.Error(w, "Invalid model path", http.StatusBadRequest)
		return
	}

	// Construct safe path from validated components
	// This breaks the taint tracking chain by using only the validated relative path
	safePath := filepath.Join(absBase, relPath)

	// Check if the model exists
	if _, err := os.Stat(safePath); err != nil {
		s.logger.Debugf("Model not found at %s", safePath)
		http.NotFound(w, r)
		return
	}

	// Build metainfo from the safe path
	mi, err := s.buildMetainfo(safePath, modelHash)
	if err != nil {
		s.logger.Errorf("Failed to build metainfo for %s: %v", modelHash, err)
		http.Error(w, "Failed to build metainfo", http.StatusInternalServerError)
		return
	}

	s.serveMetainfo(w, mi, modelHash)
}

// buildMetainfo creates torrent metainfo from a file path.
func (s *MetainfoServer) buildMetainfo(path, name string) (*metainfo.MetaInfo, error) {
	info := metainfo.Info{
		PieceLength: 4 * 1024 * 1024, // 4MB pieces
		Name:        name,
	}

	if err := info.BuildFromFilePath(path); err != nil {
		return nil, fmt.Errorf("failed to build info from path: %w", err)
	}

	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal info: %w", err)
	}

	return &metainfo.MetaInfo{
		InfoBytes: infoBytes,
	}, nil
}

// serveMetainfo writes the metainfo to the response.
func (s *MetainfoServer) serveMetainfo(w http.ResponseWriter, mi *metainfo.MetaInfo, modelHash string) {
	w.Header().Set("Content-Type", "application/x-bittorrent")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.torrent\"", modelHash))

	if err := mi.Write(w); err != nil {
		s.logger.Errorf("Failed to write metainfo response: %v", err)
		// Can't change status code after Write started
	} else {
		s.logger.Debugf("Served metainfo for model %s", modelHash)
	}
}

// handleHealth returns the health status of the metainfo server.
// GET /health
func (s *MetainfoServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleStats returns P2P distribution statistics.
// GET /stats
func (s *MetainfoServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.distributor == nil {
		http.Error(w, "Distributor not available", http.StatusServiceUnavailable)
		return
	}

	stats := s.distributor.GetStats()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "active_torrents": %d,
  "total_bytes_uploaded": %d,
  "total_bytes_downloaded": %d,
  "active_peers": %d
}`, stats.ActiveTorrents, stats.TotalBytesUploaded, stats.TotalBytesDownloaded, stats.ActivePeers)
}

// handleListModels lists all models available for P2P distribution.
// GET /models
func (s *MetainfoServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// List models from data directory
	entries, err := filepath.Glob(filepath.Join(s.dataDir, "*"))
	if err != nil {
		s.logger.Errorf("Failed to list models: %v", err)
		http.Error(w, "Failed to list models", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[\n"))

	first := true
	for _, entry := range entries {
		name := filepath.Base(entry)
		if !first {
			w.Write([]byte(",\n"))
		}
		first = false

		isSeeding := false
		if s.distributor != nil {
			isSeeding = s.distributor.IsSeeding(name)
		}

		fmt.Fprintf(w, `  {"hash": %q, "seeding": %t}`, name, isSeeding)
	}

	w.Write([]byte("\n]"))
}

// ServeWithContext runs the metainfo server until the context is cancelled.
func (s *MetainfoServer) ServeWithContext(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
