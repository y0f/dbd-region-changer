// Package firewall blocks traffic to unwanted regions' GameLift ping beacons.
package firewall

import (
	"errors"
	"fmt"
)

const ruleName = "DBD Region Changer"

// maxCIDRsPerRule keeps each netsh command under the command-length limit.
const maxCIDRsPerRule = 200

// ErrUnsupported is returned where no firewall backend exists.
var ErrUnsupported = errors.New("firewall blocking not supported on this platform")

// ErrNotRun is returned when a change never ran, for example because the
// elevation helper could not be started, so nothing was changed. Any other
// error means the change ran and failed.
var ErrNotRun = errors.New("firewall change did not run")

// ErrNotAuthorized is returned when the password prompt for a change was
// refused or dismissed. It matches ErrNotRun: nothing was changed.
var ErrNotAuthorized = fmt.Errorf("not authorized: %w", ErrNotRun)

// ErrStateUnknown means cleanup failed and some rules may still be active.
var ErrStateUnknown = errors.New("firewall state could not be verified")

func chunk(cidrs []string, size int) [][]string {
	var out [][]string
	for i := 0; i < len(cidrs); i += size {
		end := i + size
		if end > len(cidrs) {
			end = len(cidrs)
		}
		out = append(out, cidrs[i:end])
	}
	return out
}
