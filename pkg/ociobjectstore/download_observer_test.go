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
	input := bytes.Repeat([]byte("a"), 32*1024)
	reader := newTimedReader(bytes.NewReader(input), time.Now())
	var output bytes.Buffer

	written, err := io.CopyBuffer(&output, reader, make([]byte, 1024))
	require.NoError(t, err)

	assert.Equal(t, int64(len(input)), written)
	assert.Equal(t, int64(len(input)), reader.bytes)
	assert.True(t, reader.observedFirstRead)
	assert.GreaterOrEqual(t, reader.firstReadDuration, time.Duration(0))
	assert.Equal(t, input, output.Bytes())
}
