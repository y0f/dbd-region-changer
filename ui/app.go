// Package ui is the Fyne front end wiring the domain packages to the window.
package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/y0f/dbd-region-changer/assets"
	"github.com/y0f/dbd-region-changer/internal/applog"
	"github.com/y0f/dbd-region-changer/internal/apply"
	"github.com/y0f/dbd-region-changer/internal/awsranges"
	"github.com/y0f/dbd-region-changer/internal/dbdpath"
	"github.com/y0f/dbd-region-changer/internal/doh"
	"github.com/y0f/dbd-region-changer/internal/elevate"
	"github.com/y0f/dbd-region-changer/internal/firewall"
	"github.com/y0f/dbd-region-changer/internal/hostsfile"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
	"github.com/y0f/dbd-region-changer/internal/updater"
)

// System changes, replaceable in tests. fwSilent reports whether the firewall
// backend changes rules without a password prompt (Windows: the app runs
// elevated).
var (
	fwSupported = firewall.Supported
	fwSilent    = firewall.RefreshSupported
	fwApply     = firewall.Apply
	fwAdd       = firewall.Add
	fwClear     = firewall.Clear
	writeHosts  = elevate.WriteHosts
)

// beaconSamples is how many uncached lookups collect a region's beacon
// addresses on top of the full A/AAAA answer: the beacon name rotates through
// a pool of IPs and each answer usually names only one of them.
const beaconSamples = 4

type App struct {
	fyneApp fyne.App
	win     fyne.Window
	log     *applog.Logger

	host     *hostsfile.Hosts
	resolver *doh.Resolver
	regions  []region.Server // set once on the UI goroutine before the loops start

	selectWidget     *widget.Select
	setBtn, defBtn   *widget.Button
	currentLabel     *widget.Label
	selectedPingText *canvas.Text
	pingList         *PingList

	selectedCode string // UI goroutine only

	// fwMu serialises firewall changes (apply, clear, refresh), so a refresh
	// can never re-add rules after Set Default or block a newly chosen region.
	// appliedCode and blocked only change while it is held.
	fwMu sync.Mutex

	mu          sync.Mutex
	dbdPath     string
	recent      []ping.Result
	appliedCode string          // region currently forced via the firewall or hosts file
	blocked     map[string]bool // beacon IPs already blocked
	shownCode   string          // region the "You are on" label shows; drives the Ping readout
}

func New(log *applog.Logger) *App {
	fa := app.New()
	fa.Settings().SetTheme(newTheme())
	w := fa.NewWindow("DBD Region Changer")
	w.SetIcon(fyne.NewStaticResource("icon.png", assets.IconPNG))
	w.Resize(fyne.NewSize(680, 540))
	w.SetFixedSize(true)

	return &App{
		fyneApp:  fa,
		win:      w,
		log:      log,
		host:     hostsfile.New(),
		resolver: doh.Default(),
	}
}

// discoverRegions returns the built-in regions plus every other AWS region
// whose GameLift beacon answers, so every live region gets blocked. If
// ip-ranges.json cannot be fetched it returns the built-in list.
func (a *App) discoverRegions() []region.Server {
	codes, err := awsranges.RegionCodes(nil)
	if err != nil {
		a.log.Warn("ip-ranges fetch failed, using built-in regions", "err", err)
		return region.Builtin()
	}
	return region.Discover(codes, a.beaconLive)
}

// beaconLive reports whether region s has a live GameLift beacon. Where the
// block applies system-wide (Linux), a lock left by the last session also
// drops this app's own probes, so a beacon that resolves counts as live:
// otherwise its region would be missing from the next block.
func (a *App) beaconLive(s region.Server) bool {
	ip, err := a.resolver.Resolve(s.Dualstack())
	if err != nil {
		return false
	}
	if fwSupported() && !firewall.NeedsProgram() && loadApplied() != "" {
		return true
	}
	_, ok := ping.UDPBeacon(ip)
	return ok
}

func (a *App) programPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dbdPath
}

// beaconIPs returns the addresses seen for one beacon hostname: every A/AAAA
// record plus a few fresh lookups, since the name rotates through a pool.
func (a *App) beaconIPs(host string) []string {
	var ips []string
	if all, err := a.resolver.ResolveAll(host); err == nil {
		ips = append(ips, all...)
	}
	for i := 0; i < beaconSamples; i++ {
		if ip, err := a.resolver.ResolveFresh(host); err == nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

// blockIPs resolves the beacon addresses of every region except code, sorted
// and deduplicated. Regions are resolved in parallel. The error names the
// regions no address was found for; the addresses of the others are still
// returned.
func (a *App) blockIPs(code string) ([]string, error) {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		seen   = map[string]bool{}
		out    []string
		failed []string
	)
	for _, r := range a.regions {
		if r.Code == code {
			continue
		}
		wg.Add(1)
		go func(r region.Server) {
			defer wg.Done()
			ips := a.beaconIPs(r.Dualstack())
			mu.Lock()
			defer mu.Unlock()
			if len(ips) == 0 {
				failed = append(failed, r.Code)
			}
			for _, ip := range ips {
				if !seen[ip] {
					seen[ip] = true
					out = append(out, ip)
				}
			}
		}(r)
	}
	wg.Wait()
	sort.Strings(out)
	if len(failed) > 0 {
		sort.Strings(failed)
		return out, fmt.Errorf("no beacon address found for %s", strings.Join(failed, ", "))
	}
	return out, nil
}

// setApplied records the firewall state and remembers it across restarts.
// The caller holds fwMu.
func (a *App) setApplied(code string, ips []string) {
	a.mu.Lock()
	a.appliedCode = code
	a.blocked = make(map[string]bool, len(ips))
	for _, ip := range ips {
		a.blocked[ip] = true
	}
	a.mu.Unlock()
	if err := saveApplied(code); err != nil {
		a.log.Warn("saving applied region failed", "err", err)
	}
}

// applyFirewall blocks the game's traffic to the ping beacons of every region
// except code, so the client can only measure/reach the chosen region. Only the
// beacon IPs are blocked (not whole regions), so matchmaking/EAC backends stay
// reachable.
func (a *App) applyFirewall(code string) {
	if !fwSupported() {
		return
	}
	a.fwMu.Lock()
	defer a.fwMu.Unlock()
	program := a.programPath()
	if firewall.NeedsProgram() && program == "" {
		a.log.Warn("Dead by Daylight exe not found; firewall block skipped (set it via the ... menu)")
		fyne.Do(func() {
			dialog.NewInformation("Firewall block skipped",
				"Couldn't find Dead by Daylight automatically. Use the \"...\" menu -> Set DBD path to enable firewall blocking.",
				a.win).Show()
		})
		return
	}
	ips, err := a.blockIPs(code)
	if err != nil {
		// A block without these regions would let the game reach them while
		// the label claims the lock, so the firewall is left as it is.
		a.log.Error("beacon lookup failed; firewall unchanged", "err", err)
		fyne.Do(func() {
			dialog.NewError(fmt.Errorf("could not look up the region servers, the firewall was not changed (check your connection and try again): %w", err), a.win).Show()
		})
		return
	}
	a.replaceBlock(program, code, ips)
}

// replaceBlock swaps the firewall block for one of ips that leaves code open.
// The caller holds fwMu.
func (a *App) replaceBlock(program, code string, ips []string) {
	if err := fwApply(program, ips); err != nil {
		a.log.Error("firewall apply failed", "err", err)
		msg := "firewall block failed (close DBD and try again)"
		switch {
		case fwSilent():
			// A failed apply can leave the old block, a partial new one, or
			// none. Clear it, so the rules and the label agree.
			if cerr := fwClear(); cerr != nil {
				a.log.Error("firewall clear failed", "err", cerr)
			}
			a.setApplied("", nil)
			msg = "firewall block failed and the region lock was removed (close DBD and try again)"
		case !errors.Is(err, firewall.ErrNotRun):
			// The authorized script failed and removed its partial block.
			a.setApplied("", nil)
			msg = "firewall block failed and the region lock was removed (try again)"
		}
		// Otherwise the change never ran (pkexec missing or the password
		// prompt dismissed): nothing changed, so the previous state is kept.
		fyne.Do(func() { dialog.NewError(fmt.Errorf("%s: %w", msg, err), a.win).Show() })
		return
	}
	a.setApplied(code, ips)
	a.log.Info("firewall block applied", "allow", code, "blocked_beacons", len(ips))
}

// showBlock records code as locked by rules that are already in place, with
// none of their addresses known yet. The caller holds fwMu.
func (a *App) showBlock(code string) {
	a.mu.Lock()
	a.appliedCode = code
	a.blocked = map[string]bool{}
	a.mu.Unlock()
}

// restoreApplied picks up a block left by an earlier session. Firewall rules
// outlive the app, so without this the label would read Automatic while the
// game stays locked, and the block would never be refreshed.
func (a *App) restoreApplied() {
	code := loadApplied()
	if code == "" || !fwSupported() {
		return
	}
	known := false
	for _, r := range a.regions {
		known = known || r.Code == code
	}
	switch {
	case fwSilent() && known && (!firewall.NeedsProgram() || a.programPath() != ""):
		// Re-resolve the beacons, which have rotated since, and replace the block.
		a.log.Info("restoring region lock from last session", "code", code)
		a.fwMu.Lock()
		defer a.fwMu.Unlock()
		ips, err := a.blockIPs(code)
		if err != nil {
			// Offline at startup: keep the old rules; the refresh loop adds the
			// current beacons once lookups work again.
			a.log.Warn("beacon lookup failed; keeping region lock from last session", "code", code, "err", err)
			a.showBlock(code)
			return
		}
		a.replaceBlock(a.programPath(), code, ips)
	case fwSilent():
		// The block cannot be rebuilt (region gone or DBD path unknown): remove it.
		a.log.Warn("removing region lock from last session", "code", code)
		a.fwMu.Lock()
		if err := fwClear(); err != nil {
			a.log.Error("firewall clear failed", "err", err)
		}
		a.setApplied("", nil)
		a.fwMu.Unlock()
	default:
		// Changing rules needs a password prompt here, so only show the block.
		a.log.Info("region lock from last session still active", "code", code)
		a.fwMu.Lock()
		a.showBlock(code)
		a.fwMu.Unlock()
	}
}

// firewallRefreshLoop keeps the block current as beacon IPs rotate, where that
// needs no password prompt.
func (a *App) firewallRefreshLoop() {
	for {
		time.Sleep(60 * time.Second)
		if fwSilent() {
			a.refreshBlock()
		}
	}
}

// refreshBlock blocks beacon IPs that rotated in since the last apply. It only
// ADDS (never clears), so the block is never momentarily lifted and the active
// game-server connection is never disturbed. Resolving runs without locks; if
// Set Server or Set Default changed the block meanwhile, the result is dropped.
func (a *App) refreshBlock() {
	a.mu.Lock()
	code := a.appliedCode
	a.mu.Unlock()
	if code == "" {
		return
	}
	ips, _ := a.blockIPs(code) // adding only: regions that failed to resolve keep their rules

	a.fwMu.Lock()
	defer a.fwMu.Unlock()
	a.mu.Lock()
	if a.appliedCode != code {
		a.mu.Unlock()
		return
	}
	var newIPs []string
	for _, ip := range ips {
		if !a.blocked[ip] {
			newIPs = append(newIPs, ip)
		}
	}
	program := a.dbdPath
	a.mu.Unlock()
	if len(newIPs) == 0 {
		return
	}
	if err := fwAdd(program, newIPs); err != nil {
		a.log.Error("firewall refresh failed", "err", err)
		return
	}
	a.mu.Lock()
	if a.blocked == nil {
		a.blocked = make(map[string]bool, len(newIPs))
	}
	for _, ip := range newIPs {
		a.blocked[ip] = true
	}
	a.mu.Unlock()
	a.log.Info("firewall refresh added beacons", "new", len(newIPs))
}

// Run shows the window at once and loads the regions in the background.
func (a *App) Run() {
	a.win.SetContent(a.buildUI())
	a.win.SetCloseIntercept(a.onCloseRequested)
	a.fyneApp.Lifecycle().SetOnStarted(func() { go a.start() })
	a.win.ShowAndRun()
}

// start finds DBD and the live regions (network bound, seconds), restores a
// block from the last session, then enables the controls and starts the loops.
func (a *App) start() {
	found := dbdpath.Find()
	regions := a.discoverRegions()
	path := a.useFoundPath(found)
	a.log.Info("regions loaded", "count", len(regions), "dbd", path)

	fyne.DoAndWait(func() { a.setRegions(regions) })
	a.restoreApplied()
	a.refreshCurrent()
	if len(regions) > 0 {
		a.enableButtons(true)
	}
	go a.pingLoop()
	go a.selectedPingLoop()
	go a.firewallRefreshLoop()
}

// useFoundPath adopts the autodetected DBD path unless the user set one while
// the regions were loading, and returns the path in effect.
func (a *App) useFoundPath(found string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dbdPath == "" {
		a.dbdPath = found
	}
	return a.dbdPath
}

// setRegions fills the region list. It runs on the UI goroutine.
func (a *App) setRegions(regions []region.Server) {
	a.regions = regions
	opts := make([]string, len(regions))
	for i, r := range regions {
		opts[i] = r.Label()
	}
	a.selectWidget.PlaceHolder = "(Select one)"
	a.selectWidget.SetOptions(opts)
	if len(opts) > 0 {
		a.selectWidget.SetSelectedIndex(0)
		a.selectWidget.Enable()
	}
}

func (a *App) buildUI() fyne.CanvasObject {
	a.selectWidget = widget.NewSelect(nil, a.onSelectChanged)
	a.selectWidget.PlaceHolder = "Loading regions..."
	a.selectWidget.Disable()
	a.setBtn = widget.NewButton("Set Server", a.onSetServer)
	a.setBtn.Importance = widget.HighImportance
	a.setBtn.Disable()
	a.defBtn = widget.NewButton("Set Default", a.onSetDefault)
	a.defBtn.Disable()
	selectPanel := panel("SELECT SERVER",
		container.NewVBox(a.selectWidget, container.NewGridWithColumns(2, a.setBtn, a.defBtn)))

	a.currentLabel = widget.NewLabel("You are on: ...")
	a.selectedPingText = newColoredText("Ping: —", colGrey)
	currentPanel := panel("",
		container.NewBorder(nil, nil, a.currentLabel, a.selectedPingText))

	a.pingList = NewPingList()
	scroll := container.NewVScroll(a.pingList.Object())
	scroll.SetMinSize(fyne.NewSize(600, 200))
	pingPanel := panel("REGION PING", scroll)

	content := container.NewVBox(a.buildTitle(), selectPanel, currentPanel, pingPanel)
	backdrop := canvas.NewVerticalGradient(colBgTop, colBgBottom)
	return container.NewStack(backdrop, container.NewPadded(content))
}

func (a *App) onSelectChanged(label string) {
	for _, r := range a.regions {
		if r.Label() == label {
			a.selectedCode = r.Code
			a.log.Info("selected region", "code", r.Code)
			return
		}
	}
}

func (a *App) enableButtons(enabled bool) {
	fyne.Do(func() {
		if enabled {
			a.setBtn.Enable()
			a.defBtn.Enable()
			return
		}
		a.setBtn.Disable()
		a.defBtn.Disable()
	})
}

// hostsRedirectActive reports whether the hosts file holds a redirect for any
// region, as written by the hosts fallback.
func (a *App) hostsRedirectActive() bool {
	known := make(map[string]bool, 2*len(a.regions))
	for _, hn := range apply.KnownHostnames(a.regions) {
		known[hn] = true
	}
	hosts, err := a.host.List(func(hn string) bool { return known[hn] })
	return err == nil && len(hosts) > 0
}

// clearHostsInFirewallMode removes hosts redirects left by the hosts fallback.
// On Windows the GUI is already elevated, so it always runs; elsewhere only
// when a redirect exists, to avoid a needless password prompt.
func (a *App) clearHostsInFirewallMode() {
	if runtime.GOOS != "windows" && !a.hostsRedirectActive() {
		return
	}
	clear := elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)}
	if err := writeHosts(clear); err != nil {
		a.log.Error("hosts clear failed", "err", err)
	}
}

func (a *App) onSetServer() {
	code := a.selectedCode // read on the UI goroutine, which owns it
	a.enableButtons(false)
	go func() {
		defer a.afterApply()
		if !fwSupported() {
			// No firewall backend: fall back to the hosts redirect (best effort).
			a.setServerViaHosts(code)
			return
		}
		a.clearHostsInFirewallMode()
		a.applyFirewall(code)
		a.log.Info("set server", "code", code)
	}()
}

// setServerViaHosts is the fallback when no firewall backend exists: redirect
// every region's hostnames at the chosen region in the hosts file.
func (a *App) setServerViaHosts(code string) {
	entries, err := apply.BuildEntries(code, a.regions, a.resolver)
	if err != nil {
		a.log.Error("build entries failed", "err", err)
		fyne.Do(func() {
			dialog.NewError(fmt.Errorf("could not look up the region's servers (check your connection and try again): %w", err), a.win).Show()
		})
		return
	}
	payload := elevate.Payload{
		Op:              elevate.OpWrite,
		RemoveHostnames: apply.KnownHostnames(a.regions),
		Entries:         entries,
	}
	if err := writeHosts(payload); err != nil {
		a.log.Error("set server failed", "err", err)
		fyne.Do(a.showPermissionError)
		return
	}
	a.fwMu.Lock()
	a.mu.Lock()
	a.appliedCode = code
	a.mu.Unlock()
	a.fwMu.Unlock()
	a.log.Info("set server via hosts", "code", code)
}

func (a *App) onSetDefault() {
	a.enableButtons(false)
	go func() {
		defer a.afterApply()
		if fwSupported() {
			if err := a.clearFirewall(); err != nil {
				a.log.Error("firewall clear failed", "err", err)
				fyne.Do(func() {
					dialog.NewError(fmt.Errorf("removing the firewall block failed: %w", err), a.win).Show()
				})
				return
			}
		} else {
			if err := writeHosts(elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)}); err != nil {
				a.log.Error("set default failed", "err", err)
				fyne.Do(a.showPermissionError)
				return
			}
			a.fwMu.Lock()
			a.mu.Lock()
			a.appliedCode = ""
			a.blocked = nil
			a.mu.Unlock()
			a.fwMu.Unlock()
		}
		a.log.Info("set default")
	}()
}

// clearFirewall removes the block and any hosts redirect. If removing the
// block fails (a dismissed password prompt), the state is left as it was.
func (a *App) clearFirewall() error {
	a.fwMu.Lock()
	defer a.fwMu.Unlock()
	if err := fwClear(); err != nil {
		return err
	}
	a.clearHostsInFirewallMode()
	a.setApplied("", nil)
	return nil
}

func (a *App) afterApply() {
	a.refreshCurrent()
	time.Sleep(1 * time.Second)
	a.enableButtons(true)
}

func (a *App) onSetDBDPath() {
	dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		defer rc.Close()
		path := rc.URI().Path()
		if serr := a.setDBDPath(path); serr != nil {
			a.log.Warn("dbd path rejected", "path", path, "err", serr)
			dialog.NewError(serr, a.win).Show()
			return
		}
		a.log.Info("dbd path set", "path", path)
		dialog.NewInformation("DBD path set", path, a.win).Show()
	}, a.win)
}

// setDBDPath uses and remembers path if it names a DBD shipping binary.
// Firewall rules scoped to any other exe (such as the DeadByDaylight.exe
// launcher stub) would miss the game's traffic.
func (a *App) setDBDPath(path string) error {
	if !dbdpath.IsGameExe(path) {
		return fmt.Errorf("%s is not the Dead by Daylight game binary: pick DeadByDaylight-Win64-Shipping.exe (Steam), DeadByDaylight-EGS-Shipping.exe (Epic) or DeadByDaylight-WinGDK-Shipping.exe (Xbox / Game Pass)", filepath.Base(path))
	}
	a.mu.Lock()
	a.dbdPath = path
	a.mu.Unlock()
	if err := dbdpath.Save(path); err != nil {
		a.log.Warn("saving dbd path failed", "err", err)
	}
	return nil
}

func (a *App) onCloseRequested() {
	msg := "You sure?"
	a.mu.Lock()
	locked := a.appliedCode != ""
	a.mu.Unlock()
	if locked && fwSupported() {
		msg = "The region lock stays active after closing,\nuntil you click Set Default. Close anyway?"
	}
	dialog.NewConfirm("Close", msg, func(yes bool) {
		if !yes {
			return
		}
		a.log.Info("closing")
		a.log.Cleanup()
		// Guarantee the process actually dies (and the single-instance mutex is
		// released) instead of lingering on background goroutines.
		os.Exit(0)
	}, a.win).Show()
}

func (a *App) onCheckUpdates() {
	go func() {
		res := updater.Check(nil)
		fyne.Do(func() { a.reportUpdate(res) })
	}()
}

// reportUpdate logs the update check result, with the reason if it failed,
// and shows it.
func (a *App) reportUpdate(res updater.Result) {
	a.log.Info("update check", "code", res.Code, "local", res.Local, "remote", res.Remote, "err", res.Err)
	a.showUpdateResult(res)
}

func (a *App) onDumpLogs() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		if derr := a.log.DumpTo(uri.Path()); derr != nil {
			if errors.Is(derr, applog.ErrSameDir) {
				dialog.NewInformation("Warning",
					"You cannot use the same directory as the source, task aborted", a.win).Show()
				return
			}
			a.log.Error("dump logs failed", "err", derr)
			dialog.NewError(derr, a.win).Show()
		}
	}, a.win)
}

func (a *App) isKnownEndpoint(hn string) bool {
	for _, r := range a.regions {
		if r.Endpoint() == hn {
			return true
		}
	}
	return false
}

func (a *App) regionByEndpoint(hn string) (region.Server, bool) {
	for _, r := range a.regions {
		if r.Endpoint() == hn {
			return r, true
		}
	}
	return region.Server{}, false
}

func (a *App) fastestCode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return pickFastest(a.regions, a.recent)
}

// currentRegion returns the region the game is held to: the one applied via
// the firewall, else a hosts redirect, else the fastest (what matchmaking picks).
func (a *App) currentRegion() (pretty, code string) {
	a.mu.Lock()
	applied := a.appliedCode
	a.mu.Unlock()
	if applied != "" {
		return region.PrettyName(applied), applied
	}
	if hosts, err := a.host.List(a.isKnownEndpoint); err == nil && len(hosts) > 0 {
		if r, ok := a.regionByEndpoint(hosts[0][1]); ok {
			return r.Pretty, r.Code
		}
	}
	return "Automatic", a.fastestCode()
}

// refreshCurrent updates the "You are on" label, and the region whose ping the
// Ping readout shows, so both always name the same region.
func (a *App) refreshCurrent() {
	pretty, code := a.currentRegion()
	a.mu.Lock()
	a.shownCode = code
	a.mu.Unlock()
	text := fmt.Sprintf("You are on: %s (%s)", pretty, code)
	fyne.Do(func() { a.currentLabel.SetText(text) })
}

func (a *App) pingLoop() {
	for {
		ips := make([]string, len(a.regions))
		for i, r := range a.regions {
			if ip, err := a.resolver.Resolve(r.Endpoint()); err == nil {
				ips[i] = ip
			}
		}
		results := ping.PingAll(ips)

		a.mu.Lock()
		a.recent = results
		a.mu.Unlock()

		rows := make([]PingRow, len(results))
		for i, res := range results {
			ms := res.DisplayPing()
			rows[i] = PingRow{
				Text:  fmt.Sprintf("●  %-16s %4d ms   %5.1f%% loss", a.regions[i].Code, ms, res.DisplayLossPct()),
				Color: pingColor(ms),
			}
		}
		fyne.Do(func() { a.pingList.SetRows(rows) })
		a.refreshCurrent()
		time.Sleep(1 * time.Second)
	}
}

// selectedPingMs returns the latest ping of the region the label shows, or -1.
func (a *App) selectedPingMs() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.regions {
		if a.regions[i].Code == a.shownCode {
			if i < len(a.recent) {
				return a.recent[i].DisplayPing()
			}
			break
		}
	}
	return -1
}

func (a *App) selectedPingLoop() {
	time.Sleep(1 * time.Second)
	for {
		ms := a.selectedPingMs()
		text := fmt.Sprintf("Ping: %dms", ms)
		col := pingColor(ms)
		fyne.Do(func() {
			a.selectedPingText.Text = text
			a.selectedPingText.Color = col
			a.selectedPingText.Refresh()
		})
		time.Sleep(1 * time.Second)
	}
}

// FatalDialog shows a standalone blocking message window, used before the main window exists.
func FatalDialog(title, msg string) {
	fa := app.New()
	fa.Settings().SetTheme(newTheme())
	w := fa.NewWindow(title)
	w.Resize(fyne.NewSize(380, 140))
	backdrop := canvas.NewVerticalGradient(colBgTop, colBgBottom)
	w.SetContent(container.NewStack(backdrop, container.NewCenter(widget.NewLabel(msg))))
	w.SetCloseIntercept(func() { fa.Quit() })
	w.Show()
	fa.Run()
}
