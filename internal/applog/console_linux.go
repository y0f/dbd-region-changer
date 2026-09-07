//go:build linux

package applog

// openConsole is a no-op on Linux; stderr is already attached to the terminal.
func openConsole() {}
