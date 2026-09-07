// Package singleinstance provides a single-instance lock: named mutex on Windows, flock on Linux.
package singleinstance

// DefaultName is the mutex/lockfile name used to detect a second instance.
const DefaultName = "dcreg_{c60ca70d-59a9-43c6-bd96-44f755b45095}"

type Lock interface {
	IsRunning() bool
	Release()
}
