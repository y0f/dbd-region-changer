// Package ui is the Fyne front end wiring the domain packages to the window.
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/y0f/dbd-region-changer/assets"
	"github.com/y0f/dbd-region-changer/internal/applog"
	"github.com/y0f/dbd-region-changer/internal/apply"
	"github.com/y0f/dbd-region-changer/internal/dbdpath"
	"github.com/y0f/dbd-region-changer/internal/doh"
	"github.com/y0f/dbd-region-changer/internal/elevate"
	"github.com/y0f/dbd-region-changer/internal/hostsfile"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
	"github.com/y0f/dbd-region-changer/internal/updater"
)

type App struct {
	fyneApp fyne.App
	win     fyne.Window
	log     *applog.Logger
	ctx     context.Context
	cancel  context.CancelFunc

	host     *hostsfile.Hosts
	resolver *doh.Resolver
	regions  []region.Server // set once on the UI goroutine before the loops start

	selectWidget     *widget.Select
	setBtn, defBtn   *widget.Button
	currentLabel     *widget.Label
	selectedPingText *canvas.Text
	pingList         *PingList

	selectedCode string // UI goroutine only
	busy         bool   // UI goroutine only; one user operation at a time

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
	w.Resize(fyne.NewSize(620, 640))
	w.SetFixedSize(true)
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		fyneApp:  fa,
		win:      w,
		log:      log,
		ctx:      ctx,
		cancel:   cancel,
		host:     hostsfile.New(),
		resolver: doh.NewContext(ctx, doh.DefaultProviders()),
	}
}

// Run shows the window at once and loads the regions in the background.
func (a *App) Run() {
	defer a.cancel()
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
	if a.ctx.Err() != nil {
		return
	}
	path := a.useFoundPath(found)
	a.log.Info("regions loaded", "count", len(regions), "dbd", path)

	fyne.DoAndWait(func() { a.setRegions(regions) })
	a.restoreApplied()
	a.refreshCurrent()
	if len(regions) > 0 {
		a.enableButtons(true)
	}
	go a.pingLoop()
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

	a.currentLabel = widget.NewLabel("Loading regions…")
	a.selectedPingText = newMonoText("—", colMuted)
	currentPanel := panel("",
		container.NewBorder(nil, nil, a.currentLabel, a.selectedPingText))

	a.pingList = NewPingList()
	rows := container.New(layout.NewCustomPaddedLayout(4, 4, 0, 16), a.pingList.Object())
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(540, 240))
	pingPanel := panel("REGION LATENCY", scroll)

	header := container.NewVBox(a.buildTitle(), newColoredText("Close Dead by Daylight before changing region.", colMuted), selectPanel, currentPanel)
	content := container.NewBorder(header, nil, nil, nil, pingPanel)
	backdrop := canvas.NewRectangle(colBg)
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
		a.busy = !enabled
		if enabled {
			a.setBtn.Enable()
			a.defBtn.Enable()
			a.selectWidget.Enable()
			return
		}
		a.setBtn.Disable()
		a.defBtn.Disable()
		a.selectWidget.Disable()
	})
}

// beginApply runs on the UI goroutine, before launching any worker.
func (a *App) beginApply() bool {
	if a.busy {
		return false
	}
	a.busy = true
	a.setBtn.Disable()
	a.defBtn.Disable()
	a.selectWidget.Disable()
	return true
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
func (a *App) clearHostsInFirewallMode() error {
	if runtime.GOOS != "windows" && !a.hostsRedirectActive() {
		return nil
	}
	clear := elevate.Payload{Op: elevate.OpClear, RemoveHostnames: apply.KnownHostnames(a.regions)}
	return writeHosts(clear)
}

func (a *App) onSetServer() {
	code := a.selectedCode // read on the UI goroutine, which owns it
	if code == "" || !a.beginApply() {
		return
	}
	go func() {
		defer a.afterApply()
		if !fwSupported() {
			// No firewall backend: fall back to the hosts redirect (best effort).
			a.setServerViaHosts(code)
			return
		}
		if err := a.clearHostsInFirewallMode(); err != nil {
			a.showOperationError("Could not remove the previous hosts redirect", err)
			return
		}
		a.applyFirewall(code)
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
	if !a.beginApply() {
		return
	}
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
		a.setApplied(unknownRegion, nil)
		return err
	}
	if err := a.clearHostsInFirewallMode(); err != nil {
		a.setApplied(unknownRegion, nil)
		return err
	}
	a.setApplied("", nil)
	return nil
}

func (a *App) afterApply() {
	a.refreshCurrent()
	a.enableButtons(true)
}

func (a *App) showOperationError(message string, err error) {
	a.log.Error(message, "err", err)
	fyne.Do(func() { dialog.NewError(fmt.Errorf("%s: %w", message, err), a.win).Show() })
}

func (a *App) onSetDBDPath() {
	if a.busy {
		return
	}
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
	if !a.fwMu.TryLock() {
		return errors.New("wait for the current firewall operation to finish")
	}
	defer a.fwMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.appliedCode != "" {
		return errors.New("use Set Default before changing the game path")
	}
	if err := dbdpath.Save(path); err != nil {
		return fmt.Errorf("save game path: %w", err)
	}
	a.dbdPath = path
	return nil
}

func (a *App) onCloseRequested() {
	if a.busy {
		dialog.NewInformation("Change in progress", "Wait for the region change to finish before closing.", a.win).Show()
		return
	}
	a.mu.Lock()
	locked := a.appliedCode != ""
	a.mu.Unlock()
	if !locked {
		a.close()
		return
	}
	dialog.NewConfirm("Close", "The region lock stays active after closing.\nUse Set Default to remove it. Close anyway?", func(yes bool) {
		if !yes {
			return
		}
		a.close()
	}, a.win).Show()
}

func (a *App) close() {
	if !a.fwMu.TryLock() {
		dialog.NewInformation("Change in progress", "Wait for the firewall update to finish before closing.", a.win).Show()
		return
	}
	defer a.fwMu.Unlock()
	a.log.Info("closing")
	a.cancel()
	a.fyneApp.Quit()
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

// FatalDialog shows a standalone blocking message window, used before the main window exists.
func FatalDialog(title, msg string) {
	fa := app.New()
	fa.Settings().SetTheme(newTheme())
	w := fa.NewWindow(title)
	w.Resize(fyne.NewSize(380, 140))
	backdrop := canvas.NewRectangle(colBg)
	w.SetContent(container.NewStack(backdrop, container.NewCenter(widget.NewLabel(msg))))
	w.SetCloseIntercept(func() { fa.Quit() })
	w.Show()
	fa.Run()
}
