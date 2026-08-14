package ociobjectstore

import (
	"io"
	"time"
)

// DownloadPhase identifies a bounded step in the object download pipeline.
// Values are suitable for use as metric labels; object and part identifiers
// belong in DownloadObservation and must remain structured-log fields.
type DownloadPhase string

const (
	PhaseQueueWait          DownloadPhase = "worker_queue_wait"
	PhaseStatusUpdating     DownloadPhase = "status_updating"
	PhasePrefixList         DownloadPhase = "prefix_list_objects"
	PhaseBulkDownload       DownloadPhase = "bulk_download"
	PhaseFinalVerification  DownloadPhase = "final_verification"
	PhaseModelConfigUpdate  DownloadPhase = "model_config_update"
	PhaseStatusReady        DownloadPhase = "status_ready"
	PhaseExistingCopyCheck  DownloadPhase = "existing_copy_check"
	PhaseStrategyList       DownloadPhase = "strategy_list_objects"
	PhaseMultipartList      DownloadPhase = "multipart_list_objects"
	PhaseMultipartTotal     DownloadPhase = "multipart_total"
	PhaseGetObjectRequest   DownloadPhase = "get_object_request"
	PhaseGetObjectFirstRead DownloadPhase = "get_object_first_read"
	PhaseGetObjectBodyRead  DownloadPhase = "get_object_body_read"
	PhaseObjectToPartCopy   DownloadPhase = "object_to_part_file_copy"
	PhasePartFileSync       DownloadPhase = "part_file_sync"
	PhasePartChannelWait    DownloadPhase = "part_channel_wait"
	PhasePartToModelCopy    DownloadPhase = "part_to_model_file_copy"
	PhaseModelFileSync      DownloadPhase = "model_file_sync"
	PhaseModelFileClose     DownloadPhase = "model_file_close"
	PhaseModelFileRename    DownloadPhase = "model_file_rename"
	PhaseStandardFileCopy   DownloadPhase = "standard_file_copy"
	PhaseHeadObject         DownloadPhase = "head_object"
	PhaseMD5Read            DownloadPhase = "md5_read"
	PhaseLocalValidation    DownloadPhase = "local_validation"
)

// DownloadOutcome is deliberately bounded for use as a metric label.
type DownloadOutcome string

const (
	DownloadOutcomeSuccess  DownloadOutcome = "success"
	DownloadOutcomeError    DownloadOutcome = "error"
	DownloadOutcomeSkipped  DownloadOutcome = "skipped"
	DownloadOutcomeMismatch DownloadOutcome = "mismatch"
)

// DownloadObservation describes one completed phase. High-cardinality fields
// such as ObjectName and PartNumber are intended for structured logs only.
type DownloadObservation struct {
	Phase      DownloadPhase
	Duration   time.Duration
	Bytes      int64
	Outcome    DownloadOutcome
	ObjectName string
	PartNumber int
	HasPart    bool
	Attempt    int
	ChunkSize  int64
	Err        error
}

// DownloadObserver receives timing observations without coupling this package
// to a specific metrics or logging implementation.
type DownloadObserver interface {
	ObserveDownloadPhase(DownloadObservation)
}

// SetDownloadObserver configures an optional observer for this data-store
// instance. A data-store created by model-agent is scoped to one model task.
func (cds *OCIOSDataStore) SetDownloadObserver(observer DownloadObserver) {
	cds.observer = observer
}

func (cds *OCIOSDataStore) observeDownloadPhase(observation DownloadObservation) {
	if cds.observer != nil {
		cds.observer.ObserveDownloadPhase(observation)
	}
}

// timedReader accumulates time spent inside body Read calls without logging
// every buffer. The surrounding copy is timed separately because wrapping the
// file writer would disable io.Copy fast paths and perturb the measurement.
type timedReader struct {
	reader            io.Reader
	duration          time.Duration
	bytes             int64
	copyStartedAt     time.Time
	firstReadDuration time.Duration
	observedFirstRead bool
}

func newTimedReader(reader io.Reader, copyStartedAt time.Time) *timedReader {
	return &timedReader{reader: reader, copyStartedAt: copyStartedAt}
}

func (r *timedReader) Read(buffer []byte) (int, error) {
	startedAt := time.Now()
	n, err := r.reader.Read(buffer)
	r.duration += time.Since(startedAt)
	r.bytes += int64(n)
	if n > 0 && !r.observedFirstRead {
		r.firstReadDuration = time.Since(r.copyStartedAt)
		r.observedFirstRead = true
	}
	return n, err
}
