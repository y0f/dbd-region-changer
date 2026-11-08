//go:build linux

package firewall

import (
	"errors"
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
		"iptables -w -A DBD_REGION -d 192.0.2.1 -j DROP", "iptables -w -I OUTPUT -j DBD_REGION",
		"ip6tables -w -A DBD_REGION -d 2001:db8::1 -j DROP", "ip6tables -w -I OUTPUT -j DBD_REGION",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in:\n%s", want, log)
		}
	}
	if strings.Contains(log, "not-an-ip") || strings.Contains(log, "iptables -w -A DBD_REGION -d 2001") {
		t.Errorf("address routed to the wrong tool or invalid entry passed through:\n%s", log)
	}
}

func TestClearScriptToleratesMissingChains(t *testing.T) {
	if _, err := runScript(t, clearScript(), "DBD_REGION"); err != nil {
		t.Fatalf("clear failed on missing chains: %v", err)
	}
}

// A rule command failing after the old chain was flushed must not leave half
// a block behind.
func TestApplyScriptRemovesPartialBlockOnFailure(t *testing.T) {
	log, err := runScript(t, applyScript([]string{"192.0.2.1", "192.0.2.2"}),
		"-A DBD_REGION -d 192.0.2.2 ")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit status 1", err)
	}
	_, after, _ := strings.Cut(log, "-A DBD_REGION -d 192.0.2.2 ")
	for _, want := range []string{"iptables -w -F DBD_REGION", "iptables -w -D OUTPUT -j DBD_REGION", "iptables -w -X DBD_REGION"} {
		if !strings.Contains(after, want) {
			t.Errorf("missing %q after the failed rule in:\n%s", want, log)
		}
	}
}

func TestClearReportsFailures(t *testing.T) {
	for _, failure := range []string{"iptables -w -S", "-F DBD_REGION", "-D OUTPUT", "-X DBD_REGION"} {
		if _, err := runScript(t, clearScript(), failure); err == nil {
			t.Errorf("clear hid %q failure", failure)
		}
	}
}

func TestCleanupFailureReportsUnknownState(t *testing.T) {
	fakePkexec(t, "exit 2")
	if err := pkexecScript("true"); !errors.Is(err, ErrStateUnknown) {
		t.Fatalf("got %v, want unknown firewall state", err)
	}
}

// Every iptables call waits for the xtables lock instead of failing on it.
func TestScriptsWaitForXtablesLock(t *testing.T) {
	log, err := runScript(t, applyScript([]string{"192.0.2.1", "2001:db8::1"}), "-C OUTPUT")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if !strings.Contains(line, " -w ") {
			t.Errorf("call without -w: %q", line)
		}
	}
}

// A failing script must not exit 126/127, which mean a refused prompt.
func TestFailedScriptExitsOne(t *testing.T) {
	err := exec.Command("sh", "-c", onFailure("")+"set -e; dbd-region-changer-no-such-command").Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit status 1", err)
	}
}

// fakePkexec replaces pkexec with a shell script body for one test.
func fakePkexec(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pkexec")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := pkexec
	pkexec = p
	t.Cleanup(func() { pkexec = old })
}

func TestPkexecScriptReportsRefusedPrompt(t *testing.T) {
	for _, code := range []string{"126", "127"} {
		fakePkexec(t, "exit "+code)
		if err := pkexecScript("true"); !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("pkexec exit %s: err = %v, want ErrNotAuthorized", code, err)
		}
	}
}

// pkexec that cannot start never ran the script: nothing changed.
func TestPkexecScriptReportsStartFailureAsNotRun(t *testing.T) {
	old := pkexec
	pkexec = filepath.Join(t.TempDir(), "no-such-pkexec")
	t.Cleanup(func() { pkexec = old })
	err := pkexecScript("true")
	if !errors.Is(err, ErrNotRun) || errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotRun only", err)
	}
}

// A refused prompt also never ran the script.
func TestNotAuthorizedIsNotRun(t *testing.T) {
	fakePkexec(t, "exit 126")
	if err := pkexecScript("true"); !errors.Is(err, ErrNotRun) {
		t.Fatalf("err = %v, want ErrNotRun", err)
	}
}

func TestPkexecScriptReportsScriptFailure(t *testing.T) {
	fakePkexec(t, `exec "$@"`)
	err := pkexecScript(onFailure("") + "set -e; dbd-region-changer-no-such-command")
	if err == nil || errors.Is(err, ErrNotRun) {
		t.Fatalf("err = %v, want a failure other than ErrNotRun", err)
	}
}
