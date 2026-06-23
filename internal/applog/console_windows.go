//go:build windows

package applog

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procAllocConsole = kernel32.NewProc("AllocConsole")
)

// openConsole allocates a console for the (GUI) process and reattaches stderr
// so debug logs are visible.
func openConsole() {
	if r, _, _ := procAllocConsole.Call(); r == 0 {
		return
	}
	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 {
		os.Stderr = os.NewFile(uintptr(h), "CONOUT$")
	}
}
