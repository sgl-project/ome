package distributor

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics collects Prometheus metrics for P2P model distribution.
type Metrics struct {
	namespace string

	// Download metrics
	downloadTotal     *prometheus.CounterVec
	downloadDuration  *prometheus.HistogramVec
	downloadInFlight  prometheus.Gauge
	downloadBytesP2P  *prometheus.CounterVec
	downloadBytesHF   *prometheus.CounterVec
	downloadFailures  *prometheus.CounterVec
	verificationFails *prometheus.CounterVec

	// P2P specific metrics
	peersDiscovered  *prometheus.GaugeVec
	peersConnected   *prometheus.GaugeVec
	leasesAcquired   prometheus.Counter
	leasesWaiting    prometheus.Gauge
	seedingTorrents  prometheus.Gauge
	bytesUploaded    prometheus.Counter
	bytesDownloaded  prometheus.Counter
	p2pDownloadRatio prometheus.Gauge
	metainfoRequests *prometheus.CounterVec
	metainfoLatency  *prometheus.HistogramVec
}

// NewMetrics creates a new Metrics instance and registers all metrics.
func NewMetrics(namespace string) *Metrics {
	m := &Metrics{
		namespace: namespace,
	}

	m.downloadTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_total",
			Help:      "Total number of model downloads by source (p2p, hf, local)",
		},
		[]string{"source", "model_hash"},
	)

	m.downloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_duration_seconds",
			Help:      "Duration of model downloads in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
		},
		[]string{"source", "model_hash"},
	)

	m.downloadInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "downloads_in_flight",
			Help:      "Number of downloads currently in progress",
		},
	)

	m.downloadBytesP2P = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_bytes_p2p_total",
			Help:      "Total bytes downloaded via P2P",
		},
		[]string{"model_hash"},
	)

	m.downloadBytesHF = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_bytes_hf_total",
			Help:      "Total bytes downloaded from HuggingFace",
		},
		[]string{"model_hash"},
	)

	m.downloadFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_failures_total",
			Help:      "Total number of download failures by reason",
		},
		[]string{"model_hash", "reason"},
	)

	m.verificationFails = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "verification_failures_total",
			Help:      "Total number of SHA256 verification failures",
		},
		[]string{"model_hash"},
	)

	m.peersDiscovered = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "peers_discovered",
			Help:      "Number of peers discovered via DNS",
		},
		[]string{"model_hash"},
	)

	m.peersConnected = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "peers_connected",
			Help:      "Number of peers currently connected for P2P transfer",
		},
		[]string{"model_hash"},
	)

	m.leasesAcquired = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "leases_acquired_total",
			Help:      "Total number of download leases acquired",
		},
	)

	m.leasesWaiting = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "leases_waiting",
			Help:      "Number of pods waiting for lease to complete",
		},
	)

	m.seedingTorrents = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "seeding_torrents",
			Help:      "Number of models currently being seeded",
		},
	)

	m.bytesUploaded = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "bytes_uploaded_total",
			Help:      "Total bytes uploaded to peers",
		},
	)

	m.bytesDownloaded = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "bytes_downloaded_total",
			Help:      "Total bytes downloaded from peers",
		},
	)

	m.p2pDownloadRatio = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "download_ratio",
			Help:      "Ratio of P2P downloads to total downloads (higher is better)",
		},
	)

	m.metainfoRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "metainfo_requests_total",
			Help:      "Total number of metainfo requests",
		},
		[]string{"status"},
	)

	m.metainfoLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ome",
			Subsystem: "p2p",
			Name:      "metainfo_latency_seconds",
			Help:      "Latency of metainfo requests in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"model_hash"},
	)

	return m
}

// RecordDownloadStart records that a download has started.
func (m *Metrics) RecordDownloadStart(modelHash string) {
	m.downloadInFlight.Inc()
}

// RecordDownloadComplete records a successful download.
func (m *Metrics) RecordDownloadComplete(modelHash, source string, duration time.Duration) {
	m.downloadInFlight.Dec()
	m.downloadTotal.WithLabelValues(source, modelHash).Inc()
	m.downloadDuration.WithLabelValues(source, modelHash).Observe(duration.Seconds())
}

// RecordDownloadFailed records a failed download.
func (m *Metrics) RecordDownloadFailed(modelHash, reason string) {
	m.downloadInFlight.Dec()
	m.downloadFailures.WithLabelValues(modelHash, reason).Inc()
}

// RecordVerificationFailed records a SHA256 verification failure.
func (m *Metrics) RecordVerificationFailed(modelHash string) {
	m.verificationFails.WithLabelValues(modelHash).Inc()
}

// RecordPeersDiscovered records the number of peers discovered.
func (m *Metrics) RecordPeersDiscovered(modelHash string, count int) {
	m.peersDiscovered.WithLabelValues(modelHash).Set(float64(count))
}

// RecordPeersConnected records the number of connected peers.
func (m *Metrics) RecordPeersConnected(modelHash string, count int) {
	m.peersConnected.WithLabelValues(modelHash).Set(float64(count))
}

// RecordLeaseAcquired records a lease acquisition.
func (m *Metrics) RecordLeaseAcquired(modelHash string) {
	m.leasesAcquired.Inc()
}

// RecordWaitingForP2P records that this node is waiting for P2P availability.
func (m *Metrics) RecordWaitingForP2P(modelHash string) {
	m.leasesWaiting.Inc()
}

// RecordSeeding records that this node has started seeding a model.
func (m *Metrics) RecordSeeding(modelHash string) {
	m.seedingTorrents.Inc()
}

// RecordBytesUploaded records bytes uploaded to peers.
func (m *Metrics) RecordBytesUploaded(bytes int64) {
	m.bytesUploaded.Add(float64(bytes))
}

// RecordBytesDownloaded records bytes downloaded from peers.
func (m *Metrics) RecordBytesDownloaded(bytes int64) {
	m.bytesDownloaded.Add(float64(bytes))
}

// RecordP2PDownloadBytes records bytes downloaded via P2P for a specific model.
func (m *Metrics) RecordP2PDownloadBytes(modelHash string, bytes int64) {
	m.downloadBytesP2P.WithLabelValues(modelHash).Add(float64(bytes))
}

// RecordHFDownloadBytes records bytes downloaded from HuggingFace for a specific model.
func (m *Metrics) RecordHFDownloadBytes(modelHash string, bytes int64) {
	m.downloadBytesHF.WithLabelValues(modelHash).Add(float64(bytes))
}

// RecordMetainfoRequest records a metainfo request.
func (m *Metrics) RecordMetainfoRequest(status string) {
	m.metainfoRequests.WithLabelValues(status).Inc()
}

// RecordMetainfoLatency records the latency of a metainfo request.
func (m *Metrics) RecordMetainfoLatency(modelHash string, duration time.Duration) {
	m.metainfoLatency.WithLabelValues(modelHash).Observe(duration.Seconds())
}

// UpdateP2PRatio updates the P2P download ratio metric.
// This should be called periodically or after downloads complete.
func (m *Metrics) UpdateP2PRatio(p2pDownloads, totalDownloads int) {
	if totalDownloads > 0 {
		ratio := float64(p2pDownloads) / float64(totalDownloads)
		m.p2pDownloadRatio.Set(ratio)
	}
}
