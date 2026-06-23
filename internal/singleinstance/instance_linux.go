//go:build linux

package singleinstance

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type linuxLock struct {
	fl     *flock.Flock
	locked bool
}

// Acquire takes a flock on a lockfile in the runtime/cache dir; failure to lock means another instance holds it.
func Acquire(name string) (Lock, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		if d, err := os.UserCacheDir(); err == nil {
			dir = d
		} else {
			dir = os.TempDir()
		}
	}
	fl := flock.New(filepath.Join(dir, name+".lock"))
	locked, err := fl.TryLock()
	if err != nil {
		return nil, err
	}
	return &linuxLock{fl: fl, locked: locked}, nil
}

func (l *linuxLock) IsRunning() bool { return !l.locked }

func (l *linuxLock) Release() {
	if l.locked {
		l.fl.Unlock()
		l.locked = false
	}
}
