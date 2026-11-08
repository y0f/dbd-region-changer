package doh

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveAllStartsBothFamiliesConcurrently(t *testing.T) {
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		<-release
		ip := "192.0.2.1"
		if r.URL.Query().Get("type") == "AAAA" {
			ip = "2001:db8::1"
		}
		json.NewEncoder(w).Encode(map[string]any{"Answer": []map[string]string{{"data": ip}}})
	}))
	defer srv.Close()
	r := New([]Provider{{"test", srv.URL}})
	done := make(chan struct{})
	go func() { defer close(done); r.ResolveAll("beacon") }()
	for range 2 {
		select {
		case <-arrived:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatal("DNS families ran serially")
		}
	}
	close(release)
	<-done
}

func TestCancelledResolverDoesNotTryProviders(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewContext(ctx, []Provider{{"test", srv.URL}}).Resolve("beacon")
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("error=%v, calls=%d", err, calls.Load())
	}
}
