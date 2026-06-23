// Package apply owns the remove-then-write sequence that forces a region.
package apply

import (
	"fmt"

	"github.com/y0f/dbd-region-changer/internal/hostsfile"
	"github.com/y0f/dbd-region-changer/internal/region"
)

// Resolver is satisfied by *doh.Resolver.
type Resolver interface {
	Resolve(host string) (string, error)
}

// HostsWriter is satisfied by *hostsfile.Hosts.
type HostsWriter interface {
	RemoveAll(hostnames []string) error
	Apply(entries []hostsfile.Entry) error
}

// KnownHostnames returns every region's endpoint+dualstack hostname, used to strip stale entries.
func KnownHostnames(regions []region.Server) []string {
	out := make([]string, 0, len(regions)*2)
	for _, r := range regions {
		out = append(out, r.Endpoint(), r.Dualstack())
	}
	return out
}

// BuildEntries resolves the selected region's two IPs and points every region's endpoint+dualstack at them, selected pair first.
func BuildEntries(selectedCode string, regions []region.Server, r Resolver) ([]hostsfile.Entry, error) {
	var selected region.Server
	found := false
	for _, s := range regions {
		if s.Code == selectedCode {
			selected, found = s, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("region %q not found", selectedCode)
	}

	endpointIP, err := r.Resolve(selected.Endpoint())
	if err != nil {
		return nil, err
	}
	dualstackIP, err := r.Resolve(selected.Dualstack())
	if err != nil {
		return nil, err
	}

	entries := []hostsfile.Entry{
		{IP: endpointIP, Hostname: selected.Endpoint()},
		{IP: dualstackIP, Hostname: selected.Dualstack()},
	}
	for _, s := range regions {
		if s.Code == selectedCode {
			continue
		}
		entries = append(entries,
			hostsfile.Entry{IP: endpointIP, Hostname: s.Endpoint()},
			hostsfile.Entry{IP: dualstackIP, Hostname: s.Dualstack()},
		)
	}
	return entries, nil
}

func SetServer(h HostsWriter, selectedCode string, regions []region.Server, r Resolver) error {
	entries, err := BuildEntries(selectedCode, regions, r)
	if err != nil {
		return err
	}
	if err := h.RemoveAll(KnownHostnames(regions)); err != nil {
		return err
	}
	return h.Apply(entries)
}

// ClearServer removes every known entry (Set Default).
func ClearServer(h HostsWriter, regions []region.Server) error {
	return h.RemoveAll(KnownHostnames(regions))
}
