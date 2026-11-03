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
		"1.2.3":         {1, 2, 3, Final, 0},
		"v1.0.0":        {1, 0, 0, Final, 0},
		"1.2.0-rc.4":    {1, 2, 0, RC, 4},
		"2.0.1-dev.0":   {2, 0, 1, Dev, 0},
		"1.1.0-alpha.2": {1, 1, 0, Alpha, 2},
		"1.1.0-beta.3":  {1, 1, 0, Beta, 3},
		// Release tag forms the updater accepts.
		"1.2":       {1, 2, 0, Final, 0},
		"v1.2":      {1, 2, 0, Final, 0},
		"1.1.0rc1":  {1, 1, 0, RC, 1},
		"1.1.0a1":   {1, 1, 0, Alpha, 1},
		"1.1.0.b2":  {1, 1, 0, Beta, 2},
		"1.1.0-rc3": {1, 1, 0, RC, 3},
		"1.2rc4":    {1, 2, 0, RC, 4},
		"1.2.dev0":  {1, 2, 0, Dev, 0},
		"1.2.3-rc":  {1, 2, 3, RC, 0},
		// 9 digits is the per-number cap.
		"999999999.0.1-rc.999999999": {999999999, 0, 1, RC, 999999999},
	} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1", "garbage", "1.2.3.4", "1.x.3", "1.2.3-foo.1", "1.2.0rc1x", "1.0.0-3-gabcdef", " 1.2.3", "1.2.3 ",
		// Over 9 digits: may overflow int, and the updater and build gates reject it.
		"1.2.99999999999999999999", "1234567890.0.0", "1.2.0-rc.1234567890"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

// Release tags may be written in either the Semver or the Display form; both
// must parse back to the same version, or a build stamped with that tag keeps
// the fallback version.
func TestParseRoundTripsSemverAndDisplay(t *testing.T) {
	for _, v := range []Info{
		{1, 0, 1, Final, 0},
		{1, 2, 0, Final, 0},
		{1, 2, 3, Final, 0},
		{1, 2, 0, RC, 4},
		{1, 2, 3, Alpha, 1},
		{1, 2, 3, Beta, 2},
		{1, 2, 0, Dev, 0},
		{1, 1, 2, Dev, 3},
	} {
		for _, s := range []string{v.Semver(), v.Display(), "v" + v.Display()} {
			if got, err := Parse(s); err != nil || got != v {
				t.Errorf("Parse(%q) = %+v, %v; want %+v", s, got, err, v)
			}
		}
	}
}
