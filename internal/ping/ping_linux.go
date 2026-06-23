//go:build linux

package ping

import probing "github.com/prometheus-community/pro-bing"

// unprivileged UDP ICMP datagram socket: works when net.ipv4.ping_group_range covers the user.
func configurePinger(p *probing.Pinger) {
	p.SetPrivileged(false)
}
