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
			Calls:           3,
			Bytes:           2048,
			Duration:        time.Second,
			MaxDuration:     500 * time.Millisecond,
			MinRequestBytes: 512,
			MaxRequestBytes: 1024,
			CallsUpTo16KiB:  3,
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
}
