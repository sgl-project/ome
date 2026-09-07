//go:build !linux && !darwin && !windows

package printers

func terminalWidthForFD(uintptr) (int, bool) {
	return 0, false
}
