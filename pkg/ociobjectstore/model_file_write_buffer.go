package ociobjectstore

import (
	"fmt"
	"sync"
)

const (
	// DefaultModelFileWriteBufferSizeBytes preserves the existing 1 MiB
	// coalescing behavior for direct multipart model-file writes.
	DefaultModelFileWriteBufferSizeBytes = 1024 * 1024
	MinModelFileWriteBufferSizeBytes     = 4 * 1024
	MaxModelFileWriteBufferSizeBytes     = 64 * 1024 * 1024
)

// ValidateModelFileWriteBufferSize rejects values that would either defeat
// write coalescing or create excessive per-part-worker memory pressure.
func ValidateModelFileWriteBufferSize(size int) error {
	if size < MinModelFileWriteBufferSizeBytes || size > MaxModelFileWriteBufferSizeBytes {
		return fmt.Errorf(
			"model file write buffer size must be between %d and %d bytes, got %d",
			MinModelFileWriteBufferSizeBytes,
			MaxModelFileWriteBufferSizeBytes,
			size,
		)
	}
	return nil
}

// sizedBufferPool keeps direct-write buffers separate from the package-wide
// pools used by standard downloads and file copies.
type sizedBufferPool struct {
	pool sync.Pool
}

var defaultModelFileWriteBufferPool = newSizedBufferPool(DefaultModelFileWriteBufferSizeBytes)

func newSizedBufferPool(size int) *sizedBufferPool {
	pool := &sizedBufferPool{}
	pool.pool.New = func() interface{} {
		buffer := make([]byte, size)
		return &buffer
	}
	return pool
}

func (pool *sizedBufferPool) get() *[]byte {
	return pool.pool.Get().(*[]byte)
}

func (pool *sizedBufferPool) put(buffer *[]byte) {
	pool.pool.Put(buffer)
}
