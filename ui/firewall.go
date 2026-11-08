package ui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/y0f/dbd-region-changer/internal/awsranges"
	"github.com/y0f/dbd-region-changer/internal/elevate"
	"github.com/y0f/dbd-region-changer/internal/firewall"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
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

// Sample both address families repeatedly because beacons rotate their answers.
const beaconSamples = 4

const unknownRegion = "unknown"

// discoverRegions returns the built-in regions plus every other AWS region
// whose GameLift beacon answers, so every live region gets blocked. If
// ip-ranges.json cannot be fetched it returns the built-in list.
func (a *App) discoverRegions() []region.Server {
	codes, err := awsranges.RegionCodesContext(a.ctx, nil)
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

// beaconIPs collects fresh A/AAAA answers from one beacon's rotating pool.
func (a *App) beaconIPs(host string) []string {
	var ips []string
	for i := 0; i < beaconSamples; i++ {
		all, err := a.resolver.ResolveAllFresh(host)
		if err != nil {
			break
		}
		ips = append(ips, all...)
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
		case errors.Is(err, firewall.ErrNotRun):
			// Authorization or staging failed before the old rules changed.
		case errors.Is(err, firewall.ErrStateUnknown):
			a.setApplied(unknownRegion, nil)
			msg = "firewall cleanup failed; some rules may remain. Use Set Default to retry"
		case fwSilent():
			// A failed apply can leave the old block, a partial new one, or
			// none. Clear it, so the rules and the label agree.
			if cerr := fwClear(); cerr != nil {
				a.log.Error("firewall clear failed", "err", cerr)
				a.setApplied(unknownRegion, nil)
				msg = "firewall cleanup failed; some rules may remain. Use Set Default to retry"
				break
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
			a.setApplied(unknownRegion, nil)
		} else {
			a.setApplied("", nil)
		}
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
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
		}
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
	if code == "" || code == unknownRegion {
		return
	}
	ips, _ := a.blockIPs(code) // adding only: regions that failed to resolve keep their rules

	a.fwMu.Lock()
	defer a.fwMu.Unlock()
	if a.ctx != nil && a.ctx.Err() != nil {
		return
	}
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
