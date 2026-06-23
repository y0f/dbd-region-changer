package version

import "testing"

func TestDisplayAndSemver(t *testing.T) {
	// (1,2,0,final,0): patch==0 -> 2 parts -> "1.2"; semver always "1.2.0".
	v := Info{1, 2, 0, Final, 0}
	if got := v.Display(); got != "1.2" {
		t.Fatalf("Display=%q want 1.2", got)
	}
	if got := v.Semver(); got != "1.2.0" {
		t.Fatalf("Semver=%q want 1.2.0", got)
	}

	pre := Info{1, 2, 0, RC, 4}
	if got := pre.Display(); got != "1.2rc4" {
		t.Fatalf("Display=%q want 1.2rc4", got)
	}
	if got := pre.Semver(); got != "1.2.0-rc.4" {
		t.Fatalf("Semver=%q want 1.2.0-rc.4", got)
	}

	// patch != 0 -> 3 parts.
	p := Info{1, 2, 3, Final, 0}
	if got := p.Display(); got != "1.2.3" {
		t.Fatalf("Display=%q want 1.2.3", got)
	}

	dev := Info{1, 1, 2, Dev, 0}
	if got := dev.Display(); got != "1.1.2.dev0" {
		t.Fatalf("Display=%q want 1.1.2.dev0", got)
	}
	if got := dev.Semver(); got != "1.1.2-dev.0" {
		t.Fatalf("Semver=%q want 1.1.2-dev.0", got)
	}
}

func TestCurrent(t *testing.T) {
	if Current.Display() != "1.0" {
		t.Fatalf("Current.Display=%q", Current.Display())
	}
	if Current.Semver() != "1.0.0" {
		t.Fatalf("Current.Semver=%q", Current.Semver())
	}
}
