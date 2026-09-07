// Package firewall blocks the game's traffic to unwanted AWS regions so it can
// only reach the chosen region. This holds where the hosts-redirect method
// cannot, because it blocks the actual game-server IPs, not just DNS.
package firewall

import "errors"

const ruleName = "DBD Region Changer"

// maxCIDRsPerRule keeps each netsh command under the command-length limit.
const maxCIDRsPerRule = 200

// ErrUnsupported is returned where no firewall backend exists.
var ErrUnsupported = errors.New("firewall blocking not supported on this platform")

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
