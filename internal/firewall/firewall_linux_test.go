//go:build linux

package firewall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runScript runs script with fake iptables/ip6tables on PATH. A fake fails
// (exit 1) for any argument list containing one of fail; every call is logged.
func runScript(t *testing.T, script string, fail ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	logf := filepath.Join(dir, "calls.log")
	var cases strings.Builder
	for _, f := range fail {
		cases.WriteString("  *\"" + f + "\"*) exit 1;;\n")
	}
	for _, tool := range []string{"iptables", "ip6tables"} {
		fake := "#!/bin/sh\necho \"$(basename \"$0\") $*\" >> " + logf + "\ncase \"$(basename \"$0\") $*\" in\n" + cases.String() + "esac\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, tool), []byte(fake), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	err := cmd.Run()
	data, _ := os.ReadFile(logf)
	return string(data), err
}

// Any failing rule command must fail the script, not just the last one.
func TestApplyScriptFailsOnEarlierError(t *testing.T) {
	script := applyScript([]string{"192.0.2.1", "192.0.2.2"})
	for _, fail := range []string{"-I OUTPUT", "-A DBD_REGION -d 192.0.2.1 "} {
		if _, err := runScript(t, script, "-C OUTPUT", fail); err == nil {
			t.Errorf("script exited 0 although %q failed", fail)
		}
	}
}

func TestApplyScriptSucceedsWhenChainExists(t *testing.T) {
	// -N fails when the chain already exists; that is not an error.
	if _, err := runScript(t, applyScript([]string{"192.0.2.1"}), "-N DBD_REGION"); err != nil {
		t.Fatal(err)
	}
}

func TestApplyScriptBlocksIPv6WithIP6tables(t *testing.T) {
	// "-C OUTPUT" fails: the jump to the chain does not exist yet, so the
	// script must insert it for both families.
	log, err := runScript(t, applyScript([]string{"192.0.2.1", "2001:db8::1", "not-an-ip"}), "-C OUTPUT")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"iptables -A DBD_REGION -d 192.0.2.1 -j DROP", "iptables -I OUTPUT -j DBD_REGION",
		"ip6tables -A DBD_REGION -d 2001:db8::1 -j DROP", "ip6tables -I OUTPUT -j DBD_REGION",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in:\n%s", want, log)
		}
	}
	if strings.Contains(log, "not-an-ip") || strings.Contains(log, "iptables -A DBD_REGION -d 2001") {
		t.Errorf("address routed to the wrong tool or invalid entry passed through:\n%s", log)
	}
}

func TestClearScriptToleratesMissingChains(t *testing.T) {
	if _, err := runScript(t, clearScript(), "DBD_REGION"); err != nil {
		t.Fatalf("clear failed on missing chains: %v", err)
	}
}
