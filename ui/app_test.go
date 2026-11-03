package ui

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/y0f/dbd-region-changer/internal/applog"
	"github.com/y0f/dbd-region-changer/internal/doh"
	"github.com/y0f/dbd-region-changer/internal/elevate"
	"github.com/y0f/dbd-region-changer/internal/hostsfile"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

// stubDoH serves DoH JSON answers from answer (nil means HTTP 500). Every
// address is returned for A and AAAA queries alike; the resolver keeps the
// ones of the queried family.
func stubDoH(t *testing.T, answer func(name string) []string) *doh.Resolver {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ips := answer(r.URL.Query().Get("name"))
		if ips == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		type ans struct {
			Data string `json:"data"`
		}
		var out struct {
			Answer []ans `json:"Answer"`
		}
		for _, ip := range ips {
			out.Answer = append(out.Answer, ans{ip})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return doh.New([]doh.Provider{{Name: "stub", URL: srv.URL}})
}

// fakeSystem replaces the firewall and hosts backends and the state file for
// one test, and records the calls.
type fakeSystem struct {
	mu       sync.Mutex
	applied  [][]string
	added    [][]string
	clears   int
	hostsOps int
	applyErr error
}

func newFakeSystem(t *testing.T, silent bool) *fakeSystem {
	t.Helper()
	f := &fakeSystem{}
	stateFile := filepath.Join(t.TempDir(), "applied_region")
	oldSupported, oldSilent, oldApply, oldAdd, oldClear, oldHosts, oldState :=
		fwSupported, fwSilent, fwApply, fwAdd, fwClear, writeHosts, appliedStateFile
	t.Cleanup(func() {
		fwSupported, fwSilent, fwApply, fwAdd, fwClear, writeHosts, appliedStateFile =
			oldSupported, oldSilent, oldApply, oldAdd, oldClear, oldHosts, oldState
	})
	fwSupported = func() bool { return true }
	fwSilent = func() bool { return silent }
	fwApply = func(_ string, ips []string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.applied = append(f.applied, ips)
		return f.applyErr
	}
	fwAdd = func(_ string, ips []string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.added = append(f.added, ips)
		return nil
	}
	fwClear = func() error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.clears++
		return nil
	}
	writeHosts = func(elevate.Payload) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.hostsOps++
		return nil
	}
	appliedStateFile = func() string { return stateFile }
	return f
}

func newTestApp(t *testing.T, regions []region.Server, res *doh.Resolver, hostsContent string) *App {
	t.Helper()
	fa := test.NewTempApp(t)
	hp := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(hp, []byte(hostsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return &App{
		fyneApp:          fa,
		win:              test.NewWindow(nil),
		log:              &applog.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		host:             &hostsfile.Hosts{Path: hp},
		resolver:         res,
		regions:          regions,
		dbdPath:          filepath.Join(t.TempDir(), "DeadByDaylight-Win64-Shipping.exe"),
		currentLabel:     widget.NewLabel(""),
		selectedPingText: newColoredText("", colGrey),
	}
}

func (a *App) state() (string, map[string]bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appliedCode, a.blocked
}

// The beacon name rotates through a pool, one address per answer, so a single
// lookup would block only part of it.
func TestBlockIPsCollectsRotatingBeaconPool(t *testing.T) {
	pool := []string{"3.83.203.126", "98.93.6.62"}
	var n atomic.Int32
	res := stubDoH(t, func(name string) []string {
		if name != "gamelift-ping.us-east-1.api.aws" {
			return []string{"10.0.0.1"}
		}
		return []string{pool[int(n.Add(1)-1)%len(pool)]}
	})
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	if got := a.blockIPs("eu-west-1"); !slices.Equal(got, pool) {
		t.Fatalf("blockIPs = %v, want the whole pool %v", got, pool)
	}
}

func TestBlockIPsKeepsEveryRecordOfAnAnswer(t *testing.T) {
	res := stubDoH(t, func(string) []string { return []string{"10.0.0.2", "10.0.0.1"} })
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	if got := a.blockIPs("eu-west-1"); !slices.Equal(got, []string{"10.0.0.1", "10.0.0.2"}) {
		t.Fatalf("blockIPs = %v", got)
	}
}

// gatedDoH answers from ips but holds every request until release is closed;
// started is closed on the first request.
func gatedDoH(t *testing.T, ips map[string]string) (res *doh.Resolver, started, release chan struct{}) {
	started, release = make(chan struct{}), make(chan struct{})
	var once sync.Once
	res = stubDoH(t, func(name string) []string {
		once.Do(func() { close(started) })
		<-release
		return []string{ips[name]}
	})
	return res, started, release
}

var twoBeacons = map[string]string{
	"gamelift-ping.eu-west-1.api.aws": "10.0.0.1",
	"gamelift-ping.us-east-1.api.aws": "10.0.0.2",
}

// A refresh that resolves while the user clicks Set Default must not put the
// block back (or write to the cleared map).
func TestRefreshDuringSetDefaultAddsNothing(t *testing.T) {
	fs := newFakeSystem(t, true)
	res, started, release := gatedDoH(t, twoBeacons)
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	a.appliedCode, a.blocked = "eu-west-1", map[string]bool{"10.0.0.9": true}

	done := make(chan struct{})
	go func() { defer close(done); a.refreshBlock() }()
	<-started
	if err := a.clearFirewall(); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done

	if len(fs.added) != 0 {
		t.Fatalf("refresh re-added rules after Set Default: %v", fs.added)
	}
	if code, _ := a.state(); code != "" {
		t.Fatalf("appliedCode = %q after Set Default", code)
	}
}

// A refresh for the old region must not block the region the user just chose.
func TestRefreshDuringSetServerKeepsChosenRegionOpen(t *testing.T) {
	fs := newFakeSystem(t, true)
	res, started, release := gatedDoH(t, twoBeacons)
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	a.appliedCode, a.blocked = "eu-west-1", map[string]bool{}

	done := make(chan struct{})
	go func() { defer close(done); a.refreshBlock() }()
	<-started
	a.fwMu.Lock() // the state a successful applyFirewall("us-east-1") leaves
	a.setApplied("us-east-1", []string{"10.0.0.1"})
	a.fwMu.Unlock()
	close(release)
	<-done

	if len(fs.added) != 0 {
		t.Fatalf("refresh added %v after the switch to us-east-1", fs.added)
	}
}

func TestRefreshAddsOnlyNewBeacons(t *testing.T) {
	fs := newFakeSystem(t, true)
	res := stubDoH(t, func(name string) []string {
		return map[string][]string{
			"gamelift-ping.us-east-1.api.aws": {"10.0.0.2"},
			"gamelift-ping.ap-east-1.api.aws": {"10.0.0.3"},
		}[name]
	})
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}, {Code: "ap-east-1"}}, res, "")
	a.appliedCode, a.blocked = "eu-west-1", map[string]bool{"10.0.0.2": true}
	a.refreshBlock()
	if len(fs.added) != 1 || !slices.Equal(fs.added[0], []string{"10.0.0.3"}) {
		t.Fatalf("added = %v, want [[10.0.0.3]]", fs.added)
	}
	if _, blocked := a.state(); !blocked["10.0.0.3"] {
		t.Fatal("new beacon not recorded")
	}
}

// Rules outlive the app; a restart must show the block, not "Automatic".
func TestRestoreAppliedShowsBlockFromLastSession(t *testing.T) {
	fs := newFakeSystem(t, false)
	if err := saveApplied("eu-west-1"); err != nil {
		t.Fatal(err)
	}
	regions := []region.Server{{Code: "eu-west-1", Pretty: "Europe (Ireland)"}}
	a := newTestApp(t, regions, stubDoH(t, func(string) []string { return nil }), "")
	a.restoreApplied()
	a.refreshCurrent()
	if got := a.currentLabel.Text; got != "You are on: Europe (Ireland) (eu-west-1)" {
		t.Fatalf("label = %q", got)
	}
	if len(fs.applied)+fs.clears != 0 {
		t.Fatal("a prompting backend must not be changed at startup")
	}
}

func TestRestoreAppliedReappliesSilently(t *testing.T) {
	fs := newFakeSystem(t, true)
	if err := saveApplied("eu-west-1"); err != nil {
		t.Fatal(err)
	}
	res := stubDoH(t, func(name string) []string { return []string{twoBeacons[name]} })
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	a.restoreApplied()
	if len(fs.applied) != 1 || !slices.Equal(fs.applied[0], []string{"10.0.0.2"}) {
		t.Fatalf("applied = %v, want [[10.0.0.2]]", fs.applied)
	}
	if code, blocked := a.state(); code != "eu-west-1" || !blocked["10.0.0.2"] {
		t.Fatalf("state = %q %v", code, blocked)
	}
}

func TestRestoreAppliedClearsUnknownRegion(t *testing.T) {
	fs := newFakeSystem(t, true)
	if err := saveApplied("xx-gone-1"); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}}, stubDoH(t, func(string) []string { return nil }), "")
	a.restoreApplied()
	if fs.clears != 1 {
		t.Fatalf("clears = %d, want 1", fs.clears)
	}
	if got := loadApplied(); got != "" {
		t.Fatalf("state %q not forgotten", got)
	}
}

func TestSaveAndLoadApplied(t *testing.T) {
	newFakeSystem(t, true)
	if got := loadApplied(); got != "" {
		t.Fatalf("empty state loaded %q", got)
	}
	if err := saveApplied("us-east-1"); err != nil {
		t.Fatal(err)
	}
	if got := loadApplied(); got != "us-east-1" {
		t.Fatalf("loaded %q", got)
	}
	if err := saveApplied(""); err != nil {
		t.Fatal(err)
	}
	if err := saveApplied(""); err != nil {
		t.Fatalf("forgetting twice: %v", err)
	}
	if got := loadApplied(); got != "" {
		t.Fatalf("forgotten state loaded %q", got)
	}
}

func TestParseBootTime(t *testing.T) {
	got, ok := parseBootTime("cpu  1 2 3\nintr 5\nbtime 1700000000\nprocesses 9\n")
	if !ok || got.Unix() != 1700000000 {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := parseBootTime("cpu 1\n"); ok {
		t.Fatal("no btime line must not parse")
	}
}

// A failed apply may already have removed the old rules; the label must not
// keep claiming the old region.
func TestFailedApplyLeavesNoStaleRegion(t *testing.T) {
	fs := newFakeSystem(t, true)
	fs.applyErr = errors.New("netsh failed")
	res := stubDoH(t, func(string) []string { return []string{"10.0.0.1"} })
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	a.appliedCode, a.blocked = "eu-west-1", map[string]bool{"10.0.0.2": true}

	a.applyFirewall("us-east-1")
	if fs.clears != 1 {
		t.Fatalf("clears = %d, want 1", fs.clears)
	}
	if code, _ := a.state(); code != "" {
		t.Fatalf("appliedCode = %q after a failed apply", code)
	}
}

// Where clearing needs a password prompt, a failed apply (usually a dismissed
// prompt) changed nothing, so the previous state stays.
func TestFailedApplyKeepsStateOnPromptingBackend(t *testing.T) {
	fs := newFakeSystem(t, false)
	fs.applyErr = errors.New("pkexec dismissed")
	res := stubDoH(t, func(string) []string { return []string{"10.0.0.1"} })
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}, res, "")
	a.appliedCode, a.blocked = "eu-west-1", map[string]bool{"10.0.0.2": true}

	a.applyFirewall("us-east-1")
	if fs.clears != 0 {
		t.Fatal("must not prompt again to clear")
	}
	if code, _ := a.state(); code != "eu-west-1" {
		t.Fatalf("appliedCode = %q", code)
	}
}

// The Ping readout must show the region the label names, not the fastest.
func TestPingReadoutFollowsAppliedRegion(t *testing.T) {
	regions := []region.Server{{Code: "ap-southeast-2", Pretty: "Asia Pacific (Sydney)"}, {Code: "eu-west-1", Pretty: "Europe (Ireland)"}}
	a := newTestApp(t, regions, stubDoH(t, func(string) []string { return nil }), "")
	a.recent = []ping.Result{{OK: true, RTTAvgMs: 300}, {OK: true, RTTAvgMs: 20}}
	a.appliedCode = "ap-southeast-2"
	a.refreshCurrent()
	if got := a.selectedPingMs(); got != 300 {
		t.Fatalf("Ping readout = %dms, want Sydney's 300ms (label %q)", got, a.currentLabel.Text)
	}
}

func TestPingReadoutAutomaticShowsFastest(t *testing.T) {
	regions := []region.Server{{Code: "ap-southeast-2"}, {Code: "eu-west-1"}}
	a := newTestApp(t, regions, stubDoH(t, func(string) []string { return nil }), "")
	a.recent = []ping.Result{{OK: true, RTTAvgMs: 300}, {OK: true, RTTAvgMs: 20}}
	a.refreshCurrent()
	if got := a.selectedPingMs(); got != 20 {
		t.Fatalf("Ping readout = %dms, want 20ms", got)
	}
}

// Redirects left by the hosts fallback must be found (and so cleared) in
// firewall mode on every OS.
func TestHostsRedirectActive(t *testing.T) {
	regions := []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}
	res := stubDoH(t, func(string) []string { return nil })
	for _, tc := range []struct {
		hosts string
		want  bool
	}{
		{"127.0.0.1 localhost\n", false},
		{"127.0.0.1 localhost\n52.1.1.1 gamelift.eu-west-1.amazonaws.com\n", true},
		{"52.1.1.2 gamelift-ping.us-east-1.api.aws\n", true},
	} {
		a := newTestApp(t, regions, res, tc.hosts)
		if got := a.hostsRedirectActive(); got != tc.want {
			t.Errorf("hosts %q: active = %v, want %v", tc.hosts, got, tc.want)
		}
	}
}

func TestClearFirewallRemovesStaleHostsRedirect(t *testing.T) {
	fs := newFakeSystem(t, false)
	a := newTestApp(t, []region.Server{{Code: "eu-west-1"}}, stubDoH(t, func(string) []string { return nil }),
		"52.1.1.1 gamelift.eu-west-1.amazonaws.com\n")
	a.appliedCode = "eu-west-1"
	if err := a.clearFirewall(); err != nil {
		t.Fatal(err)
	}
	if fs.clears != 1 || fs.hostsOps != 1 {
		t.Fatalf("clears = %d, hosts writes = %d, want 1 and 1", fs.clears, fs.hostsOps)
	}
	if code, _ := a.state(); code != "" {
		t.Fatalf("appliedCode = %q", code)
	}
}

// A DNS failure is not a permission problem and must not be reported as one.
func TestHostsFallbackReportsLookupFailure(t *testing.T) {
	fs := newFakeSystem(t, false)
	regions := []region.Server{{Code: "eu-west-1"}, {Code: "us-east-1"}}
	a := newTestApp(t, regions, stubDoH(t, func(string) []string { return nil }), "")
	a.setServerViaHosts("eu-west-1")
	if fs.hostsOps != 0 {
		t.Fatal("hosts file written despite the failed lookup")
	}
	text := overlayText(a.win.Canvas())
	if strings.Contains(text, permMessage) || !strings.Contains(text, "look up") {
		t.Fatalf("dialog text = %q", text)
	}
}

func overlayText(c fyne.Canvas) string {
	top := c.Overlays().Top()
	if top == nil {
		return ""
	}
	var b strings.Builder
	for _, o := range test.LaidOutObjects(top) {
		switch w := o.(type) {
		case *widget.Label:
			b.WriteString(w.Text + "\n")
		case *widget.RichText:
			b.WriteString(w.String() + "\n")
		}
	}
	return b.String()
}
