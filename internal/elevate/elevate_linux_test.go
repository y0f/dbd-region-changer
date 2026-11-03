//go:build linux

package elevate

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeTools puts fake pkexec (exiting with pkexecExit, or absent when empty) and
// sudo (touching a marker) first on PATH and returns the marker path.
func fakeTools(t *testing.T, pkexecExit string) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "sudo-called")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if pkexecExit != "" {
		write("pkexec", "#!/bin/sh\nexit "+pkexecExit+"\n")
	}
	write("sudo", "#!/bin/sh\n: > "+marker+"\nexit 0\n")
	t.Setenv("PATH", dir)
	return marker
}

// A dismissed (126), refused (127) or failed (1) pkexec run must not fall back to sudo.
func TestWriteHostsNoSudoAfterPkexecFailure(t *testing.T) {
	for _, code := range []string{"126", "127", "1"} {
		marker := fakeTools(t, code)
		if err := WriteHosts(Payload{Op: OpClear}); err == nil {
			t.Errorf("pkexec exit %s: expected error", code)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Errorf("pkexec exit %s: sudo was invoked", code)
		}
	}
}

func TestWriteHostsFallsBackToSudoWithoutPkexec(t *testing.T) {
	marker := fakeTools(t, "")
	if err := WriteHosts(Payload{Op: OpClear}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("sudo not used although pkexec is missing")
	}
}
