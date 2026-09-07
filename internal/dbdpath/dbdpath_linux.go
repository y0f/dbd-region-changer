//go:build linux

// Package dbdpath locates the Dead by Daylight executable.
package dbdpath

// Find is a stub on Linux: iptables blocks the beacon IPs system-wide, so no
// game-exe path is needed.
func Find() string { return "" }
