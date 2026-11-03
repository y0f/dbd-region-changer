package singleinstance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

// Without XDG_RUNTIME_DIR the lock falls back to the user cache dir, which does
// not exist yet on a fresh account; lockFile must create it so the lock works.
func TestLockFileCreatesMissingCacheDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", home)                                   // Linux: $HOME/.cache
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData")) // Windows
	path, err := lockFile(DefaultName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("lock dir not created: %v", err)
	}
	fl := flock.New(path)
	locked, err := fl.TryLock()
	if err != nil || !locked {
		t.Fatalf("TryLock locked=%v err=%v", locked, err)
	}
	fl.Unlock()
}
