//go:build windows

package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// fakeNetsh points netshPath at a netsh.bat that logs each call and exits 1 for
// "add" when failAdd is set, so no real firewall rule is touched.
func fakeNetsh(t *testing.T, failAdd bool) (logf string) {
	t.Helper()
	dir := t.TempDir()
	logf = filepath.Join(dir, "calls.log")
	bat := "@echo off\r\necho %* >> \"" + logf + "\"\r\n"
	if failAdd {
		bat += "if \"%3\"==\"add\" exit /b 1\r\n"
	}
	bat += "exit /b 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "netsh.bat"), []byte(bat), 0o755); err != nil {
		t.Fatal(err)
	}
	old := netshPath
	netshPath = filepath.Join(dir, "netsh.bat")
	t.Cleanup(func() { netshPath = old })
	return logf
}

func calls(t *testing.T, logf string) []string {
	t.Helper()
	data, _ := os.ReadFile(logf)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// The elevated process must run the system netsh.exe, never one found earlier
// on PATH.
func TestNetshIgnoresPath(t *testing.T) {
	sys, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(netshPath, filepath.Join(sys, "netsh.exe")) {
		t.Fatalf("netshPath = %q, want netsh.exe in %q", netshPath, sys)
	}
	dir := t.TempDir()
	logf := filepath.Join(dir, "calls.log")
	bat := "@echo off\r\necho %* >> \"" + logf + "\"\r\nexit /b 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "netsh.bat"), []byte(bat), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, _ = runNetsh("advfirewall", "show", "currentprofile", "state")
	if _, err := os.Stat(logf); err == nil {
		t.Fatal("runNetsh ran netsh.bat from PATH")
	}
}

// deletesLive reports whether call deletes the live (non-pending) rule set.
func deletesLive(call string) bool {
	return strings.Contains(call, "delete rule") && strings.Contains(call, "name="+ruleName+`"`)
}

// A failed add must leave the previous block in place.
func TestApplyKeepsOldRulesWhenAddFails(t *testing.T) {
	logf := fakeNetsh(t, true)
	if err := Apply(`C:\fake\DeadByDaylight.exe`, []string{"192.0.2.1", "192.0.2.2"}); err == nil {
		t.Fatal("expected add failure")
	}
	for _, c := range calls(t, logf) {
		if deletesLive(c) {
			t.Fatalf("old rules deleted although the replacement failed: %q", c)
		}
		if strings.Contains(c, "add rule") && !strings.Contains(c, pendingRuleName) {
			t.Fatalf("added directly to the live rule name: %q", c)
		}
	}
}

func TestApplyStagesThenSwaps(t *testing.T) {
	logf := fakeNetsh(t, false)
	if err := Apply(`C:\fake\DeadByDaylight.exe`, []string{"192.0.2.1", "2001:db8::1"}); err != nil {
		t.Fatal(err)
	}
	cs := calls(t, logf)
	addAt, delAt, renameAt := -1, -1, -1
	for i, c := range cs {
		switch {
		case strings.Contains(c, "add rule"):
			addAt = i
			if !strings.Contains(c, "remoteip=192.0.2.1,2001:db8::1") {
				t.Errorf("add missing addresses: %q", c)
			}
		case deletesLive(c):
			delAt = i
		case strings.Contains(c, "set rule") && strings.Contains(c, "new"):
			renameAt = i
		}
	}
	if addAt < 0 || delAt < addAt || renameAt < delAt {
		t.Fatalf("want add, delete old, rename in order; calls:\n%s", strings.Join(cs, "\n"))
	}
}
