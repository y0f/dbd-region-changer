//go:build linux

// Command beacon-e2e checks the Linux region lock against the live GameLift
// beacons with real iptables rules. It runs as root in a throwaway container
// (see build/Dockerfile.arch-e2e) and changes that container's firewall only.
//
// For each way of collecting beacon addresses (the old DoH-only sampling and
// the authoritative pool), it applies the block for -allow, then sends a UDP
// ping to every address the authoritative servers hand out for every region,
// which is every address the game can be given. A lock works only if no
// blocked region answers while the allowed one still does.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"

	"github.com/y0f/dbd-region-changer/internal/beaconpool"
	"github.com/y0f/dbd-region-changer/internal/doh"
	"github.com/y0f/dbd-region-changer/internal/firewall"
	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

func main() {
	allow := flag.String("allow", "us-east-1", "region left open")
	flag.Parse()
	ctx := context.Background()
	regions := region.Builtin()
	pool := beaconpool.New()
	res := doh.Default()

	// Every address the game can be handed, per region.
	reachable := map[string][]string{}
	for _, r := range regions {
		ips, err := pool.Collect(ctx, r.Dualstack())
		if err != nil {
			fail("pool of %s: %v", r.Code, err)
		}
		reachable[r.Code] = ips
	}

	dohOnly := func(host string) []string { // the collection before the fix
		var ips []string
		for range 4 {
			all, _ := res.ResolveAllFresh(host)
			ips = append(ips, all...)
		}
		return ips
	}
	withPool := func(host string) []string { // the collection after the fix
		ips, _ := pool.Collect(ctx, host)
		return append(ips, dohOnly(host)...)
	}

	leaks := map[string]int{}
	for _, m := range []struct {
		name    string
		collect func(string) []string
	}{{"before (DoH sampling)", dohOnly}, {"after (authoritative pool)", withPool}} {
		block := map[string]bool{}
		for _, r := range regions {
			if r.Code != *allow {
				for _, ip := range m.collect(r.Dualstack()) {
					block[ip] = true
				}
			}
		}
		ips := keys(block)
		if err := firewall.Apply("", ips); err != nil {
			fail("apply: %v", err)
		}
		fmt.Printf("\n== %s: %d addresses blocked, allow=%s\n", m.name, len(ips), *allow)
		allowedUp := 0
		for _, r := range regions {
			up := answering(reachable[r.Code])
			switch {
			case r.Code == *allow:
				allowedUp = len(up)
				fmt.Printf("   %-15s allowed   answering %v\n", r.Code, up)
			case len(up) > 0:
				leaks[m.name]++
				fmt.Printf("   %-15s LEAK      answering %v\n", r.Code, up)
			}
		}
		if allowedUp == 0 {
			fail("allowed region %s does not answer; the network check is not meaningful", *allow)
		}
		fmt.Printf("   regions still reachable that should be blocked: %d\n", leaks[m.name])
		if err := firewall.Clear(); err != nil {
			fail("clear: %v", err)
		}
	}

	if leaks["after (authoritative pool)"] > 0 {
		fail("region lock still leaks after the fix")
	}
	fmt.Println("\nPASS: with the authoritative pool no blocked region answers")
}

// answering returns the IPv4 addresses that echo a UDP beacon ping. IPv6 is
// skipped: the container has no IPv6 route.
func answering(ips []string) []string {
	var (
		mu  sync.Mutex
		out []string
		wg  sync.WaitGroup
	)
	for _, ip := range ips {
		if net.ParseIP(ip).To4() == nil {
			continue
		}
		wg.Go(func() {
			ok := false
			for range 2 { // one lost datagram must not count as blocked
				if _, ok = ping.UDPBeacon(ip); ok {
					break
				}
			}
			if ok {
				mu.Lock()
				out = append(out, ip)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	sort.Strings(out)
	return out
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
