package ociobjectstore

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingDownloadObserver struct {
	mu           sync.Mutex
	observations []DownloadObservation
}

type cappedReader struct {
	reader io.Reader
	max    int
}

func (r *cappedReader) Read(buffer []byte) (int, error) {
	if len(buffer) > r.max {
		buffer = buffer[:r.max]
	}
	return r.reader.Read(buffer)
}

func (o *recordingDownloadObserver) ObserveDownloadPhase(observation DownloadObservation) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.observations = append(o.observations, observation)
}

func TestDownloadObserver(t *testing.T) {
	observer := &recordingDownloadObserver{}
	dataStore := &OCIOSDataStore{}
	dataStore.SetDownloadObserver(observer)

	want := DownloadObservation{
		Phase:      PhaseGetObjectBodyRead,
		Duration:   250 * time.Millisecond,
		Bytes:      1024,
		Outcome:    DownloadOutcomeSuccess,
		ObjectName: "model.bin",
		PartNumber: 3,
		HasPart:    true,
		Attempt:    1,
		ChunkSize:  4096,
	}
	dataStore.observeDownloadPhase(want)

	require.Len(t, observer.observations, 1)
	assert.Equal(t, want, observer.observations[0])
}

func TestNilDownloadObserver(t *testing.T) {
	dataStore := &OCIOSDataStore{}
	assert.NotPanics(t, func() {
		dataStore.observeDownloadPhase(DownloadObservation{Phase: PhaseBulkDownload})
	})
}

func TestTimedReader(t *testing.T) {
	input := bytes.Repeat([]byte("a"), 2*1024)
	reader := newTimedReader(&cappedReader{
		reader: bytes.NewReader(input),
		max:    256,
	}, time.Now())
	var output bytes.Buffer
	buffer := make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_, writeErr := output.Write(buffer[:n])
			require.NoError(t, writeErr)
		}
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	assert.Equal(t, int64(len(input)), reader.bytes)
	assert.Equal(t, int64(len(input)), reader.stats.Bytes)
	assert.Equal(t, int64(9), reader.stats.Calls)
	assert.Equal(t, reader.stats.Calls, reader.stats.ShortReadCalls+reader.stats.ZeroByteReadCalls)
	assert.Equal(t, int64(1024), reader.stats.MaxRequestBytes)
	assert.Equal(t, int64(256), reader.stats.MaxReturnedBytes)
	assert.Equal(t, reader.stats.Calls-reader.stats.ZeroByteReadCalls, reader.stats.CallsUpTo16KiB)
	durationCalls := reader.stats.ReadDurationBuckets.UpTo1ms +
		reader.stats.ReadDurationBuckets.From1To5ms +
		reader.stats.ReadDurationBuckets.From5To20ms +
		reader.stats.ReadDurationBuckets.From20To100ms +
		reader.stats.ReadDurationBuckets.Over100ms
	assert.Equal(t, reader.stats.Calls, durationCalls)
	assert.True(t, reader.observedFirstRead)
	assert.GreaterOrEqual(t, reader.firstReadDuration, time.Duration(0))
	assert.Equal(t, input, output.Bytes())
}
