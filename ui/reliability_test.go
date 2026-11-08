package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/y0f/dbd-region-changer/internal/firewall"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

func TestFailedCleanupNeverClaimsAutomatic(t *testing.T) {
	for _, failure := range []string{"firewall", "hosts"} {
		t.Run(failure, func(t *testing.T) {
			fs := newFakeSystem(t, true)
			if failure == "firewall" {
				fs.clearErr = errors.New("access denied")
			} else {
				fs.hostsErr = errors.New("hosts read-only")
			}
			a := newTestApp(t, []region.Server{{Code: "eu-west-1"}},
				stubDoH(t, func(string) []string { return nil }), "192.0.2.1 gamelift.eu-west-1.amazonaws.com\n")
			a.appliedCode = "eu-west-1"
			if err := a.clearFirewall(); err == nil {
				t.Fatal("cleanup failure was hidden")
			}
			a.refreshCurrent()
			if !strings.Contains(a.currentLabel.Text, "unknown") || loadApplied() != unknownRegion {
				t.Fatalf("unknown state lost: label=%q, saved=%q", a.currentLabel.Text, loadApplied())
			}
			a.refreshBlock()
			if len(fs.added) != 0 {
				t.Fatal("refreshed an unknown block")
			}
		})
	}
}

func TestFailedApplyWithFailedCleanupKeepsUnknownState(t *testing.T) {
	fs := newFakeSystem(t, true)
	fs.applyErr, fs.clearErr = errors.New("apply failed"), errors.New("clear failed")
	a := newTestApp(t, nil, stubDoH(t, func(string) []string { return nil }), "")
	a.replaceBlock(a.programPath(), "eu-west-1", []string{"192.0.2.1"})
	if code, _ := a.state(); code != unknownRegion {
		t.Fatalf("state = %q", code)
	}
}

func TestStagingFailureKeepsPreviousLock(t *testing.T) {
	fs := newFakeSystem(t, true)
	fs.applyErr = firewall.ErrNotRun
	a := newTestApp(t, nil, stubDoH(t, func(string) []string { return nil }), "")
	a.appliedCode = "eu-west-1"
	a.replaceBlock(a.programPath(), "us-east-1", []string{"192.0.2.1"})
	if code, _ := a.state(); code != "eu-west-1" || fs.clears != 0 {
		t.Fatalf("previous lock changed: code=%q clears=%d", code, fs.clears)
	}
}

func TestBeginApplyRejectsDuplicateActions(t *testing.T) {
	a := newTestApp(t, nil, stubDoH(t, func(string) []string { return nil }), "")
	a.buildUI()
	if !a.beginApply() || a.beginApply() {
		t.Fatal("operation guard failed")
	}
	if !a.selectWidget.Disabled() || !a.setBtn.Disabled() || !a.defBtn.Disabled() {
		t.Fatal("controls remained enabled")
	}
}

func TestPathCannotChangeWithActiveLock(t *testing.T) {
	a := newTestApp(t, nil, stubDoH(t, func(string) []string { return nil }), "")
	a.appliedCode = "eu-west-1"
	if err := a.setDBDPath(a.programPath()); err == nil {
		t.Fatal("changed executable with rules still scoped to the previous path")
	}
}

func TestPingRowsDistinguishUnavailableAndTCP(t *testing.T) {
	regions := []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}
	rows := pingRows(regions, []ping.Result{{}, {OK: true, RTTAvgMs: 42, Fallback: true}})
	if rows[0].Latency != "—" || rows[0].Loss != "No response" {
		t.Fatalf("failed ping = %+v", rows[0])
	}
	if rows[1].Loss != "TCP estimate" || rows[1].Latency != "42 ms" {
		t.Fatalf("TCP fallback = %+v", rows[1])
	}
}
