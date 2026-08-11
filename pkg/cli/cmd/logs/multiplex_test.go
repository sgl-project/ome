package logs

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rc(s string) io.ReadCloser { return io.NopCloser(strings.NewReader(s)) }

func TestMultiplexPrefixesEveryLine(t *testing.T) {
	var out bytes.Buffer
	err := multiplex([]namedStream{
		{Prefix: "[engine/p1] ", Reader: rc("a\nb\n")},
		{Prefix: "[decoder/p2] ", Reader: rc("c\n")},
	}, &out)
	require.NoError(t, err)
	got := out.String()
	assert.Contains(t, got, "[engine/p1] a\n")
	assert.Contains(t, got, "[engine/p1] b\n")
	assert.Contains(t, got, "[decoder/p2] c\n")
}

func TestMultiplexSingleStreamNoPrefix(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, multiplex([]namedStream{{Prefix: "", Reader: rc("raw\n")}}, &out))
	assert.Equal(t, "raw\n", out.String())
}

func TestMultiplexDoesNotInterleaveWithinALine(t *testing.T) {
	long := strings.Repeat("x", 64*1024) // exceeds default bufio.Scanner token size guard
	var out bytes.Buffer
	require.NoError(t, multiplex([]namedStream{{Prefix: "[e/p] ", Reader: rc(long + "\n")}}, &out))
	assert.Equal(t, "[e/p] "+long+"\n", out.String())
}
