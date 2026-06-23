// Package ui is the Fyne front end wiring the domain packages to the window.
package ui

import (
	"errors"
	"fmt"
	"os"
	"runtime"
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

type App struct {
	fyneApp fyne.App
	win     fyne.Window
	log     *applog.Logger

	host     *hostsfile.Hosts
	resolver *doh.Resolver
	regions  []region.Server

	selectWidget     *widget.Select
	setBtn, defBtn   *widget.Button
	currentLabel     *widget.Label
	selectedPingText *canvas.Text
	pingList         *PingList

	selectedCode string
	dbdPath      string

	mu          sync.Mutex
	recent      []ping.Result
	appliedCode string          // region currently forced via the firewall
	blocked     map[string]bool // beacon IPs already blocked
}

func New(log *applog.Logger) *App {
	fa := app.New()
	fa.Settings().SetTheme(newTheme())
	w := fa.NewWindow("DBD Region Changer")
	w.SetIcon(fyne.NewStaticResource("icon.png", assets.IconPNG))
	w.Resize(fyne.NewSize(680, 540))
	w.SetFixedSize(true)

	a := &App{
		fyneApp:  fa,
		win:      w,
		log:      log,
		host:     hostsfile.New(),
		resolver: doh.Default(),
	}
	a.dbdPath = dbdpath.Find()
	a.regions = a.discoverRegions()
	if len(a.regions) > 0 {
		a.selectedCode = a.regions[0].Code
	}
	log.Info("regions loaded", "count", len(a.regions), "dbd", a.dbdPath)
	return a
}

// discoverRegions fetches ip-ranges.json once (codes for discovery, EC2 CIDRs
// for firewall blocking) and keeps the regions whose GameLift beacon answers,
// so every live region gets redirected. Falls back to the built-in list.
func (a *App) discoverRegions() []region.Server {
	codes, err := awsranges.RegionCodes(nil)
	if err != nil {
		a.log.Warn("ip-ranges fetch failed, using built-in regions", "err", err)
		return region.Builtin()
	}
	return region.Discover(codes, func(s region.Server) bool {
		ip, rerr := a.resolver.Resolve(s.Dualstack())
		if rerr != nil {
			return false
		}
		_, ok := ping.UDPBeacon(ip)
		return ok
	})
}

// applyFirewall blocks the game's traffic to the ping beacons of every region
// except code, so the client can only measure/reach the chosen region. Only the
// handful of beacon IPs are blocked (not whole regions), so matchmaking/EAC
// backends stay reachable.
func (a *App) applyFirewall(code string) {
	if !firewall.Supported() {
		return
	}
	if firewall.NeedsProgram() && a.dbdPath == "" {
		a.log.Warn("Dead by Daylight exe not found; firewall block skipped (set it via the ... menu)")
		fyne.Do(func() {
			dialog.NewInformation("Firewall block skipped",
				"Couldn't find Dead by Daylight automatically. Use the \"...\" menu -> Set DBD path to enable firewall blocking.",
				a.win).Show()
		})
		return
	}
	var blockIPs []string
	for _, r := range a.regions {
		if r.Code == code {
			continue
		}
		if ip, err := a.resolver.Resolve(r.Dualstack()); err == nil {
			blockIPs = append(blockIPs, ip)
		}
	}
	if err := firewall.Apply(a.dbdPath, blockIPs); err != nil {
		a.log.Error("firewall apply failed", "err", err)
		fyne.Do(func() {
			dialog.NewError(fmt.Errorf("firewall block failed (close DBD and try again): %w", err), a.win).Show()
		})
		return
	}
	a.mu.Lock()
	a.appliedCode = code
	a.blocked = make(map[string]bool, len(blockIPs))
	for _, ip := range blockIPs {
		a.blocked[ip] = true
	}
	a.mu.Unlock()
	a.log.Info("firewall block applied", "allow", code, "blocked_beacons", len(blockIPs))
}

// firewallRefreshLoop keeps the block current as beacon IPs rotate. It only ADDS
// newly-seen beacon IPs (never clears), so the block is never momentarily lifted
// and the active game-server connection is never disturbed.
func (a *App) firewallRefreshLoop() {
	for {
		time.Sleep(60 * time.Second)
		a.mu.Lock()
		code := a.appliedCode
		a.mu.Unlock()
		if code == "" || !firewall.RefreshSupported() {
			continue
		}
		var newIPs []string
		for _, r := range a.regions {
			if r.Code == code {
				continue
			}
			ip, err := a.resolver.ResolveFresh(r.Dualstack())
			if err != nil {
				continue
			}
			a.mu.Lock()
			seen := a.blocked[ip]
			a.mu.Unlock()
			if !seen {
				newIPs = append(newIPs, ip)
			}
		}
		if len(newIPs) == 0 {
			continue
		}
		if err := firewall.Add(a.dbdPath, newIPs); err != nil {
			a.log.Error("firewall refresh failed", "err", err)
			continue
		}
		a.mu.Lock()
		for _, ip := range newIPs {
			a.blocked[ip] = true
		}
		a.mu.Unlock()
		a.log.Info("firewall refresh added beacons", "new", len(newIPs))
	}
}

// Run shows the window and starts the ping loops once the event loop is up (so fyne.Do is safe).
func (a *App) Run() {
	a.win.SetContent(a.buildUI())
	a.win.SetCloseIntercept(a.onCloseRequested)
	a.fyneApp.Lifecycle().SetOnStarted(func() {
		go a.pingLoop()
		go a.selectedPingLoop()
		go a.firewallRefreshLoop()
	})
	a.win.ShowAndRun()
}

func (a *App) buildUI() fyne.CanvasObject {
	opts := make([]string, len(a.regions))
	for i, r := range a.regions {
		opts[i] = r.Label()
	}
	a.selectWidget = widget.NewSelect(opts, a.onSelectChanged)
	if len(opts) > 0 {
		a.selectWidget.SetSelectedIndex(0)
	}
	a.setBtn = widget.NewButton("Set Server", a.onSetServer)
	a.setBtn.Importance = widget.HighImportance
	a.defBtn = widget.NewButton("Set Default", a.onSetDefault)
	selectPanel := panel("SELECT SERVER",
		container.NewVBox(a.selectWidget, container.NewGridWithColumns(2, a.setBtn, a.defBtn)))

	a.currentLabel = widget.NewLabel("You are on: ...")
	a.selectedPingText = newColoredText("Ping:", colGrey)
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

func (a *App) onSetServer() {
	a.enableButtons(false)
	go func() {
		defer a.afterApply()
		if !firewall.Supported() {
			// No firewall backend: fall back to the hosts redirect (best effort).
			a.setServerViaHosts()
			return
		}
		// On Windows the GUI is already elevated, so clear any prior hosts
		// redirect for free. On Linux skip it to avoid a second pkexec prompt.
		if runtime.GOOS == "windows" {
			clear := elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)}
			if err := elevate.WriteHosts(clear); err != nil {
				a.log.Error("hosts clear failed", "err", err)
			}
		}
		a.applyFirewall(a.selectedCode)
		a.log.Info("set server", "code", a.selectedCode)
	}()
}

// setServerViaHosts is the fallback when no firewall backend exists: redirect
// every region's hostnames at the chosen region in the hosts file.
func (a *App) setServerViaHosts() {
	entries, err := apply.BuildEntries(a.selectedCode, a.regions, a.resolver)
	if err != nil {
		a.log.Error("build entries failed", "err", err)
		fyne.Do(a.showPermissionError)
		return
	}
	payload := elevate.Payload{
		Op:              elevate.OpWrite,
		RemoveHostnames: apply.KnownHostnames(a.regions),
		Entries:         entries,
	}
	if err := elevate.WriteHosts(payload); err != nil {
		a.log.Error("set server failed", "err", err)
		fyne.Do(a.showPermissionError)
		return
	}
	a.mu.Lock()
	a.appliedCode = a.selectedCode
	a.mu.Unlock()
	a.log.Info("set server via hosts", "code", a.selectedCode)
}

func (a *App) onSetDefault() {
	a.enableButtons(false)
	go func() {
		defer a.afterApply()
		if firewall.Supported() {
			if err := firewall.Clear(); err != nil {
				a.log.Error("firewall clear failed", "err", err)
			}
			if runtime.GOOS == "windows" {
				_ = elevate.WriteHosts(elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)})
			}
		} else if err := elevate.WriteHosts(elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)}); err != nil {
			a.log.Error("set default failed", "err", err)
			fyne.Do(a.showPermissionError)
			return
		}
		a.mu.Lock()
		a.appliedCode = ""
		a.blocked = nil
		a.mu.Unlock()
		a.log.Info("set default")
	}()
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
		a.dbdPath = rc.URI().Path()
		a.log.Info("dbd path set", "path", a.dbdPath)
		dialog.NewInformation("DBD path set", a.dbdPath, a.win).Show()
	}, a.win)
}

func (a *App) onCloseRequested() {
	dialog.NewConfirm("Close", "You sure?", func(yes bool) {
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
		a.log.Info("update check", "code", res.Code, "local", res.Local, "remote", res.Remote)
		fyne.Do(func() { a.showUpdateResult(res) })
	}()
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

func (a *App) currentOrFastest() string {
	if hosts, err := a.host.List(a.isKnownEndpoint); err == nil && len(hosts) > 0 {
		if r, ok := a.regionByEndpoint(hosts[0][1]); ok {
			return r.Code
		}
	}
	return a.fastestCode()
}

func (a *App) refreshCurrent() {
	a.mu.Lock()
	applied := a.appliedCode
	a.mu.Unlock()

	var text string
	if applied != "" {
		text = fmt.Sprintf("You are on: %s (%s)", region.PrettyName(applied), applied)
	} else {
		pretty, code := "Automatic", a.fastestCode()
		if hosts, err := a.host.List(a.isKnownEndpoint); err == nil && len(hosts) > 0 {
			if r, ok := a.regionByEndpoint(hosts[0][1]); ok {
				pretty, code = r.Pretty, r.Code
			}
		}
		text = fmt.Sprintf("You are on: %s (%s)", pretty, code)
	}
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

func (a *App) selectedPingLoop() {
	time.Sleep(1 * time.Second)
	for {
		code := a.currentOrFastest()
		ms := -1
		a.mu.Lock()
		for i := range a.regions {
			if a.regions[i].Code == code {
				if i < len(a.recent) {
					ms = a.recent[i].DisplayPing()
				}
				break
			}
		}
		a.mu.Unlock()

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
