// Package beaconpool collects every address a GameLift ping beacon hands out.
//
// A beacon name such as gamelift-ping.eu-central-1.api.aws has a small pool of
// addresses per family, and each DNS answer carries one of them at random with
// a 60s TTL. Public DoH resolvers cache that one answer, so sampling them a few
// times in a row returns the same address and misses the rest of the pool. The
// game resolves the name itself and can be handed any address in the pool, so
// a block built from a partial pool leaves the region reachable.
//
// This package asks the zone's authoritative nameservers directly. They do not
// cache, so every query is a fresh draw from the pool, and enough draws see
// all of it.
package beaconpool

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// DefaultRounds is how many queries are sent per address family. Each answer
// is a uniform draw from the pool, so with the two-address pools the beacons
// use, the chance that one address is never drawn is 2 * 0.5^24, about 1e-7.
const DefaultRounds = 24

// queryTimeout bounds one UDP query.
const queryTimeout = 2 * time.Second

// Collector queries authoritative nameservers for the full address pool of a
// beacon name. The zero value is not usable; use New.
type Collector struct {
	// Nameservers returns the "ip:port" addresses of the authoritative
	// servers for host.
	Nameservers func(ctx context.Context, host string) ([]string, error)
	// Rounds is the number of queries per address family.
	Rounds int
	// Timeout bounds each query.
	Timeout time.Duration
}

// New returns a Collector that finds the authoritative servers through the
// system resolver.
func New() *Collector {
	return &Collector{
		Nameservers: (&nsCache{}).lookup,
		Rounds:      DefaultRounds,
		Timeout:     queryTimeout,
	}
}

// Collect returns every IPv4 and IPv6 address the authoritative servers handed
// out for host, sorted and deduplicated. It fails only if no query succeeded.
func (c *Collector) Collect(ctx context.Context, host string) ([]string, error) {
	servers, err := c.Nameservers(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no authoritative nameserver found for %s", host)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		seen    = map[string]bool{}
		lastErr error
		okCount int
	)
	for _, qtype := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		for i := 0; i < c.Rounds; i++ {
			server := servers[i%len(servers)]
			wg.Go(func() {
				ips, err := query(ctx, server, host, qtype, c.Timeout)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					lastErr = err
					return
				}
				okCount++
				for _, ip := range ips {
					seen[ip] = true
				}
			})
		}
	}
	wg.Wait()
	if okCount == 0 {
		if lastErr == nil {
			lastErr = errors.New("no query sent")
		}
		return nil, fmt.Errorf("authoritative lookup of %s failed: %w", host, lastErr)
	}
	out := make([]string, 0, len(seen))
	for ip := range seen {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out, nil
}

// query sends one non-recursive question to server and returns the addresses
// of type qtype in the answer section.
func query(ctx context.Context, server, host string, qtype dnsmessage.Type, timeout time.Duration) ([]string, error) {
	name, err := dnsmessage.NewName(fqdn(host))
	if err != nil {
		return nil, err
	}
	var idBuf [2]byte
	if _, err := rand.Read(idBuf[:]); err != nil {
		return nil, err
	}
	id := binary.BigEndian.Uint16(idBuf[:])
	msg := dnsmessage.Message{
		Header:    dnsmessage.Header{ID: id},
		Questions: []dnsmessage.Question{{Name: name, Type: qtype, Class: dnsmessage.ClassINET}},
	}
	packed, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if _, err := conn.Write(packed); err != nil {
		return nil, err
	}

	buf := make([]byte, 1232)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		var resp dnsmessage.Message
		if err := resp.Unpack(buf[:n]); err != nil || resp.ID != id || !resp.Response {
			continue // not our answer; keep waiting until the deadline
		}
		if resp.RCode != dnsmessage.RCodeSuccess {
			return nil, fmt.Errorf("%s answered %s for %s", server, resp.RCode, host)
		}
		var ips []string
		for _, a := range resp.Answers {
			switch body := a.Body.(type) {
			case *dnsmessage.AResource:
				if qtype == dnsmessage.TypeA {
					ips = append(ips, net.IP(body.A[:]).String())
				}
			case *dnsmessage.AAAAResource:
				if qtype == dnsmessage.TypeAAAA {
					ips = append(ips, net.IP(body.AAAA[:]).String())
				}
			}
		}
		return ips, nil
	}
}

// nsCache finds and remembers the authoritative servers of each zone, so the
// beacons of all regions (which share a zone) cost one lookup.
type nsCache struct {
	mu    sync.Mutex
	zones map[string][]string
}

// lookup walks up from host to the closest name with NS records and returns
// its servers' addresses on port 53.
func (c *nsCache) lookup(ctx context.Context, host string) ([]string, error) {
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	var lastErr error
	for i := 0; i < len(labels)-1; i++ {
		zone := strings.Join(labels[i:], ".")
		c.mu.Lock()
		cached, ok := c.zones[zone]
		c.mu.Unlock()
		if ok {
			return cached, nil
		}
		nss, err := net.DefaultResolver.LookupNS(ctx, zone)
		if err != nil || len(nss) == 0 {
			lastErr = err
			continue
		}
		var addrs []string
		for _, ns := range nss {
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, ns.Host)
			if err != nil {
				lastErr = err
				continue
			}
			for _, ip := range ips {
				if ip.IP.To4() != nil { // IPv6 to the nameserver may not be routed
					addrs = append(addrs, net.JoinHostPort(ip.IP.String(), "53"))
				}
			}
		}
		if len(addrs) == 0 {
			continue
		}
		c.mu.Lock()
		if c.zones == nil {
			c.zones = map[string][]string{}
		}
		c.zones[zone] = addrs
		c.mu.Unlock()
		return addrs, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no NS records")
	}
	return nil, fmt.Errorf("find nameservers for %s: %w", host, lastErr)
}

// fqdn returns host with the trailing dot DNS names carry on the wire.
func fqdn(host string) string {
	if strings.HasSuffix(host, ".") {
		return host
	}
	return host + "."
}
