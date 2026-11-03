//go:build linux

package firewall

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

const chain = "DBD_REGION"

// Supported reports whether iptables is available to block with. The GUI runs
// unprivileged, and on Debian-style systems /usr/sbin is not on a user's PATH,
// so also probe the sbin locations (pkexec runs the script with a root PATH).
func Supported() bool {
	if _, err := exec.LookPath("iptables"); err == nil {
		return true
	}
	for _, p := range []string{"/usr/sbin/iptables", "/sbin/iptables"} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// NeedsProgram is false on Linux: iptables blocks the beacon IPs system-wide
// (those IPs are GameLift-only), so no game-exe path is required.
func NeedsProgram() bool { return false }

// RefreshSupported is false on Linux: each apply needs a pkexec prompt, so a
// silent 60s refresh is not possible.
func RefreshSupported() bool { return false }

// Apply rebuilds the block chains with ips in a single elevated (pkexec) call.
// IPv4 addresses go to iptables, IPv6 addresses to ip6tables. It returns an
// ErrNotRun error (ErrNotAuthorized for a refused or dismissed prompt) if the
// script never ran; after any other error no block is left.
func Apply(program string, ips []string) error {
	return pkexecScript(applyScript(ips))
}

// Add appends drops without tearing down the chains.
func Add(program string, ips []string) error {
	return pkexecScript(onFailure("") + addScript(ips))
}

// Clear removes the block chains.
func Clear() error { return pkexecScript(clearScript()) }

// The scripts run under "set -e", so any failing rule command fails the whole
// call (and Apply/Add return an error) instead of only the last one counting.
// Clearing is best effort: the chain may not exist. ip6tables is skipped when
// the host has no usable IPv6 filter table, since then no IPv6 path exists.
// -w waits for the xtables lock instead of failing while another tool holds it.

const ip6Usable = "ip6tables -w -L OUTPUT -n >/dev/null 2>&1"

// applyScript replaces the chains; if a rule command fails it removes the
// partial block again, so a failed Apply leaves none.
func applyScript(ips []string) string {
	return onFailure(clearScript()) + clearScript() + addScript(ips)
}

// onFailure runs cleanup if the script fails and then exits 1, so a failing
// command (sh exits 127 for a missing one) is never mistaken for pkexec's own
// 126/127, which mean the prompt was refused or dismissed.
func onFailure(cleanup string) string {
	return "trap 'if [ $? -ne 0 ]; then set +e; " + cleanup + "exit 1; fi' EXIT; "
}

func addScript(ips []string) string {
	v4, v6 := splitFamilies(ips)
	s := "set -e; " + setupScript("iptables") + dropScript("iptables", v4)
	if len(v6) > 0 {
		s += "if " + ip6Usable + "; then " + setupScript("ip6tables") + dropScript("ip6tables", v6) + "fi; "
	}
	return s
}

func setupScript(tool string) string {
	return fmt.Sprintf("%[1]s -w -N %[2]s 2>/dev/null || true; %[1]s -w -C OUTPUT -j %[2]s 2>/dev/null || %[1]s -w -I OUTPUT -j %[2]s; ",
		tool, chain)
}

func clearScript() string {
	var b strings.Builder
	for _, tool := range []string{"iptables", "ip6tables"} {
		fmt.Fprintf(&b, "%[1]s -w -F %[2]s 2>/dev/null; %[1]s -w -D OUTPUT -j %[2]s 2>/dev/null; %[1]s -w -X %[2]s 2>/dev/null; ",
			tool, chain)
	}
	return b.String() + "true; "
}

func dropScript(tool string, ips []string) string {
	var b strings.Builder
	for _, ip := range ips {
		fmt.Fprintf(&b, "%s -w -A %s -d %s -j DROP; ", tool, chain, ip)
	}
	return b.String()
}

// splitFamilies separates IPv4 from IPv6 addresses; unparsable entries are dropped
// so nothing but an address ever reaches the shell.
func splitFamilies(ips []string) (v4, v6 []string) {
	for _, s := range ips {
		ip := net.ParseIP(s)
		switch {
		case ip == nil:
		case ip.To4() != nil:
			v4 = append(v4, ip.String())
		default:
			v6 = append(v6, ip.String())
		}
	}
	return v4, v6
}

// pkexec is the program that runs the scripts as root; replaceable in tests.
var pkexec = "pkexec"

func pkexecScript(script string) error {
	cmd := exec.Command(pkexec, "sh", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			// pkexec could not be started (not installed, not executable).
			return fmt.Errorf("pkexec iptables: %w: %v", ErrNotRun, err)
		}
		if ee.ExitCode() == 126 || ee.ExitCode() == 127 {
			return fmt.Errorf("pkexec iptables: %w: %s", ErrNotAuthorized, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("pkexec iptables failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
