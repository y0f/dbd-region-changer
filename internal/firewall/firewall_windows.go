//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// Supported reports that firewall blocking is available.
func Supported() bool { return true }

// NeedsProgram reports that rules are scoped to the game exe (so its path is required).
func NeedsProgram() bool { return true }

// RefreshSupported reports that the block can be refreshed silently (no prompt).
func RefreshSupported() bool { return true }

// pendingRuleName holds a new rule set while it is being added, so the old
// rules stay in force until the replacement is complete.
const pendingRuleName = ruleName + " (pending)"

// Apply replaces the current rules with a block of ips. The new rules are added
// under a staging name first; the old rules are only deleted once every add
// succeeded, so a failed Apply leaves the previous block in place. Requires admin.
func Apply(program string, ips []string) error {
	if len(ips) == 0 {
		return Clear()
	}
	deleteRules(pendingRuleName)
	if err := add(pendingRuleName, program, ips); err != nil {
		deleteRules(pendingRuleName)
		return err
	}
	deleteRules(ruleName)
	if _, err := runNetsh("advfirewall", "firewall", "set", "rule",
		"name="+pendingRuleName, "new", "name="+ruleName); err != nil {
		// The new block is active under the staging name; Clear removes both.
		return err
	}
	return nil
}

// Add appends outbound block rules for ips WITHOUT clearing existing rules.
// Used by the periodic refresh so the block is never momentarily lifted.
func Add(program string, ips []string) error {
	return add(ruleName, program, ips)
}

func add(name, program string, ips []string) error {
	for _, group := range chunk(ips, maxCIDRsPerRule) {
		args := []string{
			"advfirewall", "firewall", "add", "rule",
			"name=" + name,
			"dir=out", "action=block", "enable=yes",
			"remoteip=" + strings.Join(group, ","),
		}
		if program != "" {
			args = append(args, "program="+program)
		}
		if err := addWithRetry(args); err != nil {
			return err
		}
	}
	return nil
}

// Clear removes all rules this tool created.
func Clear() error {
	deleteRules(ruleName)
	deleteRules(pendingRuleName)
	return nil
}

// deleteRules removes every rule called name. A delete with no matching rule
// returns an error; that is not a failure.
func deleteRules(name string) {
	_, _ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+name)
}

// addWithRetry works around transient "Windows Defender Firewall error (0x2)"
// hiccups by retrying a few times.
func addWithRetry(args []string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		if _, err := runNetsh(args...); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// netshPath is netsh.exe in the system directory. It is not looked up on PATH:
// this process runs elevated, so a user-writable PATH entry must not be able to
// supply the binary. Tests point it at a fake.
var netshPath = func() string {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		dir = `C:\Windows\System32`
	}
	return filepath.Join(dir, "netsh.exe")
}()

func runNetsh(args ...string) (string, error) {
	cmd := exec.Command(netshPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("netsh %v failed: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
