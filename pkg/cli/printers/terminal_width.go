package printers

import "io"

type terminalWidthProvider interface {
	TerminalWidth() (int, bool)
}

type fileDescriptor interface {
	Fd() uintptr
}

// TerminalWidth returns the visible width of an actual terminal. Redirected
// files, pipes, and writers without terminal metadata report unavailable so
// their deterministic legacy table output remains unchanged.
func TerminalWidth(w io.Writer) (int, bool) {
	if provider, ok := w.(terminalWidthProvider); ok {
		width, terminal := provider.TerminalWidth()
		if !terminal || width <= 0 {
			return 0, false
		}
		return width, true
	}
	fd, ok := w.(fileDescriptor)
	if !ok {
		return 0, false
	}
	return terminalWidthForFD(fd.Fd())
}
