//go:build linux || darwin

package printers

import "golang.org/x/sys/unix"

func terminalWidthForFD(fd uintptr) (int, bool) {
	size, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil || size == nil || size.Col == 0 {
		return 0, false
	}
	return int(size.Col), true
}
