package hub

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	fortiopb "fortio.org/progressbar"
	schollzpb "github.com/schollz/progressbar/v3"
	"github.com/sgl-project/ome/pkg/logging"
)

// ProgressManager manages progress reporting for downloads
type ProgressManager struct {
	logger             logging.Interface
	enableProgressBars bool
	enableDetailedLogs bool
	displayMode        ProgressDisplayMode
	multiProgress      *MultiProgressManager
	// Fields for log-only mode progress tracking
	fileProgress map[string]*FileProgress
	mu           sync.Mutex
}

// FileProgress tracks download progress for log-only mode
type FileProgress struct {
	filename    string
	totalSize   int64
	downloaded  int64
	startTime   time.Time
	lastLogTime time.Time
}

// MultiProgressManager coordinates multiple concurrent progress bars
type MultiProgressManager struct {
	multiBar    *fortiopb.MultiBar
	maxWorkers  int
	enabled     bool
	mutex       sync.RWMutex
	workerFiles map[int]string // Track which file each worker is downloading
}

// NewMultiProgressManager creates a new multi-progress manager
func NewMultiProgressManager(maxWorkers int, enabled bool) *MultiProgressManager {
	if !enabled {
		return &MultiProgressManager{
			maxWorkers: maxWorkers,
			enabled:    false,
		}
	}

	// Create fortio progressbar configuration
	cfg := fortiopb.DefaultConfig()
	cfg.ExtraLines = 1
	cfg.ScreenWriter = os.Stdout

	// Create prefixes: overall + workers
	prefixes := make([]string, maxWorkers+1)
	prefixes[0] = "📦 Overall     "
	for i := 1; i <= maxWorkers; i++ {
		prefixes[i] = fmt.Sprintf("Worker %d      ", i-1)
	}

	multiBar := cfg.NewMultiBarPrefixes(prefixes...)

	return &MultiProgressManager{
		multiBar:    multiBar,
		maxWorkers:  maxWorkers,
		enabled:     true,
		workerFiles: make(map[int]string),
	}
}

// CreateSnapshotProgressBar initializes the overall progress bar
func (mpm *MultiProgressManager) CreateSnapshotProgressBar(totalFiles int, totalSize int64) ProgressBar {
	if !mpm.enabled || mpm.multiBar == nil || len(mpm.multiBar.Bars) == 0 {
		return nil
	}

	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()

	overallBar := mpm.multiBar.Bars[0]
	overallBar.WriteAbove(fmt.Sprintf("📦 Starting download: %d files (%s) with %d workers",
		totalFiles, formatSize(totalSize), mpm.maxWorkers))

	return &ConcurrentProgressBar{
		bar:         overallBar,
		total:       int64(totalFiles),
		manager:     mpm,
		workerID:    -1, // -1 indicates overall bar
		description: "Overall Progress",
		startTime:   time.Now(),
	}
}

// CreateWorkerProgressBar creates a progress bar for a specific worker
func (mpm *MultiProgressManager) CreateWorkerProgressBar(workerID int, filename string, size int64) ProgressBar {
	if !mpm.enabled || mpm.multiBar == nil || workerID >= mpm.maxWorkers {
		return nil
	}

	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()

	if len(mpm.multiBar.Bars) <= workerID+1 {
		return nil
	}

	// Store which file this worker is downloading
	mpm.workerFiles[workerID] = filename

	workerBar := mpm.multiBar.Bars[workerID+1]

	// Update the worker's display name to show current file
	shortFilename := filename
	if len(shortFilename) > 25 {
		shortFilename = shortFilename[:22] + "..."
	}

	return &ConcurrentProgressBar{
		bar:         workerBar,
		total:       size,
		manager:     mpm,
		workerID:    workerID,
		filename:    filename,
		description: fmt.Sprintf("Worker %d: %s", workerID, shortFilename),
		startTime:   time.Now(),
	}
}

// UpdateWorkerFile updates which file a worker is currently downloading
func (mpm *MultiProgressManager) UpdateWorkerFile(workerID int, filename string) {
	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()
	mpm.workerFiles[workerID] = filename
}

// RemoveWorkerProgressBar completes a worker's progress bar
func (mpm *MultiProgressManager) RemoveWorkerProgressBar(workerID int) {
	if !mpm.enabled || workerID >= mpm.maxWorkers {
		return
	}

	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()

	if len(mpm.multiBar.Bars) > workerID+1 {
		workerBar := mpm.multiBar.Bars[workerID+1]
		workerBar.Progress(100.0)
	}

	delete(mpm.workerFiles, workerID)
}

// GetActiveWorkerCount returns the number of active workers
func (mpm *MultiProgressManager) GetActiveWorkerCount() int {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()
	return len(mpm.workerFiles)
}

// Shutdown gracefully shuts down the multi-progress manager
func (mpm *MultiProgressManager) Shutdown() {
	if mpm.enabled && mpm.multiBar != nil {
		mpm.multiBar.End()
		fmt.Println("\n🎉 All downloads completed!")
	}
}

// ConcurrentProgressBar wraps fortio/progressbar with download speed tracking
type ConcurrentProgressBar struct {
	bar         *fortiopb.Bar
	total       int64
	current     int64
	manager     *MultiProgressManager
	workerID    int // -1 for overall bar
	filename    string
	description string
	startTime   time.Time
	lastUpdate  time.Time
	mutex       sync.Mutex
}

func (cpb *ConcurrentProgressBar) Add(n int) error {
	cpb.mutex.Lock()
	defer cpb.mutex.Unlock()

	cpb.current += int64(n)

	if cpb.total > 0 {
		percentage := float64(cpb.current) / float64(cpb.total) * 100

		// Update progress with speed info for worker bars
		if cpb.workerID >= 0 && cpb.current > 0 {
			elapsed := time.Since(cpb.startTime)
			now := time.Now()

			// Only update display every 500ms to avoid flickering
			if now.Sub(cpb.lastUpdate) > 500*time.Millisecond || cpb.current == cpb.total {
				if elapsed > 0 {
					speed := float64(cpb.current) / elapsed.Seconds()
					shortFilename := truncateFilename(cpb.filename, 15)

					// Use WriteAbove to display current status without interfering with progress bar
					cpb.bar.WriteAbove(fmt.Sprintf("Worker %d: %s - %s/s (%.1f%%)",
						cpb.workerID,
						shortFilename,
						formatSize(int64(speed)),
						percentage))
				}
				cpb.lastUpdate = now
			}
		}

		cpb.bar.Progress(percentage)
	}

	return nil
}

func (cpb *ConcurrentProgressBar) Finish() error {
	cpb.mutex.Lock()
	defer cpb.mutex.Unlock()

	if cpb.workerID >= 0 {
		// Show final completion status
		elapsed := time.Since(cpb.startTime)
		if elapsed > 0 && cpb.total > 0 {
			avgSpeed := float64(cpb.total) / elapsed.Seconds()
			shortFilename := truncateFilename(cpb.filename, 15)
			cpb.bar.WriteAbove(fmt.Sprintf("Worker %d: %s ✅ DONE (%s/s)",
				cpb.workerID,
				shortFilename,
				formatSize(int64(avgSpeed))))
		}
	}

	cpb.bar.Progress(100.0)
	return nil
}

func (cpb *ConcurrentProgressBar) Current() int64 {
	cpb.mutex.Lock()
	defer cpb.mutex.Unlock()
	return cpb.current
}

func (cpb *ConcurrentProgressBar) SetTotal(total int64, wontAdd ...bool) error {
	cpb.mutex.Lock()
	defer cpb.mutex.Unlock()

	cpb.total = total
	if len(wontAdd) > 0 && wontAdd[0] {
		cpb.bar.Progress(100.0)
	}
	return nil
}

// NewProgressManager creates a new progress manager
func NewProgressManager(logger logging.Interface, enableBars, enableLogs bool) *ProgressManager {
	// For backward compatibility, determine display mode from enableBars
	displayMode := ProgressModeLog
	if enableBars {
		displayMode = ProgressModeBars
	}

	return NewProgressManagerWithMode(logger, displayMode, enableLogs)
}

// NewProgressManagerWithMode creates a new progress manager with explicit display mode
func NewProgressManagerWithMode(logger logging.Interface, displayMode ProgressDisplayMode, enableLogs bool) *ProgressManager {
	pm := &ProgressManager{
		logger:             logger,
		enableProgressBars: displayMode == ProgressModeBars,
		enableDetailedLogs: enableLogs,
		displayMode:        displayMode,
		fileProgress:       make(map[string]*FileProgress),
	}

	return pm
}

// InitializeMultiProgress initializes multi-progress support for concurrent downloads
func (pm *ProgressManager) InitializeMultiProgress(maxWorkers int) {
	if pm.enableProgressBars && maxWorkers > 1 {
		pm.multiProgress = NewMultiProgressManager(maxWorkers, true)
	}
}

// ProgressBar interface for different progress bar implementations
type ProgressBar interface {
	Add(int) error
	Finish() error
	Current() int64
	SetTotal(total int64, wontAdd ...bool) error
}

// CreateWorkerFileProgressBar creates a progress bar for a worker-specific file download
func (pm *ProgressManager) CreateWorkerFileProgressBar(workerID int, filename string, size int64) ProgressBar {
	if pm.multiProgress != nil {
		return pm.multiProgress.CreateWorkerProgressBar(workerID, filename, size)
	}
	return pm.CreateFileProgressBar(filename, size)
}

// CompleteWorkerProgress handles completion of worker progress
func (pm *ProgressManager) CompleteWorkerProgress(workerID int) {
	if pm.multiProgress != nil {
		pm.multiProgress.RemoveWorkerProgressBar(workerID)
	}
}

// CreateSnapshotProgressBar creates a progress bar for snapshot downloads
func (pm *ProgressManager) CreateSnapshotProgressBar(totalFiles int, totalSize int64) ProgressBar {
	switch pm.displayMode {
	case ProgressModeBars:
		if pm.multiProgress != nil {
			return pm.multiProgress.CreateSnapshotProgressBar(totalFiles, totalSize)
		}

		// Fallback to single progress bar
		description := fmt.Sprintf("📦 Downloading %d files (%s)", totalFiles, formatSize(totalSize))
		bar := schollzpb.NewOptions(totalFiles,
			schollzpb.OptionSetDescription(description),
			schollzpb.OptionSetWidth(40),
			schollzpb.OptionShowCount(),
			schollzpb.OptionEnableColorCodes(true),
		)
		return &SingleProgressBar{bar: bar}

	case ProgressModeLog:
		// For snapshot downloads in log mode, we'll track overall progress
		description := fmt.Sprintf("snapshot_%d_files", totalFiles)
		return &LogProgressBar{
			manager:  pm,
			filename: description,
			total:    int64(totalFiles),
		}

	default:
		return nil
	}
}

// CreateFileProgressBar creates a progress bar for a single file download
func (pm *ProgressManager) CreateFileProgressBar(filename string, size int64) ProgressBar {
	switch pm.displayMode {
	case ProgressModeBars:
		description := fmt.Sprintf("📄 %s", truncateFilename(filename, 40))
		bar := schollzpb.NewOptions64(size,
			schollzpb.OptionSetDescription(description),
			schollzpb.OptionSetWidth(30),
			schollzpb.OptionShowBytes(true),
			schollzpb.OptionShowCount(),
			schollzpb.OptionEnableColorCodes(true),
		)
		return &SingleProgressBar{bar: bar}

	case ProgressModeLog:
		return &LogProgressBar{
			manager:  pm,
			filename: filename,
			total:    size,
		}

	default:
		return nil
	}
}

// CreateSpinner creates a spinner for unknown progress operations
func (pm *ProgressManager) CreateSpinner(description string) *schollzpb.ProgressBar {
	if !pm.enableProgressBars {
		return nil
	}

	return schollzpb.NewOptions(-1,
		schollzpb.OptionSetDescription(description),
		schollzpb.OptionSetWidth(10),
		schollzpb.OptionSpinnerType(14),
		schollzpb.OptionEnableColorCodes(true),
	)
}

// SingleProgressBar wraps schollz/progressbar for single downloads
type SingleProgressBar struct {
	bar *schollzpb.ProgressBar
}

func (spb *SingleProgressBar) Add(n int) error                             { return spb.bar.Add(n) }
func (spb *SingleProgressBar) Finish() error                               { return spb.bar.Finish() }
func (spb *SingleProgressBar) Current() int64                              { return spb.bar.State().CurrentNum }
func (spb *SingleProgressBar) SetTotal(total int64, wontAdd ...bool) error { return spb.bar.Finish() }

// Shutdown gracefully shuts down the progress manager
func (pm *ProgressManager) Shutdown() {
	if pm.multiProgress != nil {
		pm.multiProgress.Shutdown()
	}
}

// Helper functions
func truncateFilename(filename string, maxLen int) string {
	if len(filename) <= maxLen {
		return filename
	}
	return filename[:maxLen-3] + "..."
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ProgressWriter wraps a progress bar as an io.Writer
type ProgressWriter struct {
	bar    ProgressBar
	writer io.Writer
}

func NewProgressWriter(bar ProgressBar, writer io.Writer) *ProgressWriter {
	return &ProgressWriter{bar: bar, writer: writer}
}

func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if err == nil && pw.bar != nil {
		_ = pw.bar.Add(n)
	}
	return n, err
}

// Logging methods (simplified)
func (pm *ProgressManager) LogDownloadStart(repoID, filename string, size int64) {
	if pm.logger != nil {
		pm.logger.WithField("repo_id", repoID).WithField("filename", filename).Info("Starting download")
	}
}

func (pm *ProgressManager) LogDownloadComplete(repoID, filename string, duration time.Duration, size int64) {
	if pm.logger != nil {
		speed := float64(size) / duration.Seconds()
		pm.logger.WithField("repo_id", repoID).WithField("filename", filename).
			WithField("speed_bps", speed).Info("Download completed")
	}
}

func (pm *ProgressManager) LogSnapshotStart(repoID string, fileCount int, totalSize int64) {
	if pm.logger != nil {
		pm.logger.WithField("repo_id", repoID).WithField("file_count", fileCount).
			WithField("total_size", totalSize).Info("Starting snapshot download")
	}
}

func (pm *ProgressManager) LogSnapshotComplete(repoID string, fileCount int, duration time.Duration, totalSize int64) {
	if pm.logger != nil {
		avgSpeed := float64(totalSize) / duration.Seconds()
		pm.logger.WithField("repo_id", repoID).WithField("file_count", fileCount).
			WithField("avg_speed", avgSpeed).Info("Snapshot download completed")
	}
}

func (pm *ProgressManager) LogError(operation, repoID string, err error) {
	if pm.logger != nil {
		pm.logger.WithField("operation", operation).WithField("repo_id", repoID).
			WithError(err).Error("Operation failed")
	}
}

func (pm *ProgressManager) LogRepoListing(repoID string, fileCount int) {
	if pm.logger != nil {
		pm.logger.WithField("repo_id", repoID).WithField("file_count", fileCount).
			Info("Repository files listed")
	}
}

// LogProgressBar implements ProgressBar interface for log-only mode
type LogProgressBar struct {
	manager  *ProgressManager
	filename string
	total    int64
	current  int64
	mu       sync.Mutex
}

func (lpb *LogProgressBar) Add(n int) error {
	lpb.mu.Lock()
	lpb.current += int64(n)
	current := lpb.current
	lpb.mu.Unlock()

	lpb.manager.logProgress(lpb.filename, current, lpb.total)
	return nil
}

func (lpb *LogProgressBar) Finish() error {
	lpb.manager.logProgress(lpb.filename, lpb.total, lpb.total)
	if lpb.manager.logger != nil {
		lpb.manager.logger.WithField("filename", lpb.filename).Info("Download completed")
	}

	// Clean up tracking
	lpb.manager.mu.Lock()
	delete(lpb.manager.fileProgress, lpb.filename)
	lpb.manager.mu.Unlock()

	return nil
}

func (lpb *LogProgressBar) Current() int64 {
	lpb.mu.Lock()
	defer lpb.mu.Unlock()
	return lpb.current
}

func (lpb *LogProgressBar) SetTotal(total int64, wontAdd ...bool) error {
	lpb.mu.Lock()
	lpb.total = total
	lpb.mu.Unlock()
	return nil
}

// logProgress logs download progress at appropriate intervals
func (pm *ProgressManager) logProgress(filename string, current, total int64) {
	if pm.displayMode != ProgressModeLog || pm.logger == nil {
		return
	}

	pm.mu.Lock()
	progress, exists := pm.fileProgress[filename]
	if !exists {
		progress = &FileProgress{
			filename:  filename,
			totalSize: total,
			startTime: time.Now(),
		}
		pm.fileProgress[filename] = progress
	}

	// Calculate progress percentages BEFORE updating downloaded
	percentComplete := float64(current) / float64(total) * 100
	lastPercent := float64(progress.downloaded) / float64(total) * 100

	// Log progress at intervals (every 5 seconds or 10% progress)
	now := time.Now()
	shouldLog := now.Sub(progress.lastLogTime) > 5*time.Second ||
		int(percentComplete/10) > int(lastPercent/10)

	if shouldLog && current < total {
		elapsed := now.Sub(progress.startTime)
		speed := float64(current) / elapsed.Seconds()

		logger := pm.logger.
			WithField("filename", truncateFilename(filename, 40)).
			WithField("progress", fmt.Sprintf("%.1f%%", percentComplete)).
			WithField("speed", formatSize(int64(speed))+"/s").
			WithField("downloaded", formatSize(current)).
			WithField("total", formatSize(total))

		// Estimate remaining time
		if speed > 0 {
			remaining := time.Duration(float64(total-current)/speed) * time.Second
			logger = logger.WithField("eta", remaining.Round(time.Second).String())
		}

		logger.Info("Download progress")
		// Only update downloaded amount after logging
		progress.downloaded = current
		progress.lastLogTime = now
	}
	pm.mu.Unlock()
}
