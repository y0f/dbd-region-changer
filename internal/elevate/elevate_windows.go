//go:build windows

package elevate

import "golang.org/x/sys/windows"

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// WriteHosts edits the hosts file directly; the process is already elevated by the manifest.
func WriteHosts(p Payload) error {
	return applyPayload(p)
}
