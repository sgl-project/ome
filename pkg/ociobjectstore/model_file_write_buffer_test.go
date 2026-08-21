package ociobjectstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateModelFileWriteBufferSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "minimum", size: MinModelFileWriteBufferSizeBytes},
		{name: "default", size: DefaultModelFileWriteBufferSizeBytes},
		{name: "maximum", size: MaxModelFileWriteBufferSizeBytes},
		{name: "below minimum", size: MinModelFileWriteBufferSizeBytes - 1, wantErr: true},
		{name: "above maximum", size: MaxModelFileWriteBufferSizeBytes + 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateModelFileWriteBufferSize(test.size)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSizedBufferPoolUsesConfiguredSize(t *testing.T) {
	const size = 512 * 1024
	pool := newSizedBufferPool(size)
	buffer := pool.get()
	require.Len(t, *buffer, size)
	pool.put(buffer)
}
