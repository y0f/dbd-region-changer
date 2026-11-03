package doh

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveSkipsCNAMEInChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Status":0,"Answer":[{"name":"a.example.","type":5,"TTL":60,"data":"x.example."},{"name":"x.example.","type":1,"TTL":60,"data":"1.2.3.4"}]}`)
	}))
	defer srv.Close()
	ip, err := New([]Provider{{"fake", srv.URL}}).Resolve("a.example")
	if err != nil || ip != "1.2.3.4" {
		t.Fatalf("ip=%q err=%v, want 1.2.3.4", ip, err)
	}
}

func TestResolveCNAMEOnlyIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Status":0,"Answer":[{"type":5,"TTL":60,"data":"x.example."}]}`)
	}))
	defer srv.Close()
	if ip, err := New([]Provider{{"fake", srv.URL}}).Resolve("a.example"); err == nil {
		t.Fatalf("got %q, want an error when no A record is present", ip)
	}
}

func TestCacheHonorsRecordTTL(t *testing.T) {
	var mu sync.Mutex
	answer := "1.1.1.1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		io.WriteString(w, `{"Status":0,"Answer":[{"type":1,"TTL":10,"data":"`+answer+`"}]}`)
	}))
	defer srv.Close()
	r := New([]Provider{{"fake", srv.URL}})
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }

	if ip, err := r.Resolve("beacon"); err != nil || ip != "1.1.1.1" {
		t.Fatalf("ip=%q err=%v", ip, err)
	}
	mu.Lock()
	answer = "2.2.2.2"
	mu.Unlock()

	clock = clock.Add(5 * time.Second)
	if ip, _ := r.Resolve("beacon"); ip != "1.1.1.1" {
		t.Errorf("within record TTL: got %q, want cached 1.1.1.1", ip)
	}
	clock = clock.Add(10 * time.Second)
	if ip, _ := r.Resolve("beacon"); ip != "2.2.2.2" {
		t.Errorf("after record TTL: got %q, want rotated 2.2.2.2", ip)
	}
}

func TestCacheTTLIsCapped(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		io.WriteString(w, `{"Status":0,"Answer":[{"type":1,"TTL":86400,"data":"1.1.1.1"}]}`)
	}))
	defer srv.Close()
	r := New([]Provider{{"fake", srv.URL}})
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }
	r.Resolve("h")
	clock = clock.Add(maxCacheTTL + time.Second)
	r.Resolve("h")
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits=%d, want 2 (cache capped at %v)", got, maxCacheTTL)
	}
}

func TestResolveAllReturnsBothFamilies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "A":
			io.WriteString(w, `{"Status":0,"Answer":[{"type":1,"TTL":60,"data":"3.254.137.3"}]}`)
		case "AAAA":
			io.WriteString(w, `{"Status":0,"Answer":[{"type":28,"TTL":60,"data":"2a05:d018:17a5:2400:9a3:5f7f:c637:11ce"}]}`)
		}
	}))
	defer srv.Close()
	ips, err := New([]Provider{{"fake", srv.URL}}).ResolveAll("gamelift-ping.eu-west-1.api.aws")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"3.254.137.3", "2a05:d018:17a5:2400:9a3:5f7f:c637:11ce"}
	if !slices.Equal(ips, want) {
		t.Errorf("ips=%v, want %v", ips, want)
	}
}

func TestResolveAllIPv4Only(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "A" {
			io.WriteString(w, `{"Status":0,"Answer":[{"type":1,"TTL":60,"data":"1.2.3.4"}]}`)
			return
		}
		io.WriteString(w, `{"Status":0}`)
	}))
	defer srv.Close()
	ips, err := New([]Provider{{"fake", srv.URL}}).ResolveAll("v4only.example")
	if err != nil || !slices.Equal(ips, []string{"1.2.3.4"}) {
		t.Fatalf("ips=%v err=%v", ips, err)
	}
}

// TestDefaultProvidersLive checks every configured provider really speaks the
// JSON API (Quad9's /dns-query does not, it answers 400). Opt-in: it needs the
// network, so set DBDREGION_LIVE=1.
func TestDefaultProvidersLive(t *testing.T) {
	if os.Getenv("DBDREGION_LIVE") == "" {
		t.Skip("set DBDREGION_LIVE=1 to run live DNS checks")
	}
	for _, p := range DefaultProviders() {
		ip, err := New([]Provider{p}).Resolve("gamelift.us-east-1.amazonaws.com")
		if err != nil {
			t.Errorf("%s: %v", p.Name, err)
			continue
		}
		t.Logf("%s: %s", p.Name, ip)
	}
}

func TestAnswerWithoutTypeOrTTL(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		io.WriteString(w, `{"Answer":[{"data":"x.example."},{"data":"::1"},{"data":"1.2.3.4"}]}`)
	}))
	defer srv.Close()
	r := New([]Provider{{"fake", srv.URL}})
	for range 2 {
		if ip, err := r.Resolve("h"); err != nil || ip != "1.2.3.4" {
			t.Fatalf("ip=%q err=%v, want 1.2.3.4", ip, err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server hits=%d, want 1 (no TTL means cache for the cap)", got)
	}
}
