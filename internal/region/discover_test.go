package region

import (
	"sync/atomic"
	"testing"
)

func TestDiscoverKeepsBuiltinAndAnsweringExtras(t *testing.T) {
	codes := []string{"us-east-1", "eu-north-1", "xx-dead-1"}
	live := Discover(codes, func(s Server) bool { return s.Code != "xx-dead-1" })
	if len(live) != len(Builtin())+1 {
		t.Fatalf("got %d regions want %d: %v", len(live), len(Builtin())+1, live)
	}
	if !containsCode(live, "eu-north-1") {
		t.Error("dropped a live extra region")
	}
	if containsCode(live, "xx-dead-1") {
		t.Error("kept a region that did not respond")
	}
	for i := 1; i < len(live); i++ {
		if live[i-1].Code >= live[i].Code {
			t.Fatalf("not sorted/deduped: %v", live)
		}
	}
}

func TestDiscoverDoesNotProbeBuiltin(t *testing.T) {
	Discover([]string{"us-east-1", "eu-west-1"}, func(s Server) bool {
		t.Errorf("probed built-in region %s", s.Code)
		return false
	})
}

// A single lost UDP probe must not drop a live region.
func TestDiscoverRetriesLostProbe(t *testing.T) {
	var calls int32
	live := Discover([]string{"eu-north-1"}, func(Server) bool {
		return atomic.AddInt32(&calls, 1) > 1 // first probe lost
	})
	if !containsCode(live, "eu-north-1") {
		t.Errorf("region dropped after one lost probe (calls=%d)", calls)
	}
}

// A leftover firewall block from a previous session lets only the previously
// chosen region answer; the other built-in regions must stay in the list.
func TestDiscoverOnlyOneAnswerKeepsBuiltin(t *testing.T) {
	codes := []string{"ap-northeast-1", "eu-central-1", "eu-west-1", "us-east-1", "us-west-2"}
	live := Discover(codes, func(s Server) bool { return s.Code == "eu-west-1" })
	for _, c := range codes {
		if !containsCode(live, c) {
			t.Errorf("built-in region %s missing: %v", c, live)
		}
	}
}

func TestDiscoverEmptyFallsBack(t *testing.T) {
	if live := Discover(nil, func(Server) bool { return true }); len(live) != len(Builtin()) {
		t.Fatalf("expected builtin fallback, got %d", len(live))
	}
}

func TestPrettyName(t *testing.T) {
	if PrettyName("us-east-1") != "US East (N. Virginia)" {
		t.Error("known code wrong")
	}
	if PrettyName("zz-unknown-9") != "zz-unknown-9" {
		t.Error("unknown code should fall back to raw code")
	}
	// Live GameLift regions must not render as "code (code)".
	for _, c := range []string{"ap-southeast-5", "ap-southeast-7", "mx-central-1"} {
		if PrettyName(c) == c {
			t.Errorf("no pretty name for %s", c)
		}
	}
	for _, s := range Builtin() {
		if PrettyName(s.Code) != s.Pretty {
			t.Errorf("builtin %s pretty %q disagrees with map %q", s.Code, s.Pretty, PrettyName(s.Code))
		}
	}
}
