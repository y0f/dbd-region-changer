package ui

import (
	"testing"

	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

func TestPickFastestIgnoresFailedRegions(t *testing.T) {
	regions := []region.Server{{Code: "ap-east-1"}, {Code: "eu-central-1"}, {Code: "eu-west-1"}}
	recent := []ping.Result{
		{OK: false, RTTAvgMs: -1}, // hong kong failed
		{OK: true, RTTAvgMs: 15},  // frankfurt
		{OK: true, RTTAvgMs: 22},  // dublin
	}
	if got := pickFastest(regions, recent); got != "eu-central-1" {
		t.Fatalf("got %q want eu-central-1", got)
	}
}

func TestPickFastestPicksLowestLatency(t *testing.T) {
	regions := []region.Server{{Code: "us-east-1"}, {Code: "eu-west-1"}}
	recent := []ping.Result{
		{OK: true, RTTAvgMs: 90},
		{OK: true, RTTAvgMs: 18},
	}
	if got := pickFastest(regions, recent); got != "eu-west-1" {
		t.Fatalf("got %q want eu-west-1", got)
	}
}

func TestPickFastestAllFailed(t *testing.T) {
	regions := []region.Server{{Code: "ap-east-1"}, {Code: "eu-central-1"}}
	recent := []ping.Result{{OK: false, RTTAvgMs: -1}, {OK: false, RTTAvgMs: -1}}
	if got := pickFastest(regions, recent); got != "None" {
		t.Fatalf("got %q want None", got)
	}
}

func TestPickFastestEmpty(t *testing.T) {
	if got := pickFastest(nil, nil); got != "None" {
		t.Fatalf("got %q want None", got)
	}
}
