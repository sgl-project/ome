package distributor

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
		WriteTimeout: 10 * time.Minute, // Large metainfo files (30+ MB for 1TB models) need more time
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

	// First check if we're actively seeding this model (has metainfo in memory)
	if s.distributor != nil {
		mi, found := s.distributor.GetMetainfo(modelHash)
		if found {
			s.serveMetainfo(w, mi, modelHash)
			return
		}

		// Check for cached .torrent file on disk
		// This is much faster than regenerating (file read vs. hashing entire model)
		mi, err := s.distributor.LoadMetainfoFromFile(modelHash)
		if err != nil {
			s.logger.Warnf("Failed to load cached metainfo for %s: %v", modelHash, err)
		} else if mi != nil {
			s.serveMetainfo(w, mi, modelHash)
			return
		}
	}

	// Don't try to regenerate metainfo - it's too slow for large models (minutes for 600GB+)
	// Only pods that are actively seeding (and have the metainfo cached) should serve it
	s.logger.Debugf("Metainfo not available for %s (not seeding and no cache)", modelHash)
	http.NotFound(w, r)
}

// serveMetainfo writes the metainfo to the response.
// It serializes to a buffer first to set Content-Length for reliable transfer of large metainfo files.
func (s *MetainfoServer) serveMetainfo(w http.ResponseWriter, mi *metainfo.MetaInfo, modelHash string) {
	// Serialize to buffer first to know the exact size
	// This is important for large models (e.g., 1TB model = ~5MB metainfo)
	var buf bytes.Buffer
	if err := mi.Write(&buf); err != nil {
		s.logger.Errorf("Failed to serialize metainfo: %v", err)
		http.Error(w, "Failed to serialize metainfo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-bittorrent")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.torrent\"", modelHash))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	if _, err := w.Write(buf.Bytes()); err != nil {
		s.logger.Errorf("Failed to write metainfo response: %v", err)
		// Can't change status code after Write started
	} else {
		s.logger.Debugf("Served metainfo for model %s (%d bytes)", modelHash, buf.Len())
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
