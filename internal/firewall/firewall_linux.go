//go:build linux

package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

const chain = "DBD_REGION"

// Supported reports whether iptables is available to block with.
func Supported() bool {
	_, err := exec.LookPath("iptables")
	return err == nil
}

// NeedsProgram is false on Linux: iptables blocks the beacon IPs system-wide
// (those IPs are GameLift-only), so no game-exe path is required.
func NeedsProgram() bool { return false }

// RefreshSupported is false on Linux: each apply needs a pkexec prompt, so a
// silent 60s refresh is not possible.
func RefreshSupported() bool { return false }

// Apply rebuilds the block chain with ips in a single elevated (pkexec) call.
func Apply(program string, ips []string) error {
	return pkexecScript(clearScript() + setupScript() + dropScript(ips))
}

// Add appends drops without tearing down the chain.
func Add(program string, ips []string) error {
	return pkexecScript(setupScript() + dropScript(ips))
}

// Clear removes the block chain.
func Clear() error { return pkexecScript(clearScript()) }

func setupScript() string {
	return fmt.Sprintf("iptables -N %s 2>/dev/null; iptables -C OUTPUT -j %s 2>/dev/null || iptables -I OUTPUT -j %s; ",
		chain, chain, chain)
}

func clearScript() string {
	return fmt.Sprintf("iptables -F %s 2>/dev/null; iptables -D OUTPUT -j %s 2>/dev/null; iptables -X %s 2>/dev/null; ",
		chain, chain, chain)
}

func dropScript(ips []string) string {
	var b strings.Builder
	for _, ip := range ips {
		fmt.Fprintf(&b, "iptables -A %s -d %s -j DROP; ", chain, ip)
	}
	return b.String()
}

func pkexecScript(script string) error {
	cmd := exec.Command("pkexec", "sh", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec iptables failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
