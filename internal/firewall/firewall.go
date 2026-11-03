// Package firewall blocks the game's traffic to unwanted AWS regions so it can
// only reach the chosen region. This holds where the hosts-redirect method
// cannot, because it blocks the actual game-server IPs, not just DNS.
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
