package hub

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/logging"
)

func TestNewProgressManager(t *testing.T) {
	logger := logging.Discard()

	// Test creating progress manager with all options
	pm := NewProgressManager(logger, true, true)
	assert.NotNil(t, pm)
	assert.Equal(t, logger, pm.logger)
	assert.True(t, pm.enableProgressBars)
	assert.True(t, pm.enableDetailedLogs)

	// Test creating progress manager with bars disabled
	pm = NewProgressManager(logger, false, true)
	assert.NotNil(t, pm)
	assert.False(t, pm.enableProgressBars)
	assert.True(t, pm.enableDetailedLogs)
}

func TestCreateFileProgressBar(t *testing.T) {
	logger := logging.Discard()

	// Test with progress bars enabled
	pm := NewProgressManager(logger, true, false)
	bar := pm.CreateFileProgressBar("test_file.json", 1024)
	assert.NotNil(t, bar)

	// Test with progress bars disabled (log mode)
	pm = NewProgressManager(logger, false, false)
	bar = pm.CreateFileProgressBar("test_file.json", 1024)
	// In log mode, we still get a progress bar instance (LogProgressBar)
	assert.NotNil(t, bar)
	// Verify it's a LogProgressBar by checking if it implements the interface
	_, ok := bar.(*LogProgressBar)
	assert.True(t, ok, "Expected LogProgressBar when progress bars are disabled")

	// Test with long filename
	pm = NewProgressManager(logger, true, false)
	longFilename := strings.Repeat("a", 100) + ".json"
	bar = pm.CreateFileProgressBar(longFilename, 1024)
	assert.NotNil(t, bar)
}

func TestCreateSnapshotProgressBar(t *testing.T) {
	logger := logging.Discard()

	// Test with progress bars enabled
	pm := NewProgressManager(logger, true, false)
	bar := pm.CreateSnapshotProgressBar(10, 1024*1024)
	assert.NotNil(t, bar)

	// Test with progress bars disabled (log mode)
	pm = NewProgressManager(logger, false, false)
	bar = pm.CreateSnapshotProgressBar(10, 1024*1024)
	// In log mode, we still get a progress bar instance (LogProgressBar)
	assert.NotNil(t, bar)
	// Verify it's a LogProgressBar
	_, ok := bar.(*LogProgressBar)
	assert.True(t, ok, "Expected LogProgressBar when progress bars are disabled")
}

func TestCreateSpinner(t *testing.T) {
	logger := logging.Discard()

	// Test with progress bars enabled
	pm := NewProgressManager(logger, true, false)
	spinner := pm.CreateSpinner("Loading...")
	assert.NotNil(t, spinner)

	// Test with progress bars disabled
	pm = NewProgressManager(logger, false, false)
	spinner = pm.CreateSpinner("Loading...")
	assert.Nil(t, spinner)
}

func TestProgressManagerLogging(t *testing.T) {
	// Create a mock logger that captures log messages
	var logBuffer bytes.Buffer
	logger := &mockLogger{buffer: &logBuffer}

	// Test logging with detailed logs enabled
	pm := NewProgressManager(logger, true, true)

	// Test download start logging
	pm.LogDownloadStart("test/repo", "file.json", 1024)
	assert.Contains(t, logBuffer.String(), "test/repo")
	assert.Contains(t, logBuffer.String(), "file.json")
	logBuffer.Reset()

	// Test download complete logging
	pm.LogDownloadComplete("test/repo", "file.json", time.Second, 1024)
	assert.Contains(t, logBuffer.String(), "test/repo")
	logBuffer.Reset()

	// Test snapshot start logging
	pm.LogSnapshotStart("test/repo", 5, 5120)
	assert.Contains(t, logBuffer.String(), "test/repo")
	logBuffer.Reset()

	// Test snapshot complete logging
	pm.LogSnapshotComplete("test/repo", 5, time.Second*30, 5120)
	assert.Contains(t, logBuffer.String(), "test/repo")
	logBuffer.Reset()

	// Test error logging
	pm.LogError("download", "test/repo", assert.AnError)
	assert.Contains(t, logBuffer.String(), "test/repo")
	logBuffer.Reset()

	// Test repo listing logging
	pm.LogRepoListing("test/repo", 10)
	assert.Contains(t, logBuffer.String(), "test/repo")
}

func TestProgressManagerWithNilLogger(t *testing.T) {
	// Test that logging functions don't panic with nil logger
	pm := NewProgressManager(nil, true, true)

	// These should not panic
	pm.LogDownloadStart("test/repo", "file.json", 1024)
	pm.LogDownloadComplete("test/repo", "file.json", time.Second, 1024)
	pm.LogSnapshotStart("test/repo", 5, 5120)
	pm.LogSnapshotComplete("test/repo", 5, time.Second*30, 5120)
	pm.LogError("download", "test/repo", assert.AnError)
	pm.LogRepoListing("test/repo", 10)
}

func TestProgressWriter(t *testing.T) {
	// Create a buffer to capture writes
	var buf bytes.Buffer

	// Create a mock progress bar and wrap it
	rawBar := progressbar.DefaultBytes(100, "test")
	bar := &SingleProgressBar{bar: rawBar}

	// Create progress writer
	pw := NewProgressWriter(bar, &buf)
	assert.NotNil(t, pw)

	// Test writing data
	data := []byte("test data")
	n, err := pw.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, data, buf.Bytes())

	// Test nil progress bar
	pw2 := NewProgressWriter(nil, &buf)
	_, _ = pw2.Write(data) // Ignore return values for test
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1536, // 1.5 KB
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024 * 2, // 2 MB
			expected: "2.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024 * 3, // 3 GB
			expected: "3.0 GB",
		},
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock logger for testing
type mockLogger struct {
	buffer *bytes.Buffer
	fields map[string]interface{}
}

func (m *mockLogger) WithField(key string, value interface{}) logging.Interface {
	newLogger := &mockLogger{
		buffer: m.buffer,
		fields: make(map[string]interface{}),
	}
	for k, v := range m.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

func (m *mockLogger) WithError(err error) logging.Interface {
	return m.WithField("error", err)
}

func (m *mockLogger) Debug(msg string) {
	m.buffer.WriteString("DEBUG: ")
	m.buffer.WriteString(msg)
	m.writeFields()
}

func (m *mockLogger) Info(msg string) {
	m.buffer.WriteString("INFO: ")
	m.buffer.WriteString(msg)
	m.writeFields()
}

func (m *mockLogger) Warn(msg string) {
	m.buffer.WriteString("WARN: ")
	m.buffer.WriteString(msg)
	m.writeFields()
}

func (m *mockLogger) Error(msg string) {
	m.buffer.WriteString("ERROR: ")
	m.buffer.WriteString(msg)
	m.writeFields()
}

func (m *mockLogger) Fatal(msg string) {
	m.buffer.WriteString("FATAL: ")
	m.buffer.WriteString(msg)
	m.writeFields()
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	m.buffer.WriteString("DEBUGF: ")
	m.buffer.WriteString(format)
	m.writeFields()
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	m.buffer.WriteString("INFOF: ")
	m.buffer.WriteString(format)
	m.writeFields()
}

func (m *mockLogger) Warnf(format string, args ...interface{}) {
	m.buffer.WriteString("WARNF: ")
	m.buffer.WriteString(format)
	m.writeFields()
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	m.buffer.WriteString("ERRORF: ")
	m.buffer.WriteString(format)
	m.writeFields()
}

func (m *mockLogger) Fatalf(format string, args ...interface{}) {
	m.buffer.WriteString("FATALF: ")
	m.buffer.WriteString(format)
	m.writeFields()
}

func (m *mockLogger) writeFields() {
	for k, v := range m.fields {
		m.buffer.WriteString(" ")
		m.buffer.WriteString(k)
		m.buffer.WriteString("=")
		switch val := v.(type) {
		case string:
			m.buffer.WriteString(val)
		case error:
			m.buffer.WriteString(val.Error())
		default:
			m.buffer.WriteString("unknown")
		}
	}
	m.buffer.WriteString("\n")
}

// Benchmark tests
func BenchmarkFormatSize(b *testing.B) {
	sizes := []int64{
		512,
		1536,
		1024 * 1024 * 2,
		1024 * 1024 * 1024 * 3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			formatSize(size)
		}
	}
}

func BenchmarkProgressWriter(b *testing.B) {
	var buffer bytes.Buffer
	rawBar := progressbar.NewOptions64(1024*1024, progressbar.OptionSetWidth(10))
	bar := &SingleProgressBar{bar: rawBar}
	pw := NewProgressWriter(bar, &buffer)
	data := []byte("test data for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Reset()
		_, _ = pw.Write(data) // Ignore return values for benchmark
	}
}
