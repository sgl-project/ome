package storage

import (
	"io"
	"sync/atomic"
	"time"
)

// ProgressTracker tracks progress of operations
type ProgressTracker struct {
	totalBytes     int64
	processedBytes int64
	totalFiles     int
	processedFiles int
	currentFile    string
	startTime      time.Time
	lastUpdate     time.Time
	lastBytes      int64
	callback       ProgressCallback
	updateInterval time.Duration
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalBytes int64, totalFiles int, callback ProgressCallback) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		totalBytes:     totalBytes,
		totalFiles:     totalFiles,
		startTime:      now,
		lastUpdate:     now,
		callback:       callback,
		updateInterval: 100 * time.Millisecond, // Update every 100ms
	}
}

// SetCurrentFile sets the current file being processed
func (pt *ProgressTracker) SetCurrentFile(filename string) {
	pt.currentFile = filename
	pt.reportProgress()
}

// AddBytes adds to the processed bytes count
func (pt *ProgressTracker) AddBytes(bytes int64) {
	atomic.AddInt64(&pt.processedBytes, bytes)

	// Check if we should report progress
	now := time.Now()
	if now.Sub(pt.lastUpdate) >= pt.updateInterval {
		pt.reportProgress()
		pt.lastUpdate = now
	}
}

// CompleteFile marks a file as completed
func (pt *ProgressTracker) CompleteFile() {
	pt.processedFiles++
	pt.reportProgress()
}

// SetError sets an error
func (pt *ProgressTracker) SetError(err error) {
	progress := pt.calculateProgress()
	progress.Error = err
	if pt.callback != nil {
		pt.callback(progress)
	}
}

// Complete marks the operation as complete
func (pt *ProgressTracker) Complete() {
	pt.processedBytes = pt.totalBytes
	pt.processedFiles = pt.totalFiles
	pt.reportProgress()
}

// reportProgress calculates and reports current progress
func (pt *ProgressTracker) reportProgress() {
	if pt.callback != nil {
		pt.callback(pt.calculateProgress())
	}
}

// calculateProgress calculates the current progress
func (pt *ProgressTracker) calculateProgress() Progress {
	now := time.Now()
	elapsedTime := now.Sub(pt.startTime)

	// Calculate speeds
	currentSpeed := float64(0)
	if timeDiff := now.Sub(pt.lastUpdate).Seconds(); timeDiff > 0 {
		bytesDiff := atomic.LoadInt64(&pt.processedBytes) - pt.lastBytes
		currentSpeed = float64(bytesDiff) / timeDiff
	}

	averageSpeed := float64(0)
	if elapsedSeconds := elapsedTime.Seconds(); elapsedSeconds > 0 {
		averageSpeed = float64(atomic.LoadInt64(&pt.processedBytes)) / elapsedSeconds
	}

	// Estimate remaining time
	estimatedTime := time.Duration(0)
	if averageSpeed > 0 && pt.totalBytes > 0 {
		remainingBytes := pt.totalBytes - atomic.LoadInt64(&pt.processedBytes)
		estimatedSeconds := float64(remainingBytes) / averageSpeed
		estimatedTime = time.Duration(estimatedSeconds * float64(time.Second))
	}

	pt.lastBytes = atomic.LoadInt64(&pt.processedBytes)

	return Progress{
		TotalBytes:     pt.totalBytes,
		ProcessedBytes: atomic.LoadInt64(&pt.processedBytes),
		TotalFiles:     pt.totalFiles,
		ProcessedFiles: pt.processedFiles,
		CurrentFile:    pt.currentFile,
		StartTime:      pt.startTime,
		CurrentSpeed:   currentSpeed,
		AverageSpeed:   averageSpeed,
		EstimatedTime:  estimatedTime,
		ElapsedTime:    elapsedTime,
	}
}

// ProgressReader wraps a reader to track progress
type ProgressReader struct {
	reader   io.Reader
	tracker  *ProgressTracker
	readSize int64
}

// NewProgressReader creates a new progress reader
func NewProgressReader(reader io.Reader, tracker *ProgressTracker) *ProgressReader {
	return &ProgressReader{
		reader:  reader,
		tracker: tracker,
	}
}

// Read implements io.Reader
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		pr.tracker.AddBytes(int64(n))
		pr.readSize += int64(n)
	}
	return n, err
}

// Close implements io.Closer if the underlying reader supports it
func (pr *ProgressReader) Close() error {
	if closer, ok := pr.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// ProgressWriter wraps a writer to track progress
type ProgressWriter struct {
	writer    io.Writer
	tracker   *ProgressTracker
	writeSize int64
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(writer io.Writer, tracker *ProgressTracker) *ProgressWriter {
	return &ProgressWriter{
		writer:  writer,
		tracker: tracker,
	}
}

// Write implements io.Writer
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if n > 0 {
		pw.tracker.AddBytes(int64(n))
		pw.writeSize += int64(n)
	}
	return n, err
}

// MultiProgressTracker tracks progress of multiple operations
type MultiProgressTracker struct {
	trackers map[string]*ProgressTracker
	callback ProgressCallback
}

// NewMultiProgressTracker creates a new multi-progress tracker
func NewMultiProgressTracker(callback ProgressCallback) *MultiProgressTracker {
	return &MultiProgressTracker{
		trackers: make(map[string]*ProgressTracker),
		callback: callback,
	}
}

// AddTracker adds a progress tracker for a specific operation
func (mpt *MultiProgressTracker) AddTracker(id string, totalBytes int64, totalFiles int) *ProgressTracker {
	tracker := NewProgressTracker(totalBytes, totalFiles, func(progress Progress) {
		// Aggregate progress from all trackers
		aggregated := mpt.aggregateProgress()
		if mpt.callback != nil {
			mpt.callback(aggregated)
		}
	})
	mpt.trackers[id] = tracker
	return tracker
}

// RemoveTracker removes a tracker
func (mpt *MultiProgressTracker) RemoveTracker(id string) {
	delete(mpt.trackers, id)
}

// aggregateProgress aggregates progress from all trackers
func (mpt *MultiProgressTracker) aggregateProgress() Progress {
	var totalBytes, processedBytes int64
	var totalFiles, processedFiles int
	var totalSpeed, avgSpeed float64
	var earliestStart time.Time

	for _, tracker := range mpt.trackers {
		progress := tracker.calculateProgress()
		totalBytes += progress.TotalBytes
		processedBytes += progress.ProcessedBytes
		totalFiles += progress.TotalFiles
		processedFiles += progress.ProcessedFiles
		totalSpeed += progress.CurrentSpeed
		avgSpeed += progress.AverageSpeed

		if earliestStart.IsZero() || progress.StartTime.Before(earliestStart) {
			earliestStart = progress.StartTime
		}
	}

	// Calculate aggregate values
	elapsedTime := time.Since(earliestStart)
	estimatedTime := time.Duration(0)
	if avgSpeed > 0 && totalBytes > 0 {
		remainingBytes := totalBytes - processedBytes
		estimatedSeconds := float64(remainingBytes) / avgSpeed
		estimatedTime = time.Duration(estimatedSeconds * float64(time.Second))
	}

	return Progress{
		TotalBytes:     totalBytes,
		ProcessedBytes: processedBytes,
		TotalFiles:     totalFiles,
		ProcessedFiles: processedFiles,
		StartTime:      earliestStart,
		CurrentSpeed:   totalSpeed,
		AverageSpeed:   avgSpeed,
		EstimatedTime:  estimatedTime,
		ElapsedTime:    elapsedTime,
	}
}
