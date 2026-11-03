//go:build linux

package singleinstance

import "github.com/gofrs/flock"

type linuxLock struct {
	fl     *flock.Flock
	locked bool
}

// Acquire takes a flock on a lockfile in the runtime/cache dir; failure to lock means another instance holds it.
func Acquire(name string) (Lock, error) {
	path, err := lockFile(name)
	if err != nil {
		return nil, err
	}
	fl := flock.New(path)
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
