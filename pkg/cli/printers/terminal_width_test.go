package printers

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerminalWidthUnavailableForBuffer(t *testing.T) {
	width, terminal := TerminalWidth(&bytes.Buffer{})

	assert.Zero(t, width)
	assert.False(t, terminal)
}

func TestTerminalWidthUsesProvider(t *testing.T) {
	width, terminal := TerminalWidth(&providerWithFD{terminalBuffer: terminalBuffer{width: 91, terminal: true}})

	assert.Equal(t, 91, width)
	assert.True(t, terminal)
}

type providerWithFD struct {
	terminalBuffer
}

func (providerWithFD) Fd() uintptr {
	panic("provider must take precedence over file descriptor probing")
}

func TestTerminalWidthRejectsInvalidProviderValues(t *testing.T) {
	for _, writer := range []*terminalBuffer{
		{width: 0, terminal: true},
		{width: -1, terminal: true},
		{width: 80, terminal: false},
	} {
		width, terminal := TerminalWidth(writer)
		assert.Zero(t, width)
		assert.False(t, terminal)
	}
}
