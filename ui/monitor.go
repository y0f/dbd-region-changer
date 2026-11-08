package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

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

// currentRegion returns the configured lock, or the fastest measured endpoint
// in automatic mode. The app cannot inspect the game's actual matchmaking.
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

// refreshCurrent updates the lock status and latency together.
func (a *App) refreshCurrent() {
	pretty, code := a.currentRegion()
	a.mu.Lock()
	a.shownCode = code
	a.mu.Unlock()
	text := "Locked to " + pretty
	ms := a.selectedPingMs()
	latency := latencyText(ms)
	if pretty == "Automatic" {
		text = "Automatic matchmaking"
		if ms >= 0 {
			latency = "Fastest " + latency
		}
	} else if code == unknownRegion {
		text = "Lock status unknown · use Set Default"
	}
	fyne.Do(func() {
		a.currentLabel.SetText(text)
		a.selectedPingText.Text = latency
		a.selectedPingText.Color = pingColor(ms)
		a.selectedPingText.Refresh()
	})
}

func (a *App) pingLoop() {
	for {
		ips := a.pingAddresses()
		results := ping.PingAll(ips)
		if a.ctx.Err() != nil {
			return
		}

		a.mu.Lock()
		a.recent = results
		a.mu.Unlock()

		rows := pingRows(a.regions, results)
		fyne.Do(func() { a.pingList.SetRows(rows) })
		a.refreshCurrent()
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// Resolve in parallel so a slow DNS request cannot delay every later region.
func (a *App) pingAddresses() []string {
	ips := make([]string, len(a.regions))
	var wg sync.WaitGroup
	for i, r := range a.regions {
		wg.Go(func() {
			ips[i], _ = a.resolver.Resolve(r.Endpoint())
		})
	}
	wg.Wait()
	return ips
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
