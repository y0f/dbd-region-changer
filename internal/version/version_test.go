package version

import (
	"strings"
	"testing"
)

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

// Current changes every release; it must stay a valid version that survives a
// round trip, since the update check compares it against the latest tag.
func TestCurrentRoundTrips(t *testing.T) {
	got, err := Parse(Current.Semver())
	if err != nil {
		t.Fatalf("Parse(Current.Semver()=%q): %v", Current.Semver(), err)
	}
	if got != Current {
		t.Fatalf("round trip = %+v, want %+v", got, Current)
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]Info{
		"1.2.3":       {1, 2, 3, Final, 0},
		"v1.0.0":      {1, 0, 0, Final, 0},
		"1.2.0-rc.4":  {1, 2, 0, RC, 4},
		"2.0.1-dev.0": {2, 0, 1, Dev, 0},
	} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
		if err == nil && got.Semver() != strings.TrimPrefix(in, "v") {
			t.Errorf("Parse(%q).Semver() = %q", in, got.Semver())
		}
	}
	for _, bad := range []string{"", "1.2", "1.2.3.4", "1.x.3", "1.2.3-foo.1", "1.2.3-rc", "1.0.0-3-gabcdef"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}
