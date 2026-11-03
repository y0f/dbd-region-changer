// Package singleinstance provides a single-instance lock: named mutex on Windows, flock on Linux.
package singleinstance

import (
	"os"
	"path/filepath"
)

// DefaultName is the mutex/lockfile name used to detect a second instance.
const DefaultName = "dcreg_{c60ca70d-59a9-43c6-bd96-44f755b45095}"

type Lock interface {
	IsRunning() bool
	Release()
}

// lockFile returns the lockfile path for name in $XDG_RUNTIME_DIR, else the
// user cache dir, else the temp dir, creating the directory if needed (the
// cache dir does not exist yet on a fresh account, and flock does not create
// parent directories). Used by the Linux backend.
func lockFile(name string) (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		if d, err := os.UserCacheDir(); err == nil {
			dir = d
		} else {
			dir = os.TempDir()
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".lock"), nil
}
