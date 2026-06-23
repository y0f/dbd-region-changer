//go:build windows

package ping

import probing "github.com/prometheus-community/pro-bing"

// privileged mode: on Win10/11 pro-bing uses the OS ICMP echo and needs no admin; UDP path unsupported here.
func configurePinger(p *probing.Pinger) {
	p.SetPrivileged(true)
}
