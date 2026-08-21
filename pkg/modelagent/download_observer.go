package modelagent

import (
	"os"
	"time"

	"go.uber.org/zap"

	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

type modelDownloadObserver struct {
	metrics                       *Metrics
	logger                        *zap.SugaredLogger
	downloadID                    string
	modelType                     string
	modelNamespace                string
	modelName                     string
	nodeName                      string
	objectConcurrency             int
	multipartConcurrency          int
	modelFileWriteConcurrency     int
	modelFileWriteBufferSizeBytes int
	modelVerificationConcurrency  int
}

func newModelDownloadObserver(gopher *Gopher, task *GopherTask) *modelDownloadObserver {
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)
	return &modelDownloadObserver{
		metrics:                       gopher.metrics,
		logger:                        gopher.logger,
		downloadID:                    task.DownloadID,
		modelType:                     modelType,
		modelNamespace:                namespace,
		modelName:                     name,
		nodeName:                      os.Getenv("NODE_NAME"),
		objectConcurrency:             gopher.concurrency,
		multipartConcurrency:          gopher.multipartConcurrency,
		modelFileWriteConcurrency:     gopher.modelFileWriteConcurrency,
		modelFileWriteBufferSizeBytes: gopher.modelFileWriteBufferSizeBytes,
		modelVerificationConcurrency:  gopher.effectiveModelVerificationConcurrency(),
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
		if observation.WriteStats != nil {
			stats := observation.WriteStats
			o.metrics.ObserveDownloadWriteStats(string(observation.Outcome), stats)
		}
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
		"model_file_write_concurrency", o.modelFileWriteConcurrency,
		"model_file_write_buffer_size_bytes", o.modelFileWriteBufferSizeBytes,
		"model_verification_concurrency", o.modelVerificationConcurrency,
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
	if observation.WriteStats != nil {
		stats := observation.WriteStats
		fields = append(fields,
			"write_calls", stats.Calls,
			"write_duration_ms", float64(stats.Duration.Microseconds())/1000,
			"write_limiter_wait_ms", float64(stats.LimiterWaitDuration.Microseconds())/1000,
			"write_limiter_wait_calls", stats.LimiterWaitCalls,
			"max_write_duration_ms", float64(stats.MaxDuration.Microseconds())/1000,
			"max_write_limiter_wait_ms", float64(stats.MaxLimiterWaitDuration.Microseconds())/1000,
			"max_inflight_writes", stats.MaxInflightWrites,
			"max_waiting_writers", stats.MaxWaitingWriters,
			"write_buffer_size_bytes", stats.BufferSizeBytes,
			"min_write_request_bytes", stats.MinRequestBytes,
			"max_write_request_bytes", stats.MaxRequestBytes,
			"write_calls_up_to_16_kib", stats.CallsUpTo16KiB,
			"write_calls_16_kib_to_64_kib", stats.Calls16KiBTo64KiB,
			"write_calls_64_kib_to_256_kib", stats.Calls64KiBTo256KiB,
			"write_calls_256_kib_to_1_mib", stats.Calls256KiBTo1MiB,
			"write_calls_over_1_mib", stats.CallsOver1MiB,
			"write_duration_calls_up_to_1_ms", stats.WriteDurationBuckets.UpTo1ms,
			"write_duration_calls_1_ms_to_5_ms", stats.WriteDurationBuckets.From1To5ms,
			"write_duration_calls_5_ms_to_20_ms", stats.WriteDurationBuckets.From5To20ms,
			"write_duration_calls_20_ms_to_100_ms", stats.WriteDurationBuckets.From20To100ms,
			"write_duration_calls_over_100_ms", stats.WriteDurationBuckets.Over100ms,
			"write_limiter_wait_calls_up_to_1_ms", stats.LimiterWaitDurationBuckets.UpTo1ms,
			"write_limiter_wait_calls_1_ms_to_5_ms", stats.LimiterWaitDurationBuckets.From1To5ms,
			"write_limiter_wait_calls_5_ms_to_20_ms", stats.LimiterWaitDurationBuckets.From5To20ms,
			"write_limiter_wait_calls_20_ms_to_100_ms", stats.LimiterWaitDurationBuckets.From20To100ms,
			"write_limiter_wait_calls_over_100_ms", stats.LimiterWaitDurationBuckets.Over100ms,
		)
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
