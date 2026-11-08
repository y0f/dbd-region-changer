// Package doh resolves hostnames via DNS-over-HTTPS JSON with provider failover and a TTL cache.
package doh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/y0f/dbd-region-changer/internal/config"
)

type Provider struct {
	Name string
	URL  string
}

func DefaultProviders() []Provider {
	out := make([]Provider, 0, len(config.DNSProviders))
	for _, p := range config.DNSProviders {
		out = append(out, Provider{Name: p.Name, URL: p.URL})
	}
	return out
}

// maxCacheTTL caps how long an answer is cached; shorter record TTLs win, so
// rotating records (the GameLift beacons use 10-60s) are re-resolved in time.
const maxCacheTTL = 600 * time.Second

// DNS record types as they appear in the JSON "type" field.
const (
	typeA    = 1
	typeAAAA = 28
)

type cacheKey struct {
	host  string
	qtype int
}

type cacheEntry struct {
	ips     []string
	expires time.Time
}

type Resolver struct {
	ctx       context.Context
	providers []Provider
	client    *http.Client
	now       func() time.Time

	mu    sync.Mutex
	cache map[cacheKey]cacheEntry
}

func New(providers []Provider) *Resolver {
	return NewContext(context.Background(), providers)
}

func NewContext(ctx context.Context, providers []Provider) *Resolver {
	return &Resolver{
		ctx:       ctx,
		providers: providers,
		client:    &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
		cache:     make(map[cacheKey]cacheEntry),
	}
}

// Default resolves against the configured providers (see config.DNSProviders).
func Default() *Resolver { return New(DefaultProviders()) }

// dnsAnswer is one JSON answer record. Type and TTL are optional: a record
// without a type is accepted if its data is an address of the queried family,
// and one without a TTL is cached for maxCacheTTL.
type dnsAnswer struct {
	Type int    `json:"type"`
	TTL  *int   `json:"TTL"`
	Data string `json:"data"`
}

type dnsResponse struct {
	Answer []dnsAnswer `json:"Answer"`
}

// Resolve returns the first IPv4 address for hostname, trying providers in
// order and caching hits for the record's TTL (capped at maxCacheTTL).
func (r *Resolver) Resolve(hostname string) (string, error) {
	ips, err := r.lookup(hostname, typeA, true)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}

// ResolveFresh skips the cache read (still updating it on success), so callers
// can detect rotated records.
func (r *Resolver) ResolveFresh(hostname string) (string, error) {
	ips, err := r.lookup(hostname, typeA, false)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}

// ResolveAll returns every IPv4 and IPv6 address of hostname (A then AAAA), so
// dual-stack names can be blocked on both families. It fails only if neither
// family resolves.
func (r *Resolver) ResolveAll(hostname string) ([]string, error) {
	return r.resolveAll(hostname, true)
}

// ResolveAllFresh samples both address families without reading the TTL cache.
func (r *Resolver) ResolveAllFresh(hostname string) ([]string, error) {
	return r.resolveAll(hostname, false)
}

func (r *Resolver) resolveAll(hostname string, useCache bool) ([]string, error) {
	var v4, v6 []string
	var err4, err6 error
	var wg sync.WaitGroup
	wg.Go(func() { v4, err4 = r.lookup(hostname, typeA, useCache) })
	wg.Go(func() { v6, err6 = r.lookup(hostname, typeAAAA, useCache) })
	wg.Wait()
	out := make([]string, 0, len(v4)+len(v6))
	out = append(append(out, v4...), v6...)
	if len(out) == 0 {
		return nil, errors.Join(err4, err6)
	}
	return out, nil
}

func (r *Resolver) lookup(hostname string, qtype int, useCache bool) ([]string, error) {
	key := cacheKey{hostname, qtype}
	if useCache {
		if ips, ok := r.cached(key); ok {
			return ips, nil
		}
	}
	var lastErr error
	for _, p := range r.providers {
		if err := r.ctx.Err(); err != nil {
			return nil, err
		}
		ips, ttl, err := r.query(p, hostname, qtype)
		if err != nil {
			lastErr = err
			continue
		}
		r.store(key, ips, ttl)
		return ips, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no DoH provider returned an answer for %s", hostname)
	}
	return nil, lastErr
}

// query returns the addresses of the requested type (skipping CNAMEs and other
// records in the chain) and the smallest TTL among them.
func (r *Resolver) query(p Provider, hostname string, qtype int) ([]string, time.Duration, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, 0, err
	}
	typeName := "A"
	if qtype == typeAAAA {
		typeName = "AAAA"
	}
	q := req.URL.Query()
	q.Set("name", hostname)
	q.Set("type", typeName)
	q.Set("ct", "application/dns-json")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("accept", "application/dns-json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, 0, fmt.Errorf("%s returned status %d", p.Name, resp.StatusCode)
	}
	var parsed dnsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, 0, err
	}
	var ips []string
	ttl := maxCacheTTL
	for _, a := range parsed.Answer {
		ip := net.ParseIP(a.Data)
		if (a.Type != qtype && a.Type != 0) || ip == nil || (qtype == typeA) != (ip.To4() != nil) {
			continue
		}
		ips = append(ips, ip.String())
		if a.TTL != nil {
			ttl = min(ttl, time.Duration(max(*a.TTL, 0))*time.Second)
		}
	}
	if len(ips) == 0 {
		return nil, 0, fmt.Errorf("%s returned no %s records for %s", p.Name, typeName, hostname)
	}
	return ips, ttl, nil
}

func (r *Resolver) cached(key cacheKey) ([]string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.cache[key]
	if !ok || !r.now().Before(e.expires) {
		if ok {
			delete(r.cache, key)
		}
		return nil, false
	}
	return e.ips, true
}

func (r *Resolver) store(key cacheKey, ips []string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = cacheEntry{ips: ips, expires: r.now().Add(ttl)}
}
