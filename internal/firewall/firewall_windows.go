//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Supported reports that firewall blocking is available.
func Supported() bool { return true }

// NeedsProgram reports that rules are scoped to the game exe (so its path is required).
func NeedsProgram() bool { return true }

// RefreshSupported reports that the block can be refreshed silently (no prompt).
func RefreshSupported() bool { return true }

// Apply clears prior rules then blocks cidrs. Requires admin.
func Apply(program string, cidrs []string) error {
	if err := Clear(); err != nil {
		return err
	}
	return Add(program, cidrs)
}

// Add appends outbound block rules for cidrs WITHOUT clearing existing rules.
// Used by the periodic refresh so the block is never momentarily lifted.
func Add(program string, cidrs []string) error {
	for _, group := range chunk(cidrs, maxCIDRsPerRule) {
		args := []string{
			"advfirewall", "firewall", "add", "rule",
			"name=" + ruleName,
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
	// A delete with no matching rule returns an error; that is not a failure.
	_, _ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+ruleName)
	return nil
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

func runNetsh(args ...string) (string, error) {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("netsh %v failed: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
