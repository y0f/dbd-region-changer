package beaconpool

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/y0f/dbd-region-changer/internal/doh"
)

const beacon = "gamelift-ping.eu-central-1.api.aws"

// The pool shape measured on the live beacons: two addresses per family.
var (
	poolV4 = []string{"192.0.2.10", "192.0.2.20"}
	poolV6 = []string{"2001:db8::10", "2001:db8::20"}
	pool   = sorted(append(slices.Clone(poolV4), poolV6...))
)

func sorted(s []string) []string { slices.Sort(s); return s }

// pick draws one address of the family, as the authoritative servers do.
type picker struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newPicker(seed uint64) *picker { return &picker{rng: rand.New(rand.NewPCG(seed, seed))} }

func (p *picker) pick(from []string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return from[p.rng.IntN(len(from))]
}

// fakeAuthoritative serves beacon like the real authoritative servers: every
// query gets one address of the pool, drawn at random, and nothing is cached.
// rcode overrides the answer when set.
func fakeAuthoritative(t *testing.T, seed uint64, rcode dnsmessage.RCode) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	p := newPicker(seed)
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			var q dnsmessage.Message
			if q.Unpack(buf[:n]) != nil || len(q.Questions) != 1 {
				continue
			}
			resp := dnsmessage.Message{
				Header:    dnsmessage.Header{ID: q.ID, Response: true, Authoritative: true, RCode: rcode},
				Questions: q.Questions,
			}
			question := q.Questions[0]
			if rcode == dnsmessage.RCodeSuccess && question.Name.String() == beacon+"." {
				hdr := dnsmessage.ResourceHeader{Name: question.Name, Class: dnsmessage.ClassINET, TTL: 60}
				switch question.Type {
				case dnsmessage.TypeA:
					hdr.Type = dnsmessage.TypeA
					resp.Answers = append(resp.Answers, dnsmessage.Resource{Header: hdr,
						Body: &dnsmessage.AResource{A: netip.MustParseAddr(p.pick(poolV4)).As4()}})
				case dnsmessage.TypeAAAA:
					hdr.Type = dnsmessage.TypeAAAA
					resp.Answers = append(resp.Answers, dnsmessage.Resource{Header: hdr,
						Body: &dnsmessage.AAAAResource{AAAA: netip.MustParseAddr(p.pick(poolV6)).As16()}})
				}
			}
			out, err := resp.Pack()
			if err != nil {
				continue
			}
			pc.WriteTo(out, addr)
		}
	}()
	return pc.LocalAddr().String()
}

// fakeCachingDoH behaves like a public DoH resolver in front of the beacon: the
// first query of a name and type draws one pool address, and every query
// within the 60s TTL gets that cached answer back.
func fakeCachingDoH(t *testing.T, seed uint64) string {
	t.Helper()
	p := newPicker(seed)
	var mu sync.Mutex
	cache := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, typ := r.URL.Query().Get("name"), r.URL.Query().Get("type")
		from, code := poolV4, 1
		if typ == "AAAA" {
			from, code = poolV6, 28
		}
		mu.Lock()
		ip, ok := cache[name+"/"+typ]
		if !ok {
			ip = p.pick(from)
			cache[name+"/"+typ] = ip
		}
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"Answer": []map[string]any{{"type": code, "TTL": 60, "data": ip}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func collector(servers ...string) *Collector {
	return &Collector{
		Nameservers: func(context.Context, string) ([]string, error) { return servers, nil },
		Rounds:      DefaultRounds,
		Timeout:     time.Second,
	}
}

// The bug behind issue #1: the old collection asked DoH four times per beacon.
// A caching resolver answers all four from cache, so half the pool is never
// seen, and never blocked.
func TestDoHSamplingMissesHalfThePool(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		r := doh.New([]doh.Provider{{Name: "caching", URL: fakeCachingDoH(t, seed)}})
		seen := map[string]bool{}
		for range 4 { // the old ui.beaconSamples
			ips, err := r.ResolveAllFresh(beacon)
			if err != nil {
				t.Fatal(err)
			}
			for _, ip := range ips {
				seen[ip] = true
			}
		}
		if len(seen) != len(pool)/2 {
			t.Fatalf("seed %d: DoH sampling saw %d of %d addresses, want exactly half", seed, len(seen), len(pool))
		}
	}
}

// The fix: authoritative servers draw a fresh address per query, so Collect
// sees the whole pool, for both families.
func TestCollectFindsWholePool(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		got, err := collector(fakeAuthoritative(t, seed, dnsmessage.RCodeSuccess)).Collect(context.Background(), beacon)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, pool) {
			t.Fatalf("seed %d: got %v, want %v", seed, got, pool)
		}
	}
}

// Queries are spread over every nameserver, and one dead server does not hide
// addresses the others hand out.
func TestCollectSurvivesDeadNameserver(t *testing.T) {
	dead, err := net.ListenPacket("udp", "127.0.0.1:0") // never answers
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close()
	c := collector(dead.LocalAddr().String(), fakeAuthoritative(t, 7, dnsmessage.RCodeSuccess))
	c.Rounds = 2 * DefaultRounds // half the queries go to the dead server
	c.Timeout = 200 * time.Millisecond
	got, err := c.Collect(context.Background(), beacon)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, pool) {
		t.Fatalf("got %v, want %v", got, pool)
	}
}

func TestCollectFailsWhenNoServerAnswers(t *testing.T) {
	dead, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close()
	c := collector(dead.LocalAddr().String())
	c.Rounds = 2
	c.Timeout = 100 * time.Millisecond
	if got, err := c.Collect(context.Background(), beacon); err == nil {
		t.Fatalf("got %v, want an error", got)
	}
}

func TestCollectReportsErrorRcode(t *testing.T) {
	c := collector(fakeAuthoritative(t, 1, dnsmessage.RCodeNameError))
	if got, err := c.Collect(context.Background(), beacon); err == nil {
		t.Fatalf("got %v, want an error for NXDOMAIN", got)
	}
}

func TestCollectNoNameservers(t *testing.T) {
	c := collector()
	if _, err := c.Collect(context.Background(), beacon); err == nil {
		t.Fatal("want an error without nameservers")
	}
	c.Nameservers = func(context.Context, string) ([]string, error) { return nil, fmt.Errorf("boom") }
	if _, err := c.Collect(context.Background(), beacon); err == nil {
		t.Fatal("want the nameserver lookup error")
	}
}
