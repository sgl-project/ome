package hub

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

// Progress represents a simple progress reporter interface
type Progress interface {
	// Update adds to the progress
	Update(bytes int64)
	// Finish completes the progress
	Finish()
}

// NoOpProgress does nothing (for when progress is disabled)
type NoOpProgress struct{}

func (NoOpProgress) Update(int64) {}
func (NoOpProgress) Finish()      {}

// SimpleProgress provides basic console progress output
type SimpleProgress struct {
	name      string
	total     int64
	current   int64
	startTime time.Time
	lastPrint time.Time
	isATTY    bool
	offset    int64 // For resumed downloads
}

// NewProgress creates a progress reporter based on the environment
func NewProgress(name string, total int64, enabled bool) Progress {
	if !enabled {
		return NoOpProgress{}
	}

	// Check if stdout is a terminal
	// Also check for common IDE environments that might not be detected as TTY
	fileInfo, _ := os.Stdout.Stat()
	isATTY := (fileInfo.Mode() & os.ModeCharDevice) != 0

	// Force TTY mode if requested via environment variable
	if os.Getenv("FORCE_PROGRESS_BAR") == "1" {
		isATTY = true
	}

	// Debug: Print whether we're in TTY mode
	if os.Getenv("DEBUG_PROGRESS") == "1" {
		fmt.Printf("[DEBUG] Progress for %s: isATTY=%v, total=%d\n", name, isATTY, total)
	}

	return &SimpleProgress{
		name:      name,
		total:     total,
		startTime: time.Now(),
		lastPrint: time.Now(),
		isATTY:    isATTY,
		offset:    0,
	}
}

// NewProgressWithResume creates a progress reporter for resumed downloads
func NewProgressWithResume(name string, totalSize, resumedSize int64, enabled bool) Progress {
	if !enabled {
		return NoOpProgress{}
	}

	// Check if stdout is a terminal
	fileInfo, _ := os.Stdout.Stat()
	isATTY := (fileInfo.Mode() & os.ModeCharDevice) != 0

	// Force TTY mode if requested via environment variable
	if os.Getenv("FORCE_PROGRESS_BAR") == "1" {
		isATTY = true
	}

	return &SimpleProgress{
		name:      name,
		total:     totalSize,   // Total file size
		current:   0,           // Current download progress (starts at 0)
		offset:    resumedSize, // Already downloaded bytes
		startTime: time.Now(),
		lastPrint: time.Now(),
		isATTY:    isATTY,
	}
}

// Update adds bytes to the progress
func (p *SimpleProgress) Update(bytes int64) {
	atomic.AddInt64(&p.current, bytes)

	// Only print updates every 50ms to avoid spam (more frequent updates)
	if time.Since(p.lastPrint) < 50*time.Millisecond {
		return
	}
	p.lastPrint = time.Now()

	current := atomic.LoadInt64(&p.current)
	p.print(current)
}

// Finish completes the progress
func (p *SimpleProgress) Finish() {
	current := atomic.LoadInt64(&p.current)
	actualProgress := p.offset + current

	if p.isATTY {
		// Clear line and print final status
		fmt.Printf("\r%-50s %s\n", p.name, formatBytes(actualProgress))
	} else {
		// Log mode - just print completion
		elapsed := time.Since(p.startTime)
		speed := float64(current) / elapsed.Seconds()
		if p.offset > 0 {
			fmt.Printf("Downloaded %s: %s (resumed from %s) in %v (%.1f MB/s)\n",
				p.name, formatBytes(actualProgress), formatBytes(p.offset),
				elapsed.Round(time.Second), speed/1024/1024)
		} else {
			fmt.Printf("Downloaded %s: %s in %v (%.1f MB/s)\n",
				p.name, formatBytes(actualProgress), elapsed.Round(time.Second), speed/1024/1024)
		}
	}
}

// print updates the current progress display
func (p *SimpleProgress) print(current int64) {
	if p.isATTY {
		// Terminal mode - use carriage return for updating line
		if p.total > 0 {
			// Account for resumed downloads
			actualProgress := p.offset + current
			percent := float64(actualProgress) * 100 / float64(p.total)
			barWidth := 30
			filled := int(percent * float64(barWidth) / 100)
			bar := string(repeatChar('=', filled)) + string(repeatChar('-', barWidth-filled))

			elapsed := time.Since(p.startTime)
			speed := float64(current) / elapsed.Seconds()

			fmt.Printf("\r%-30s [%s] %3.0f%% %s %.1f MB/s",
				truncate(p.name, 30), bar, percent,
				formatBytes(actualProgress), speed/1024/1024)
		} else {
			// Unknown total size
			fmt.Printf("\r%-50s %s", truncate(p.name, 50), formatBytes(current))
		}
	}
}

// SimpleProgressWriter wraps an io.Writer to report progress
type SimpleProgressWriter struct {
	writer   io.Writer
	progress Progress
}

// NewSimpleProgressWriter creates a writer that reports progress
func NewSimpleProgressWriter(w io.Writer, p Progress) io.Writer {
	if p == nil {
		return w
	}
	return &SimpleProgressWriter{writer: w, progress: p}
}

// Write implements io.Writer
func (pw *SimpleProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if n > 0 {
		pw.progress.Update(int64(n))
	}
	return n, err
}

// Helper functions

func formatBytes(bytes int64) string {
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return "..." + s[len(s)-(max-3):]
}

func repeatChar(char rune, count int) []rune {
	result := make([]rune, count)
	for i := range result {
		result[i] = char
	}
	return result
}

// MultiFileProgress handles progress for multiple concurrent downloads
type MultiFileProgress struct {
	files map[string]Progress
}

// NewMultiFileProgress creates a progress tracker for multiple files
func NewMultiFileProgress() *MultiFileProgress {
	return &MultiFileProgress{
		files: make(map[string]Progress),
	}
}

// StartFile begins tracking a new file download
func (m *MultiFileProgress) StartFile(name string, size int64, enabled bool) Progress {
	p := NewProgress(name, size, enabled)
	m.files[name] = p
	return p
}

// FinishAll completes all progress tracking
func (m *MultiFileProgress) FinishAll() {
	for _, p := range m.files {
		p.Finish()
	}
}
