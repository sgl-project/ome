package modelagent

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetrics_RegistersMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Test that custom counters are registered and can be incremented
	metrics.modelDownloadsSuccessTotal.WithLabelValues("testtype", "testns", "testmodel").Inc()
	if got := testutil.ToFloat64(metrics.modelDownloadsSuccessTotal.WithLabelValues("testtype", "testns", "testmodel")); got != 1 {
		t.Errorf("modelDownloadsSuccessTotal did not increment, got = %v, want = 1", got)
	}

	metrics.modelDownloadsFailedTotal.WithLabelValues("testtype", "testns", "testmodel").Add(2)
	if got := testutil.ToFloat64(metrics.modelDownloadsFailedTotal.WithLabelValues("testtype", "testns", "testmodel")); got != 2 {
		t.Errorf("modelDownloadsFailedTotal did not increment, got = %v, want = 2", got)
	}

	// Test that histograms record observations
	metrics.modelDownloadDuration.WithLabelValues("testtype", "testns", "testmodel").Observe(1.23)
	count := testutil.CollectAndCount(metrics.modelDownloadDuration)
	if count == 0 {
		t.Error("modelDownloadDuration did not record observation")
	}
}

func TestDownloadPhaseMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	metrics.ObserveDownloadPhase("get_object_body_read", "success", 2*time.Second, 4096)

	if got := testutil.ToFloat64(metrics.modelDownloadPhaseBytes.WithLabelValues("get_object_body_read", "success")); got != 4096 {
		t.Errorf("modelDownloadPhaseBytes = %v, want 4096", got)
	}
	if got := testutil.CollectAndCount(metrics.modelDownloadPhaseDuration); got != 1 {
		t.Errorf("modelDownloadPhaseDuration metric families = %d, want 1", got)
	}
}

func TestDownloadWriteCallMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	metrics.ObserveDownloadWriteCalls("success", 250*time.Millisecond, 3, 4, 5, 6, 7)

	wants := map[string]float64{
		"up_to_16_kib":      3,
		"16_kib_to_64_kib":  4,
		"64_kib_to_256_kib": 5,
		"256_kib_to_1_mib":  6,
		"over_1_mib":        7,
	}
	for sizeRange, want := range wants {
		got := testutil.ToFloat64(metrics.modelDownloadWriteCalls.WithLabelValues("success", sizeRange))
		if got != want {
			t.Errorf("modelDownloadWriteCalls[%s] = %v, want %v", sizeRange, got, want)
		}
	}
	if got := testutil.CollectAndCount(metrics.modelDownloadWriteMaxDuration); got != 1 {
		t.Errorf("modelDownloadWriteMaxDuration metric families = %d, want 1", got)
	}
}

func TestDownloadPhaseDurationBucketsCoverLargeModels(t *testing.T) {
	buckets := downloadPhaseDurationBuckets()
	if got := buckets[len(buckets)-1]; got < 1800 {
		t.Errorf("largest download phase duration bucket = %v, want at least 1800 seconds", got)
	}
}

func TestGoRuntimeMetrics_AreSet(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = NewMetrics(reg)

	// Wait for the goroutine to update runtime metrics
	time.Sleep(200 * time.Millisecond)

	metricNames := []string{
		"go_goroutines_current",
		"go_memory_alloc_bytes",
		"go_memory_heap_alloc_bytes",
		"go_memory_heap_sys_bytes",
		"go_memory_stack_sys_bytes",
	}

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	for _, name := range metricNames {
		found := false
		for _, mf := range metricFamilies {
			if mf.GetName() == name {
				found = true
				if len(mf.Metric) == 0 || mf.Metric[0].GetGauge().GetValue() < 0 {
					t.Errorf("metric %s has invalid value", name)
				}
				break
			}
		}
		if !found {
			t.Errorf("metric %s not found", name)
		}
	}
}
