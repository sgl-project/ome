package hub

import (
	"fmt"
	"io"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/sgl-project/sgl-ome/pkg/logging"
)

// ProgressManager manages progress reporting for downloads
type ProgressManager struct {
	logger             logging.Interface
	enableProgressBars bool
	enableDetailedLogs bool
}

// NewProgressManager creates a new progress manager
func NewProgressManager(logger logging.Interface, enableBars, enableLogs bool) *ProgressManager {
	return &ProgressManager{
		logger:             logger,
		enableProgressBars: enableBars,
		enableDetailedLogs: enableLogs,
	}
}

// CreateFileProgressBar creates a progress bar for a single file download
func (pm *ProgressManager) CreateFileProgressBar(filename string, size int64) *progressbar.ProgressBar {
	if !pm.enableProgressBars {
		return nil
	}

	description := fmt.Sprintf("📄 %s", filename)
	if len(description) > 50 {
		description = description[:47] + "..."
	}

	bar := progressbar.NewOptions64(size,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "▐",
			BarEnd:        "▌",
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Printf("\n✅ %s completed\n", filename)
		}),
		progressbar.OptionThrottle(100*time.Millisecond),
	)

	return bar
}

// CreateSnapshotProgressBar creates a progress bar for snapshot downloads
func (pm *ProgressManager) CreateSnapshotProgressBar(totalFiles int, totalSize int64) *progressbar.ProgressBar {
	if !pm.enableProgressBars {
		return nil
	}

	description := fmt.Sprintf("📦 Downloading %d files (%s)", totalFiles, formatSize(totalSize))

	bar := progressbar.NewOptions(totalFiles,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "▓",
			SaucerHead:    "▓",
			SaucerPadding: "░",
			BarStart:      "▐",
			BarEnd:        "▌",
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Print("\n🎉 Snapshot download completed!\n")
		}),
		progressbar.OptionThrottle(500*time.Millisecond),
	)

	return bar
}

// CreateSpinner creates a spinner for unknown progress operations
func (pm *ProgressManager) CreateSpinner(description string) *progressbar.ProgressBar {
	if !pm.enableProgressBars {
		return nil
	}

	return progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(10),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionThrottle(100*time.Millisecond),
	)
}

// LogDownloadStart logs the start of a download operation
func (pm *ProgressManager) LogDownloadStart(repoID, filename string, size int64) {
	if pm.logger == nil {
		return
	}

	fields := map[string]interface{}{
		"repo_id":  repoID,
		"filename": filename,
		"size":     size,
	}

	if pm.enableDetailedLogs {
		logger := pm.logger
		for k, v := range fields {
			logger = logger.WithField(k, v)
		}
		logger.Info("Starting file download")
	} else {
		pm.logger.WithField("repo_id", repoID).Info("Starting download")
	}
}

// LogDownloadComplete logs the completion of a download
func (pm *ProgressManager) LogDownloadComplete(repoID, filename string, duration time.Duration, size int64) {
	if pm.logger == nil {
		return
	}

	speed := float64(size) / duration.Seconds()
	fields := map[string]interface{}{
		"repo_id":     repoID,
		"filename":    filename,
		"duration_ms": duration.Milliseconds(),
		"size":        size,
		"speed_bps":   speed,
	}

	if pm.enableDetailedLogs {
		logger := pm.logger
		for k, v := range fields {
			logger = logger.WithField(k, v)
		}
		logger.Info("Download completed successfully")
	} else {
		pm.logger.WithField("repo_id", repoID).Info("Download completed")
	}
}

// LogSnapshotStart logs the start of a snapshot download
func (pm *ProgressManager) LogSnapshotStart(repoID string, fileCount int, totalSize int64) {
	if pm.logger == nil {
		return
	}

	logger := pm.logger.
		WithField("repo_id", repoID).
		WithField("file_count", fileCount).
		WithField("total_size", totalSize).
		WithField("operation", "snapshot_download")
	logger.Info("Starting snapshot download")
}

// LogSnapshotComplete logs the completion of a snapshot download
func (pm *ProgressManager) LogSnapshotComplete(repoID string, fileCount int, duration time.Duration, totalSize int64) {
	if pm.logger == nil {
		return
	}

	avgSpeed := float64(totalSize) / duration.Seconds()
	logger := pm.logger.
		WithField("repo_id", repoID).
		WithField("file_count", fileCount).
		WithField("duration_ms", duration.Milliseconds()).
		WithField("total_size", totalSize).
		WithField("avg_speed", avgSpeed).
		WithField("operation", "snapshot_download")
	logger.Info("Snapshot download completed successfully")
}

// LogError logs an error with appropriate context
func (pm *ProgressManager) LogError(operation, repoID string, err error) {
	if pm.logger == nil {
		return
	}

	logger := pm.logger.
		WithField("operation", operation).
		WithField("repo_id", repoID).
		WithError(err)
	logger.Error("Operation failed")
}

// LogRepoListing logs repository file listing operations
func (pm *ProgressManager) LogRepoListing(repoID string, fileCount int) {
	if pm.logger == nil {
		return
	}

	logger := pm.logger.
		WithField("repo_id", repoID).
		WithField("file_count", fileCount).
		WithField("operation", "list_files")
	logger.Info("Repository files listed successfully")
}

// ProgressWriter wraps a progress bar as an io.Writer for seamless integration
type ProgressWriter struct {
	bar    *progressbar.ProgressBar
	writer io.Writer
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(bar *progressbar.ProgressBar, writer io.Writer) *ProgressWriter {
	return &ProgressWriter{
		bar:    bar,
		writer: writer,
	}
}

// Write implements io.Writer interface
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if err == nil && pw.bar != nil {
		_ = pw.bar.Add(n) // Ignore error from progress bar update
	}
	return n, err
}

// formatSize formats bytes into human readable format
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
