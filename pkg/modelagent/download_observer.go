package modelagent

import (
	"os"
	"time"

	"go.uber.org/zap"

	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

type modelDownloadObserver struct {
	metrics              *Metrics
	logger               *zap.SugaredLogger
	downloadID           string
	modelType            string
	modelNamespace       string
	modelName            string
	nodeName             string
	objectConcurrency    int
	multipartConcurrency int
}

func newModelDownloadObserver(gopher *Gopher, task *GopherTask) *modelDownloadObserver {
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)
	return &modelDownloadObserver{
		metrics:              gopher.metrics,
		logger:               gopher.logger,
		downloadID:           task.DownloadID,
		modelType:            modelType,
		modelNamespace:       namespace,
		modelName:            name,
		nodeName:             os.Getenv("NODE_NAME"),
		objectConcurrency:    gopher.concurrency,
		multipartConcurrency: gopher.multipartConcurrency,
	}
}

func (o *modelDownloadObserver) ObserveDownloadPhase(observation ociobjectstore.DownloadObservation) {
	if o == nil {
		return
	}
	if o.metrics != nil {
		o.metrics.ObserveDownloadPhase(
			string(observation.Phase),
			string(observation.Outcome),
			observation.Duration,
			observation.Bytes,
		)
	}
	if o.logger == nil {
		return
	}

	fields := []interface{}{
		"event", "model_download_phase_completed",
		"phase", observation.Phase,
		"outcome", observation.Outcome,
		"duration_ms", float64(observation.Duration.Microseconds()) / 1000,
		"bytes", observation.Bytes,
		"download_id", o.downloadID,
		"model_type", o.modelType,
		"model_namespace", o.modelNamespace,
		"model_name", o.modelName,
		"node", o.nodeName,
		"object_concurrency", o.objectConcurrency,
		"multipart_concurrency", o.multipartConcurrency,
	}
	if observation.ObjectName != "" {
		fields = append(fields, "object", observation.ObjectName)
	}
	if observation.HasPart {
		fields = append(fields, "part_number", observation.PartNumber)
	}
	if observation.Attempt > 0 {
		fields = append(fields, "attempt", observation.Attempt)
	}
	if observation.ChunkSize > 0 {
		fields = append(fields, "chunk_size_bytes", observation.ChunkSize)
	}
	if observation.Err != nil {
		fields = append(fields, "error", observation.Err)
	}
	switch {
	case observation.Outcome == ociobjectstore.DownloadOutcomeError:
		o.logger.Warnw("Model download phase completed", fields...)
	case observation.ObjectName != "" || observation.HasPart:
		// Per-object and per-part events can be numerous for large models. Keep
		// their high-cardinality detail available without increasing normal
		// production log volume.
		o.logger.Debugw("Model download phase completed", fields...)
	default:
		o.logger.Infow("Model download phase completed", fields...)
	}
}

func (s *Gopher) observeTaskDownloadPhase(
	task *GopherTask,
	phase ociobjectstore.DownloadPhase,
	startedAt time.Time,
	outcome ociobjectstore.DownloadOutcome,
	bytes int64,
	err error,
) {
	newModelDownloadObserver(s, task).ObserveDownloadPhase(ociobjectstore.DownloadObservation{
		Phase:    phase,
		Duration: time.Since(startedAt),
		Bytes:    bytes,
		Outcome:  outcome,
		Err:      err,
	})
}
