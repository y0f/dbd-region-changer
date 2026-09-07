package region

import "testing"

func TestDiscoverFiltersByProbe(t *testing.T) {
	codes := []string{"us-east-1", "eu-central-1", "xx-dead-1"}
	live := Discover(codes, func(s Server) bool { return s.Code != "xx-dead-1" })
	if len(live) != 2 {
		t.Fatalf("got %d live", len(live))
	}
	if !containsCode(live, "us-east-1") || !containsCode(live, "eu-central-1") {
		t.Error("dropped a live region")
	}
	if containsCode(live, "xx-dead-1") {
		t.Error("kept a region that did not respond")
	}
	// sorted
	if live[0].Code != "eu-central-1" {
		t.Errorf("not sorted: %v", live)
	}
}

func TestDiscoverFallsBackWhenNoneRespond(t *testing.T) {
	live := Discover([]string{"us-east-1", "eu-west-1"}, func(Server) bool { return false })
	if len(live) != len(Builtin()) {
		t.Fatalf("expected builtin fallback (%d), got %d", len(Builtin()), len(live))
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
}
