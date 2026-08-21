package modelagent

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

func TestModelDownloadObserverRecordsBoundedMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	gopher := &Gopher{
		metrics:              metrics,
		logger:               zap.NewNop().Sugar(),
		concurrency:          4,
		multipartConcurrency: 4,
	}
	task := &GopherTask{
		DownloadID: "test-download-id",
		BaseModel: &v1beta1.BaseModel{ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "test-namespace",
		}},
	}

	observer := newModelDownloadObserver(gopher, task)
	observer.ObserveDownloadPhase(ociobjectstore.DownloadObservation{
		Phase:      ociobjectstore.PhaseModelFileWrite,
		Duration:   time.Second,
		Bytes:      2048,
		Outcome:    ociobjectstore.DownloadOutcomeSuccess,
		ObjectName: "model.bin",
		PartNumber: 7,
		HasPart:    true,
		Attempt:    1,
		ChunkSize:  4096,
		WriteStats: &ociobjectstore.WriteStats{
			Calls:                  3,
			Bytes:                  2048,
			Duration:               time.Second,
			LimiterWaitCalls:       3,
			LimiterWaitDuration:    25 * time.Millisecond,
			MaxDuration:            500 * time.Millisecond,
			MaxLimiterWaitDuration: 20 * time.Millisecond,
			MinRequestBytes:        512,
			MaxRequestBytes:        1024,
			CallsUpTo16KiB:         3,
			WriteDurationBuckets: ociobjectstore.DurationBucketCounts{
				From20To100ms: 3,
			},
			LimiterWaitDurationBuckets: ociobjectstore.DurationBucketCounts{
				UpTo1ms:     2,
				From5To20ms: 1,
			},
		},
	})

	got := testutil.ToFloat64(metrics.modelDownloadPhaseBytes.WithLabelValues("model_file_write", "success"))
	if got != 2048 {
		t.Fatalf("modelDownloadPhaseBytes = %v, want 2048", got)
	}
	writeCalls := testutil.ToFloat64(metrics.modelDownloadWriteCalls.WithLabelValues("success", "up_to_16_kib"))
	if writeCalls != 3 {
		t.Fatalf("modelDownloadWriteCalls = %v, want 3", writeCalls)
	}
	writeDurationCalls := testutil.ToFloat64(metrics.modelDownloadWriteDurationCalls.WithLabelValues("success", "20_ms_to_100_ms"))
	if writeDurationCalls != 3 {
		t.Fatalf("modelDownloadWriteDurationCalls = %v, want 3", writeDurationCalls)
	}
	waitCalls := testutil.ToFloat64(metrics.modelDownloadWriteLimiterWaitCalls.WithLabelValues("success", "up_to_1_ms"))
	if waitCalls != 2 {
		t.Fatalf("modelDownloadWriteLimiterWaitCalls = %v, want 2", waitCalls)
	}
}
