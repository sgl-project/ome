//go:build linux || darwin

package printers

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestTerminalWidthReadsPseudoTerminal(t *testing.T) {
	const helperEnvironment = "OME_TEST_TERMINAL_WIDTH_PTY"
	if os.Getenv(helperEnvironment) == "1" {
		width, terminal := TerminalWidth(os.Stdout)
		if !terminal || width != 93 {
			os.Exit(1)
		}
		return
	}
	if runtime.GOOS == "darwin" {
		command := exec.Command(
			"/usr/bin/script", "-q", "-e", "/dev/null", "/bin/sh", "-c",
			`stty cols 93; exec "$1" -test.run '^TestTerminalWidthReadsPseudoTerminal$' -test.count=1`,
			"sh", os.Args[0],
		)
		command.Env = append(os.Environ(), helperEnvironment+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "pseudo-terminal helper output: %s", output)
		return
	}

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if os.IsNotExist(err) {
		t.Skip("pseudo-terminal multiplexer is unavailable")
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, master.Close()) })
	require.NoError(t, unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: 24,
		Col: 93,
	}))

	width, terminal := TerminalWidth(master)

	assert.Equal(t, 93, width)
	assert.True(t, terminal)
}

func TestTerminalWidthUnavailableForPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	width, terminal := TerminalWidth(writer)

	assert.Zero(t, width)
	assert.False(t, terminal)
}
