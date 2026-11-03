package hostsfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSaveLineEndings(t *testing.T) {
	const e = "5.5.5.5 gamelift.us-east-1.amazonaws.com"
	for _, tc := range []struct{ in, want string }{
		{"127.0.0.1 localhost", "127.0.0.1 localhost\n" + e + "\n"},
		{"127.0.0.1 localhost\n", "127.0.0.1 localhost\n" + e + "\n"},
		{"127.0.0.1 localhost\n\n", "127.0.0.1 localhost\n\n" + e + "\n"},
		{"127.0.0.1 localhost\r\n", "127.0.0.1 localhost\r\n" + e + "\r\n"},
		{"127.0.0.1 localhost\r\n::1 localhost", "127.0.0.1 localhost\r\n::1 localhost\r\n" + e + "\r\n"},
		{"", e + "\n"},
	} {
		p := writeTemp(t, tc.in)
		if err := (&Hosts{Path: p}).Save("5.5.5.5", "gamelift.us-east-1.amazonaws.com"); err != nil {
			t.Fatal(err)
		}
		if got := read(t, p); got != tc.want {
			t.Errorf("Save on %q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSaveCreatesMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	if err := (&Hosts{Path: p}).Save("5.5.5.5", "a.example"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "5.5.5.5 a.example\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoveAllMissingFileIsNoop(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	if err := (&Hosts{Path: p}).RemoveAll([]string{"a.example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("RemoveAll created the hosts file")
	}
}

// RemoveAll with nothing to remove must not rewrite the hosts file.
func TestRemoveAllNoMatchDoesNotWrite(t *testing.T) {
	orig := "127.0.0.1 localhost\r\n::1 localhost\r\n"
	p := writeTemp(t, orig)
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	var names []string
	for i := 0; i < 60; i++ {
		names = append(names, fmt.Sprintf("gamelift-ping.r%d.api.aws", i))
	}
	if err := (&Hosts{Path: p}).RemoveAll(names); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != orig {
		t.Fatalf("content changed: %q", got)
	}
	if !fi.ModTime().Equal(old) {
		t.Errorf("hosts file rewritten with no content change (mtime %v)", fi.ModTime())
	}
}

func TestUpdateRemovesThenAppends(t *testing.T) {
	p := writeTemp(t, "127.0.0.1 localhost\n9.9.9.9 a.example\n9.9.9.9 b.example\n")
	h := &Hosts{Path: p}
	if err := h.Update([]string{"a.example", "b.example"}, []Entry{{"1.2.3.4", "a.example"}}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "127.0.0.1 localhost\n1.2.3.4 a.example\n" {
		t.Fatalf("got %q", got)
	}
}

// An external append after Apply must land on its own line and survive RemoveAll.
func TestExternalAppendStaysSeparate(t *testing.T) {
	p := writeTemp(t, "127.0.0.1 localhost\n")
	h := &Hosts{Path: p}
	if err := h.Apply([]Entry{{"1.2.3.4", "gamelift-ping.us-west-2.api.aws"}}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("5.6.7.8 myhost\n") // like: echo '5.6.7.8 myhost' >> /etc/hosts
	f.Close()
	if err := h.RemoveAll([]string{"gamelift-ping.us-west-2.api.aws"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "127.0.0.1 localhost\n5.6.7.8 myhost\n" {
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

// The hosts file is written by an elevated process, so an entry must never be
// able to inject a second line or a malformed one; nothing is written then.
func TestUpdateRejectsMalformedEntries(t *testing.T) {
	for _, e := range []Entry{
		{IP: "1.2.3.4", Hostname: "gamelift.eu-west-1\n6.6.6.6 evil.example"},
		{IP: "1.2.3.4", Hostname: "a b"},
		{IP: "1.2.3.4", Hostname: "a\x00b"},
		{IP: "1.2.3.4", Hostname: ""},
		{IP: "1.2.3.4 evil", Hostname: "gamelift.us-east-1.amazonaws.com"},
		{IP: "not-an-ip", Hostname: "gamelift.us-east-1.amazonaws.com"},
	} {
		p := writeTemp(t, "127.0.0.1 localhost\n")
		h := &Hosts{Path: p}
		if err := h.Update([]string{"localhost"}, []Entry{e}); err == nil {
			t.Errorf("%q accepted", e)
		}
		if got := read(t, p); got != "127.0.0.1 localhost\n" {
			t.Errorf("%q: file changed to %q", e, got)
		}
	}
	if err := (&Hosts{Path: writeTemp(t, "")}).Save("::1", "gamelift.us-east-1.amazonaws.com"); err != nil {
		t.Errorf("valid IPv6 entry rejected: %v", err)
	}
}
