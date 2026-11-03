package awsranges

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
)

// serve points ipRangesURL at h and the cache at a fresh temp file.
func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	oldURL, oldCache := ipRangesURL, cachePath
	ipRangesURL = srv.URL
	cache := filepath.Join(t.TempDir(), "sub", "cache.json")
	cachePath = func() string { return cache }
	t.Cleanup(func() { ipRangesURL, cachePath = oldURL, oldCache })
	return srv
}

func TestRegionCodes(t *testing.T) {
	body := `{"prefixes":[
		{"ip_prefix":"1.0.0.0/8","region":"us-east-1","service":"EC2"},
		{"ip_prefix":"2.0.0.0/8","region":"us-east-1","service":"AMAZON"},
		{"ip_prefix":"3.0.0.0/8","region":"GLOBAL","service":"ROUTE53"},
		{"ip_prefix":"4.0.0.0/8","region":"eu-central-1","service":"EC2"},
		{"ip_prefix":"5.0.0.0/8","region":"cn-north-1","service":"EC2"},
		{"ip_prefix":"6.0.0.0/8","region":"us-gov-west-1","service":"EC2"},
		{"ip_prefix":"7.0.0.0/8","region":"eusc-de-east-1","service":"EC2"}
	]}`
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) })

	got, err := RegionCodes(srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"eu-central-1", "us-east-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v (deduped, GLOBAL and other partitions dropped, sorted)", got, want)
	}
}

func TestRegionCodesNetworkError(t *testing.T) {
	old, oldCache := ipRangesURL, cachePath
	ipRangesURL = "http://127.0.0.1:0/nope"
	cachePath = func() string { return "" }
	defer func() { ipRangesURL, cachePath = old, oldCache }()
	if _, err := RegionCodes(&http.Client{}); err == nil {
		t.Fatal("expected error")
	}
}

// An error status with a JSON body must be reported, not decoded as "no regions".
func TestRegionCodesErrorStatus(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{}`))
	})
	if got, err := RegionCodes(srv.Client()); err == nil {
		t.Fatalf("503 returned codes=%v err=nil", got)
	}
}

// Later calls revalidate with the ETag and reuse the cached codes on 304; a
// failing fetch falls back to the cache.
func TestRegionCodesConditionalFetchAndCache(t *testing.T) {
	var full int32
	var fail atomic.Bool
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		atomic.AddInt32(&full, 1)
		w.Write([]byte(`{"prefixes":[{"ip_prefix":"1.0.0.0/8","region":"us-east-1","service":"EC2"}]}`))
	})
	want := []string{"us-east-1"}
	for i := 0; i < 3; i++ {
		got, err := RegionCodes(srv.Client())
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("call %d: got %v, %v", i, got, err)
		}
	}
	if full != 1 {
		t.Errorf("full body downloaded %d times for 3 calls, want 1", full)
	}
	fail.Store(true)
	if got, err := RegionCodes(srv.Client()); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback to cache: got %v, %v", got, err)
	}
}
