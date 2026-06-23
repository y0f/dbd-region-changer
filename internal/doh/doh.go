// Package doh resolves hostnames via DNS-over-HTTPS JSON with provider failover and a TTL cache.
package doh

import (
	"encoding/json"
	"fmt"
	"io"
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

const cacheTTL = 600 * time.Second

type cacheEntry struct {
	ip      string
	expires time.Time
}

type Resolver struct {
	providers []Provider
	client    *http.Client
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func New(providers []Provider) *Resolver {
	return &Resolver{
		providers: providers,
		client:    &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
		cache:     make(map[string]cacheEntry),
	}
}

// Default resolves against the configured Cloudflare/Quad9 providers.
func Default() *Resolver { return New(DefaultProviders()) }

type dnsResponse struct {
	Answer []struct {
		Data string `json:"data"`
	} `json:"Answer"`
}

// Resolve returns the first A record for hostname, trying providers in order and caching hits.
func (r *Resolver) Resolve(hostname string) (string, error) {
	if ip, ok := r.cached(hostname); ok {
		return ip, nil
	}
	return r.ResolveFresh(hostname)
}

// ResolveFresh skips the cache read (still updating it on success), so callers
// can detect rotated records.
func (r *Resolver) ResolveFresh(hostname string) (string, error) {
	var lastErr error
	for _, p := range r.providers {
		ip, err := r.query(p, hostname)
		if err != nil {
			lastErr = err
			continue
		}
		r.store(hostname, ip)
		return ip, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no DoH provider returned an answer for %s", hostname)
	}
	return "", lastErr
}

func (r *Resolver) query(p Provider, hostname string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, p.URL, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Set("name", hostname)
	q.Set("type", "A")
	q.Set("ct", "application/dns-json")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("accept", "application/dns-json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("%s returned status %d", p.Name, resp.StatusCode)
	}
	var parsed dnsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Answer) == 0 {
		return "", fmt.Errorf("%s returned no answers for %s", p.Name, hostname)
	}
	return parsed.Answer[0].Data, nil
}

func (r *Resolver) cached(hostname string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.cache[hostname]
	if !ok || r.now().After(e.expires) {
		if ok {
			delete(r.cache, hostname)
		}
		return "", false
	}
	return e.ip, true
}

func (r *Resolver) store(hostname, ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[hostname] = cacheEntry{ip: ip, expires: r.now().Add(cacheTTL)}
}
