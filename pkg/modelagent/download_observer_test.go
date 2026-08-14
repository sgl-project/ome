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
		Phase:      ociobjectstore.PhaseObjectToPartCopy,
		Duration:   time.Second,
		Bytes:      2048,
		Outcome:    ociobjectstore.DownloadOutcomeSuccess,
		ObjectName: "model.bin",
		PartNumber: 7,
		HasPart:    true,
		Attempt:    1,
		ChunkSize:  4096,
	})

	got := testutil.ToFloat64(metrics.modelDownloadPhaseBytes.WithLabelValues("object_to_part_file_copy", "success"))
	if got != 2048 {
		t.Fatalf("modelDownloadPhaseBytes = %v, want 2048", got)
	}
}
