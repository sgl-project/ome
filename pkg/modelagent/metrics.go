package modelagent

import (
	"net/http"
	"runtime"
	"time"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/ociobjectstore"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is a struct that contains all metrics for the model-agent
type Metrics struct {
	// Counter metrics
	modelDownloadsSuccessTotal *prometheus.CounterVec
	modelDownloadsFailedTotal  *prometheus.CounterVec
	modelVerificationsTotal    *prometheus.CounterVec
	mdChecksumsFailedTotal     *prometheus.CounterVec
	rateLimitCounter           *prometheus.CounterVec

	// Histogram metrics
	modelDownloadDuration                    *prometheus.HistogramVec
	modelVerificationDuration                prometheus.Histogram
	modelDownloadBytesTransferred            *prometheus.CounterVec
	modelDownloadPhaseDuration               *prometheus.HistogramVec
	modelDownloadPhaseBytes                  *prometheus.CounterVec
	modelDownloadWriteCalls                  *prometheus.CounterVec
	modelDownloadWriteDurationCalls          *prometheus.CounterVec
	modelDownloadWriteLimiterWaitCalls       *prometheus.CounterVec
	modelDownloadWriteMaxDuration            *prometheus.HistogramVec
	modelDownloadWriteMaxLimiterWaitDuration *prometheus.HistogramVec
	modelDownloadReadCalls                   *prometheus.CounterVec
	modelDownloadReadDurationCalls           *prometheus.CounterVec
	modelDownloadReadShortCalls              *prometheus.CounterVec
	modelDownloadReadZeroByteCalls           *prometheus.CounterVec
	modelDownloadReadMaxDuration             *prometheus.HistogramVec
	rateLimitWaitDuration                    *prometheus.HistogramVec

	// Go runtime metrics
	goGoroutines      prometheus.Gauge
	goThreads         prometheus.Gauge
	goHeapObjects     prometheus.Gauge
	goGCDuration      prometheus.Histogram
	goMemoryAlloc     prometheus.Gauge
	goMemoryHeapAlloc prometheus.Gauge
	goMemoryHeapSys   prometheus.Gauge
	goMemoryStackSys  prometheus.Gauge
	goGCCount         prometheus.Counter
}

// NewMetrics creates a new Metrics struct with initialized Prometheus metrics
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	// Manual Go metrics for more detailed tracking
	goGoroutines := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines_current",
		Help: "Current number of goroutines",
	})

	goThreads := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_threads_current",
		Help: "Current number of OS threads",
	})

	goHeapObjects := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_heap_objects_current",
		Help: "Current number of heap objects",
	})

	goGCDuration := promauto.With(registerer).NewHistogram(prometheus.HistogramOpts{
		Name:    "go_gc_pause_duration_seconds_custom",
		Help:    "Custom: GC pause duration in seconds",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // From 100us to ~3s
	})

	// Memory metrics
	goMemoryAlloc := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_memory_alloc_bytes",
		Help: "Currently allocated memory in bytes",
	})

	goMemoryHeapAlloc := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_memory_heap_alloc_bytes",
		Help: "Heap memory allocated in bytes",
	})

	goMemoryHeapSys := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_memory_heap_sys_bytes",
		Help: "Heap memory obtained from system in bytes",
	})

	goMemoryStackSys := promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
		Name: "go_memory_stack_sys_bytes",
		Help: "Stack memory obtained from system in bytes",
	})

	goGCCount := promauto.With(registerer).NewCounter(prometheus.CounterOpts{
		Name: "go_gc_count_total",
		Help: "Total number of garbage collections",
	})

	// Start a goroutine to periodically update Go runtime metrics
	go func() {
		memStats := &runtime.MemStats{}
		var lastGC uint32

		for {
			runtime.ReadMemStats(memStats)

			// Update metrics
			goGoroutines.Set(float64(runtime.NumGoroutine()))
			goThreads.Set(float64(runtime.NumCPU()))
			goHeapObjects.Set(float64(memStats.HeapObjects))

			// Memory metrics
			goMemoryAlloc.Set(float64(memStats.Alloc))
			goMemoryHeapAlloc.Set(float64(memStats.HeapAlloc))
			goMemoryHeapSys.Set(float64(memStats.HeapSys))
			goMemoryStackSys.Set(float64(memStats.StackSys))

			// GC metrics - only record if a new GC has occurred
			if memStats.NumGC > lastGC {
				// Count how many new GCs have happened
				newGCs := memStats.NumGC - lastGC
				goGCCount.Add(float64(newGCs))
				lastGC = memStats.NumGC

				// Record the most recent GC pause time
				if newGCs > 0 {
					// Calculate index of the most recent GC pause
					pauseIndex := int(memStats.NumGC % 256)
					if pauseIndex == 0 {
						pauseIndex = 255
					} else {
						pauseIndex--
					}
					goGCDuration.Observe(float64(memStats.PauseNs[pauseIndex]) / 1e9)
				}
			}

			time.Sleep(15 * time.Second)
		}
	}()

	return &Metrics{
		modelDownloadsSuccessTotal: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_downloads_success_total",
				Help: "The total number of successful model downloads",
			},
			[]string{"model_type", "namespace", "name"},
		),
		modelDownloadsFailedTotal: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_downloads_failed_total",
				Help: "The total number of failed model downloads",
			},
			[]string{"model_type", "namespace", "name"},
		),
		modelVerificationsTotal: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_verifications_total",
				Help: "The total number of model verification attempts",
			},
			[]string{"model_type", "namespace", "name", "result"},
		),
		mdChecksumsFailedTotal: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_md5_checksum_failed_total",
				Help: "The total number of MD5 checksum failures",
			},
			[]string{"model_type", "namespace", "name"},
		),
		rateLimitCounter: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_rate_limit_total",
				Help: "The total number of rate limit (429) responses encountered",
			},
			[]string{"model_type", "namespace", "name"},
		),
		modelDownloadDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_download_duration_seconds",
				Help:    "The duration of model downloads in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // From 0.1s to ~1.7m
			},
			[]string{"model_type", "namespace", "name"},
		),
		modelVerificationDuration: promauto.With(registerer).NewHistogram(prometheus.HistogramOpts{
			Name:    "model_agent_verification_duration_seconds",
			Help:    "The duration of model verifications in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // From 0.1s to ~1.7m
		}),
		modelDownloadBytesTransferred: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_bytes_total",
				Help: "The total bytes transferred while downloading models",
			},
			[]string{"model_type", "namespace", "name"},
		),
		modelDownloadPhaseDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_download_phase_duration_seconds",
				Help:    "The duration of bounded model download phases in seconds",
				Buckets: downloadPhaseDurationBuckets(),
			},
			[]string{"phase", "outcome"},
		),
		modelDownloadPhaseBytes: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_phase_bytes_total",
				Help: "The total bytes processed by bounded model download phases",
			},
			[]string{"phase", "outcome"},
		),
		modelDownloadWriteCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_write_calls_total",
				Help: "The total direct model file write calls grouped by requested write size",
			},
			[]string{"outcome", "size_range"},
		),
		modelDownloadWriteDurationCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_write_duration_calls_total",
				Help: "The total direct model file write calls grouped by write duration",
			},
			[]string{"outcome", "duration_range"},
		),
		modelDownloadWriteLimiterWaitCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_write_limiter_wait_calls_total",
				Help: "The total direct model file write limiter acquisitions grouped by wait duration",
			},
			[]string{"outcome", "duration_range"},
		),
		modelDownloadWriteMaxDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_download_write_max_duration_seconds",
				Help:    "The longest direct model file write call observed in each downloaded part",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 18), // 100us to ~13s
			},
			[]string{"outcome"},
		),
		modelDownloadWriteMaxLimiterWaitDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_download_write_max_limiter_wait_seconds",
				Help:    "The longest direct model file write limiter wait observed in each downloaded part",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 18), // 100us to ~13s
			},
			[]string{"outcome"},
		),
		modelDownloadReadCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_http_read_calls_total",
				Help: "The total HTTP body read calls grouped by returned read size",
			},
			[]string{"outcome", "size_range"},
		),
		modelDownloadReadDurationCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_http_read_duration_calls_total",
				Help: "The total HTTP body read calls grouped by read duration",
			},
			[]string{"outcome", "duration_range"},
		),
		modelDownloadReadShortCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_http_short_read_calls_total",
				Help: "The total HTTP body reads that returned data shorter than the requested buffer",
			},
			[]string{"outcome"},
		),
		modelDownloadReadZeroByteCalls: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "model_agent_download_http_zero_byte_read_calls_total",
				Help: "The total HTTP body reads that returned zero bytes",
			},
			[]string{"outcome"},
		),
		modelDownloadReadMaxDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_download_http_read_max_duration_seconds",
				Help:    "The longest HTTP body read call observed in each downloaded part",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 18), // 100us to ~13s
			},
			[]string{"outcome"},
		),
		rateLimitWaitDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "model_agent_rate_limit_wait_seconds",
				Help:    "The duration waited due to rate limits in seconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // From 1s to ~17m
			},
			[]string{"model_type", "namespace", "name"},
		),
		// Store Go runtime metrics
		goGoroutines:      goGoroutines,
		goThreads:         goThreads,
		goHeapObjects:     goHeapObjects,
		goGCDuration:      goGCDuration,
		goMemoryAlloc:     goMemoryAlloc,
		goMemoryHeapAlloc: goMemoryHeapAlloc,
		goMemoryHeapSys:   goMemoryHeapSys,
		goMemoryStackSys:  goMemoryStackSys,
		goGCCount:         goGCCount,
	}
}

// ObserveDownloadReadStats records bounded, per-part aggregates without
// performing one Prometheus operation for every HTTP body Read call.
func (m *Metrics) ObserveDownloadReadStats(outcome string, stats *ociobjectstore.ReadStats) {
	if stats == nil {
		return
	}
	m.modelDownloadReadMaxDuration.WithLabelValues(outcome).Observe(stats.MaxDuration.Seconds())
	if stats.ShortReadCalls > 0 {
		m.modelDownloadReadShortCalls.WithLabelValues(outcome).Add(float64(stats.ShortReadCalls))
	}
	if stats.ZeroByteReadCalls > 0 {
		m.modelDownloadReadZeroByteCalls.WithLabelValues(outcome).Add(float64(stats.ZeroByteReadCalls))
	}
	counts := []struct {
		rangeName string
		count     int64
	}{
		{rangeName: "up_to_16_kib", count: stats.CallsUpTo16KiB},
		{rangeName: "16_kib_to_64_kib", count: stats.Calls16KiBTo64KiB},
		{rangeName: "64_kib_to_256_kib", count: stats.Calls64KiBTo256KiB},
		{rangeName: "256_kib_to_1_mib", count: stats.Calls256KiBTo1MiB},
		{rangeName: "over_1_mib", count: stats.CallsOver1MiB},
	}
	for _, sizeRange := range counts {
		if sizeRange.count > 0 {
			m.modelDownloadReadCalls.WithLabelValues(outcome, sizeRange.rangeName).Add(float64(sizeRange.count))
		}
	}
	observeDurationBucketCounts(m.modelDownloadReadDurationCalls, outcome, stats.ReadDurationBuckets)
}

func downloadPhaseDurationBuckets() []float64 {
	return []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1200, 1800}
}

// RecordSuccessfulDownload records a successful model download
func (m *Metrics) RecordSuccessfulDownload(modelType, namespace, name string) {
	m.modelDownloadsSuccessTotal.WithLabelValues(modelType, namespace, name).Inc()
}

// RecordFailedDownload records a failed model download
func (m *Metrics) RecordFailedDownload(modelType, namespace, name, errorType string) {
	m.modelDownloadsFailedTotal.WithLabelValues(modelType, namespace, name).Inc()
}

// RecordVerification records a model verification
func (m *Metrics) RecordVerification(modelType, namespace, name string, success bool) {
	result := "success"
	if !success {
		result = "failure"
		m.mdChecksumsFailedTotal.WithLabelValues(modelType, namespace, name).Inc()
	}
	m.modelVerificationsTotal.WithLabelValues(modelType, namespace, name, result).Inc()
}

// ObserveDownloadDuration records the duration of a model download
func (m *Metrics) ObserveDownloadDuration(modelType, namespace, name string, duration time.Duration) {
	m.modelDownloadDuration.WithLabelValues(modelType, namespace, name).Observe(duration.Seconds())
}

// ObserveVerificationDuration records the duration of a model verification
func (m *Metrics) ObserveVerificationDuration(duration time.Duration) {
	m.modelVerificationDuration.Observe(duration.Seconds())
}

// RecordBytesTransferred records the number of bytes transferred during a download
func (m *Metrics) RecordBytesTransferred(modelType, namespace, name string, bytes int64) {
	m.modelDownloadBytesTransferred.WithLabelValues(modelType, namespace, name).Add(float64(bytes))
}

// ObserveDownloadPhase records a bounded-cardinality pipeline phase. Model,
// object, part, and run identifiers are intentionally excluded from labels.
func (m *Metrics) ObserveDownloadPhase(phase, outcome string, duration time.Duration, bytes int64) {
	m.modelDownloadPhaseDuration.WithLabelValues(phase, outcome).Observe(duration.Seconds())
	if bytes > 0 {
		m.modelDownloadPhaseBytes.WithLabelValues(phase, outcome).Add(float64(bytes))
	}
}

// ObserveDownloadWriteStats records bounded, per-part aggregates without
// performing one Prometheus operation for every WriteAt call.
func (m *Metrics) ObserveDownloadWriteStats(outcome string, stats *ociobjectstore.WriteStats) {
	if stats == nil {
		return
	}
	m.modelDownloadWriteMaxDuration.WithLabelValues(outcome).Observe(stats.MaxDuration.Seconds())
	if stats.LimiterWaitCalls > 0 {
		m.modelDownloadWriteMaxLimiterWaitDuration.WithLabelValues(outcome).Observe(stats.MaxLimiterWaitDuration.Seconds())
	}
	counts := []struct {
		rangeName string
		count     int64
	}{
		{rangeName: "up_to_16_kib", count: stats.CallsUpTo16KiB},
		{rangeName: "16_kib_to_64_kib", count: stats.Calls16KiBTo64KiB},
		{rangeName: "64_kib_to_256_kib", count: stats.Calls64KiBTo256KiB},
		{rangeName: "256_kib_to_1_mib", count: stats.Calls256KiBTo1MiB},
		{rangeName: "over_1_mib", count: stats.CallsOver1MiB},
	}
	for _, sizeRange := range counts {
		if sizeRange.count > 0 {
			m.modelDownloadWriteCalls.WithLabelValues(outcome, sizeRange.rangeName).Add(float64(sizeRange.count))
		}
	}
	observeDurationBucketCounts(m.modelDownloadWriteDurationCalls, outcome, stats.WriteDurationBuckets)
	observeDurationBucketCounts(m.modelDownloadWriteLimiterWaitCalls, outcome, stats.LimiterWaitDurationBuckets)
}

func observeDurationBucketCounts(metric *prometheus.CounterVec, outcome string, counts ociobjectstore.DurationBucketCounts) {
	buckets := []struct {
		rangeName string
		count     int64
	}{
		{rangeName: "up_to_1_ms", count: counts.UpTo1ms},
		{rangeName: "1_ms_to_5_ms", count: counts.From1To5ms},
		{rangeName: "5_ms_to_20_ms", count: counts.From5To20ms},
		{rangeName: "20_ms_to_100_ms", count: counts.From20To100ms},
		{rangeName: "over_100_ms", count: counts.Over100ms},
	}
	for _, bucket := range buckets {
		if bucket.count > 0 {
			metric.WithLabelValues(outcome, bucket.rangeName).Add(float64(bucket.count))
		}
	}
}

// RecordGCDuration records the duration of a garbage collection cycle
func (m *Metrics) RecordGCDuration(duration time.Duration) {
	m.goGCDuration.Observe(duration.Seconds())
}

// RecordRateLimit records a rate limit event
func (m *Metrics) RecordRateLimit(modelType, namespace, name string, waitDuration time.Duration) {
	m.rateLimitCounter.WithLabelValues(modelType, namespace, name).Inc()
	m.rateLimitWaitDuration.WithLabelValues(modelType, namespace, name).Observe(waitDuration.Seconds())
}

// RegisterMetricsHandler registers the metrics HTTP handler
func RegisterMetricsHandler(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
}

// GetModelTypeNamespaceAndName extracts the model type, namespace, and name from a gopher task
func GetModelTypeNamespaceAndName(task *GopherTask) (string, string, string) {
	if task.BaseModel != nil {
		return constants.BaseModel, task.BaseModel.Namespace, task.BaseModel.Name
	} else if task.ClusterBaseModel != nil {
		return constants.ClusterBaseModel, "", task.ClusterBaseModel.Name
	}
	return "unknown", "unknown", "unknown"
}
