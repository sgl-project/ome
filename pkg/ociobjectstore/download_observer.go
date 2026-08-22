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
	PhaseVerificationWait   DownloadPhase = "model_verification_limiter_wait"
	PhaseModelConfigUpdate  DownloadPhase = "model_config_update"
	PhaseStatusReady        DownloadPhase = "status_ready"
	PhaseExistingCopyCheck  DownloadPhase = "existing_copy_check"
	PhaseStrategyList       DownloadPhase = "strategy_list_objects"
	PhaseMultipartList      DownloadPhase = "multipart_list_objects"
	PhaseMultipartTotal     DownloadPhase = "multipart_total"
	PhaseGetObjectRequest   DownloadPhase = "get_object_request"
	PhaseGetObjectFirstRead DownloadPhase = "get_object_first_read"
	PhaseGetObjectBodyRead  DownloadPhase = "get_object_body_read"
	PhaseModelFileWrite     DownloadPhase = "model_file_write"
	PhaseModelFileWriteWait DownloadPhase = "model_file_write_limiter_wait"
	PhaseObjectToPartCopy   DownloadPhase = "object_to_part_file_copy"
	PhasePartFileSync       DownloadPhase = "part_file_sync"
	PhasePartChannelWait    DownloadPhase = "part_channel_wait"
	PhasePartToModelCopy    DownloadPhase = "part_to_model_file_copy"
	PhaseObjectToModelWrite DownloadPhase = "object_to_model_file_write"
	PhaseModelFileAllocate  DownloadPhase = "model_file_preallocate"
	PhaseModelFileSync      DownloadPhase = "model_file_sync"
	PhaseModelFileClose     DownloadPhase = "model_file_close"
	PhaseModelFileRename    DownloadPhase = "model_file_rename"
	PhaseStandardFileCopy   DownloadPhase = "standard_file_copy"
	PhaseHeadObject         DownloadPhase = "head_object"
	PhaseMD5Read            DownloadPhase = "md5_read"
	PhaseLocalValidation    DownloadPhase = "local_validation"
)

// WriteStats summarizes the write calls made while streaming one object range.
// Size bucket counts are mutually exclusive and deliberately bounded so they
// can be exported as Prometheus labels without recording one metric per write.
type WriteStats struct {
	Calls                      int64
	Bytes                      int64
	Duration                   time.Duration
	LimiterWaitCalls           int64
	LimiterWaitDuration        time.Duration
	MaxDuration                time.Duration
	MaxLimiterWaitDuration     time.Duration
	MaxInflightWrites          int64
	MaxWaitingWriters          int64
	BufferSizeBytes            int64
	MinRequestBytes            int64
	MaxRequestBytes            int64
	CallsUpTo16KiB             int64
	Calls16KiBTo64KiB          int64
	Calls64KiBTo256KiB         int64
	Calls256KiBTo1MiB          int64
	CallsOver1MiB              int64
	WriteDurationBuckets       DurationBucketCounts
	LimiterWaitDurationBuckets DurationBucketCounts
}

// ReadStats summarizes the underlying HTTP body Read calls made while
// streaming one object range. A short read returned some data but less than
// the supplied buffer; zero-byte reads are tracked separately so normal EOF
// calls do not inflate the short-read count.
type ReadStats struct {
	Calls               int64
	Bytes               int64
	ShortReadCalls      int64
	ZeroByteReadCalls   int64
	Duration            time.Duration
	MaxDuration         time.Duration
	MinRequestBytes     int64
	MaxRequestBytes     int64
	MinReturnedBytes    int64
	MaxReturnedBytes    int64
	CallsUpTo16KiB      int64
	Calls16KiBTo64KiB   int64
	Calls64KiBTo256KiB  int64
	Calls256KiBTo1MiB   int64
	CallsOver1MiB       int64
	ReadDurationBuckets DurationBucketCounts
}

// DurationBucketCounts is a bounded per-call distribution accumulated in the
// download goroutine. It avoids one Prometheus operation or log entry per
// WriteAt call.
type DurationBucketCounts struct {
	UpTo1ms       int64
	From1To5ms    int64
	From5To20ms   int64
	From20To100ms int64
	Over100ms     int64
}

func (counts *DurationBucketCounts) observe(duration time.Duration) {
	switch {
	case duration <= time.Millisecond:
		counts.UpTo1ms++
	case duration <= 5*time.Millisecond:
		counts.From1To5ms++
	case duration <= 20*time.Millisecond:
		counts.From5To20ms++
	case duration <= 100*time.Millisecond:
		counts.From20To100ms++
	default:
		counts.Over100ms++
	}
}

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
	WriteStats *WriteStats
	ReadStats  *ReadStats
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
// every buffer. The surrounding copy is timed separately. The direct-write
// path also times its OffsetWriter; neither side implements an io.Copy fast
// path, so that wrapper does not change the selected copy implementation.
type timedReader struct {
	reader            io.Reader
	duration          time.Duration
	bytes             int64
	stats             ReadStats
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
	duration := time.Since(startedAt)
	r.duration += duration
	r.bytes += int64(n)
	r.stats.Calls++
	r.stats.Bytes += int64(n)
	r.stats.Duration += duration
	r.stats.ReadDurationBuckets.observe(duration)
	if duration > r.stats.MaxDuration {
		r.stats.MaxDuration = duration
	}
	requested := int64(len(buffer))
	if requested > 0 {
		if r.stats.MinRequestBytes == 0 || requested < r.stats.MinRequestBytes {
			r.stats.MinRequestBytes = requested
		}
		if requested > r.stats.MaxRequestBytes {
			r.stats.MaxRequestBytes = requested
		}
	}
	if n == 0 {
		r.stats.ZeroByteReadCalls++
	} else {
		returned := int64(n)
		if n < len(buffer) {
			r.stats.ShortReadCalls++
		}
		if r.stats.MinReturnedBytes == 0 || returned < r.stats.MinReturnedBytes {
			r.stats.MinReturnedBytes = returned
		}
		if returned > r.stats.MaxReturnedBytes {
			r.stats.MaxReturnedBytes = returned
		}
		switch {
		case returned <= 16*1024:
			r.stats.CallsUpTo16KiB++
		case returned <= 64*1024:
			r.stats.Calls16KiBTo64KiB++
		case returned <= 256*1024:
			r.stats.Calls64KiBTo256KiB++
		case returned <= 1024*1024:
			r.stats.Calls256KiBTo1MiB++
		default:
			r.stats.CallsOver1MiB++
		}
	}
	if n > 0 && !r.observedFirstRead {
		r.firstReadDuration = time.Since(r.copyStartedAt)
		r.observedFirstRead = true
	}
	return n, err
}
