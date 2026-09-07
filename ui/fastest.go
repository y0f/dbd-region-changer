package ui

import (
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

// pickFastest returns the lowest-latency responding region; failed pings are skipped. Returns "None" if nothing responded.
func pickFastest(regions []region.Server, recent []ping.Result) string {
	best := -1
	for i := range recent {
		if i >= len(regions) {
			break
		}
		if !recent[i].OK {
			continue
		}
		if best == -1 || recent[i].RTTAvgMs < recent[best].RTTAvgMs {
			best = i
		}
	}
	if best == -1 {
		return "None"
	}
	return regions[best].Code
}
