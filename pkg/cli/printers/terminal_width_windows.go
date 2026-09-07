//go:build windows

package printers

import "golang.org/x/sys/windows"

func terminalWidthForFD(fd uintptr) (int, bool) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, false
	}
	width := int(info.Window.Right) - int(info.Window.Left) + 1
	if width <= 0 {
		return 0, false
	}
	return width, true
}
