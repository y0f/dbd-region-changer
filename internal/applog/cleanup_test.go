package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloseRemovesTempDir(t *testing.T) {
	l, err := New(false)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(l.LogPath())
	t.Cleanup(func() { os.RemoveAll(dir) })
	l.Info("x")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("temp dir %s still exists after Close", dir)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	l.Cleanup()
}

func TestSweepStaleRemovesOnlyOldLogDirs(t *testing.T) {
	root := t.TempDir()
	mk := func(name string, age time.Duration) string {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(dir, "log.log")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-age)
		os.Chtimes(f, old, old)
		os.Chtimes(dir, old, old)
		return dir
	}
	stale := mk(tempPrefix+"old", 2*staleAge)
	fresh := mk(tempPrefix+"new", time.Minute)
	other := mk("unrelated_old", 2*staleAge)

	sweepStale(root, time.Now().Add(-staleAge))

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale dir %s was not removed", stale)
	}
	for _, keep := range []string{fresh, other} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s should be kept: %v", keep, err)
		}
	}
}

func TestWithAttrsAndGroupsAreLogged(t *testing.T) {
	l, err := New(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Cleanup)
	l.With("region", "eu-west-1").Info("m")
	l.WithGroup("g").Info("n", "k", "v")
	l.WithGroup("g").With("a", 1).WithGroup("h").Info("o", "b", 2)
	data, err := os.ReadFile(l.LogPath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"m region=eu-west-1", "n g.k=v", "o g.a=1 g.h.b=2"} {
		if !strings.Contains(s, want) {
			t.Errorf("log missing %q:\n%s", want, s)
		}
	}
}
