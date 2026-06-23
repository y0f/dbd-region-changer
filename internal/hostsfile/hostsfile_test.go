package hostsfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRemoveStripsKnownGamelift(t *testing.T) {
	p := writeTemp(t, "127.0.0.1 localhost\n9.9.9.9 gamelift.us-east-1.amazonaws.com\n")
	h := &Hosts{Path: p}
	if err := h.Remove("gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	if strings.Contains(got, "gamelift.us-east-1") {
		t.Fatal("entry not removed")
	}
	if !strings.Contains(got, "localhost") {
		t.Fatal("unrelated entry clobbered")
	}
}

func TestRemoveKeepsSingleFieldLines(t *testing.T) {
	p := writeTemp(t, "loneword\n9.9.9.9 gamelift.us-east-1.amazonaws.com\n")
	h := &Hosts{Path: p}
	if err := h.Remove("gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	if !strings.Contains(got, "loneword") {
		t.Fatal("single-field line dropped (should be kept)")
	}
}

func TestApplyWritesEntries(t *testing.T) {
	p := writeTemp(t, "127.0.0.1 localhost\n")
	h := &Hosts{Path: p}
	entries := []Entry{
		{IP: "5.5.5.5", Hostname: "gamelift.eu-west-1.amazonaws.com"},
		{IP: "5.5.5.5", Hostname: "gamelift.us-east-1.amazonaws.com"},
	}
	if err := h.Apply(entries); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	for _, e := range entries {
		if !strings.Contains(got, e.IP+" "+e.Hostname) {
			t.Fatalf("missing %q", e.IP+" "+e.Hostname)
		}
	}
}

func TestSaveNoTrailingNewlinePrefixesOne(t *testing.T) {
	// no trailing newline -> Save must prefix one so entries do not concatenate.
	p := writeTemp(t, "127.0.0.1 localhost")
	h := &Hosts{Path: p}
	if err := h.Save("5.5.5.5", "gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	if got != "127.0.0.1 localhost\n5.5.5.5 gamelift.us-east-1.amazonaws.com" {
		t.Fatalf("got %q", got)
	}
}

func TestSaveSingleTrailingNewlineStillPrefixes(t *testing.T) {
	// Parity with HostHub.save: readlines()[-1] of "...localhost\n" is
	// "...localhost\n", whose strip() is non-blank, so a newline IS prefixed
	// (yielding a blank separator line). Reproduce exactly.
	p := writeTemp(t, "127.0.0.1 localhost\n")
	h := &Hosts{Path: p}
	if err := h.Save("5.5.5.5", "gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	if got != "127.0.0.1 localhost\n\n5.5.5.5 gamelift.us-east-1.amazonaws.com" {
		t.Fatalf("got %q", got)
	}
}

func TestSaveBlankLastLineNoPrefix(t *testing.T) {
	// File ending in "\n\n" has a genuinely blank last readline -> no prefix.
	p := writeTemp(t, "127.0.0.1 localhost\n\n")
	h := &Hosts{Path: p}
	if err := h.Save("5.5.5.5", "gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Fatal(err)
	}
	got := read(t, p)
	if got != "127.0.0.1 localhost\n\n5.5.5.5 gamelift.us-east-1.amazonaws.com" {
		t.Fatalf("got %q", got)
	}
}

func TestListFiltersByCallback(t *testing.T) {
	p := writeTemp(t, "# comment\n127.0.0.1 localhost\n9.9.9.9 gamelift.us-east-1.amazonaws.com\n")
	h := &Hosts{Path: p}
	got, err := h.List(func(hn string) bool {
		return strings.HasPrefix(hn, "gamelift.")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0][1] != "gamelift.us-east-1.amazonaws.com" {
		t.Fatalf("List=%v", got)
	}
}
