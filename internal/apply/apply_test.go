package apply

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/y0f/dbd-region-changer/internal/hostsfile"
	"github.com/y0f/dbd-region-changer/internal/region"
)

type fakeResolver struct{ ips map[string]string }

func (f fakeResolver) Resolve(host string) (string, error) {
	if ip, ok := f.ips[host]; ok {
		return ip, nil
	}
	return "0.0.0.0", nil
}

func TestBuildEntriesPointsEveryRegionAtSelectedIPs(t *testing.T) {
	regions := []region.Server{{Code: "us-east-1"}, {Code: "eu-west-1"}}
	res := fakeResolver{ips: map[string]string{
		"gamelift.us-east-1.amazonaws.com": "1.1.1.1",
		"gamelift-ping.us-east-1.api.aws":  "2.2.2.2",
	}}
	entries, err := BuildEntries("us-east-1", regions, res)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Hostname] = e.IP
	}
	want := map[string]string{
		"gamelift.us-east-1.amazonaws.com": "1.1.1.1",
		"gamelift-ping.us-east-1.api.aws":  "2.2.2.2",
		"gamelift.eu-west-1.amazonaws.com": "1.1.1.1",
		"gamelift-ping.eu-west-1.api.aws":  "2.2.2.2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// selected region's pair must come first.
	if entries[0].Hostname != "gamelift.us-east-1.amazonaws.com" {
		t.Fatalf("selected endpoint not first: %v", entries[0])
	}
}

func TestBuildEntriesUnknownRegion(t *testing.T) {
	_, err := BuildEntries("zz-nope-1", []region.Server{{Code: "us-east-1"}}, fakeResolver{})
	if err == nil {
		t.Fatal("expected error for unknown region")
	}
}

func TestKnownHostnames(t *testing.T) {
	got := KnownHostnames([]region.Server{{Code: "us-east-1"}})
	want := []string{"gamelift.us-east-1.amazonaws.com", "gamelift-ping.us-east-1.api.aws"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSetThenClearRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	// pre-seed an old gamelift entry + an unrelated entry.
	if err := os.WriteFile(p, []byte("127.0.0.1 localhost\n8.8.8.8 gamelift.us-east-1.amazonaws.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &hostsfile.Hosts{Path: p}
	regions := []region.Server{{Code: "us-east-1"}, {Code: "eu-west-1"}}
	res := fakeResolver{ips: map[string]string{
		"gamelift.eu-west-1.amazonaws.com": "3.3.3.3",
		"gamelift-ping.eu-west-1.api.aws":  "4.4.4.4",
	}}

	if err := SetServer(h, "eu-west-1", regions, res); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, p)
	if strings.Contains(got, "8.8.8.8") {
		t.Error("stale entry not removed")
	}
	if !strings.Contains(got, "127.0.0.1 localhost") {
		t.Error("unrelated entry clobbered")
	}
	// every region endpoint points at eu-west-1's IPs.
	for _, want := range []string{
		"3.3.3.3 gamelift.eu-west-1.amazonaws.com",
		"4.4.4.4 gamelift-ping.eu-west-1.api.aws",
		"3.3.3.3 gamelift.us-east-1.amazonaws.com",
		"4.4.4.4 gamelift-ping.us-east-1.api.aws",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	if err := ClearServer(h, regions); err != nil {
		t.Fatal(err)
	}
	got = mustRead(t, p)
	if strings.Contains(got, "gamelift.") {
		t.Errorf("gamelift entries remain after clear: %q", got)
	}
	if !strings.Contains(got, "127.0.0.1 localhost") {
		t.Error("unrelated entry lost after clear")
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
