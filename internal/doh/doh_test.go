package doh

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestResolveReturnsFirstAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") == "" {
			t.Error("missing name param")
		}
		if r.URL.Query().Get("type") != "A" {
			t.Error("type param != A")
		}
		if r.URL.Query().Get("ct") != "application/dns-json" {
			t.Error("ct param wrong")
		}
		if r.Header.Get("accept") != "application/dns-json" {
			t.Error("accept header wrong")
		}
		io.WriteString(w, `{"Answer":[{"data":"1.2.3.4"}]}`)
	}))
	defer srv.Close()

	r := New([]Provider{{Name: "t", URL: srv.URL}})
	ip, err := r.Resolve("gamelift.us-east-1.amazonaws.com")
	if err != nil || ip != "1.2.3.4" {
		t.Fatalf("ip=%q err=%v", ip, err)
	}
}

func TestFailoverToSecondProvider(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Answer":[{"data":"9.9.9.9"}]}`)
	}))
	defer good.Close()

	r := New([]Provider{{Name: "bad", URL: bad.URL}, {Name: "good", URL: good.URL}})
	ip, err := r.Resolve("example.com")
	if err != nil || ip != "9.9.9.9" {
		t.Fatalf("ip=%q err=%v (expected failover to second provider)", ip, err)
	}
}

func TestCacheHitWithinTTL(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, `{"Answer":[{"data":"5.5.5.5"}]}`)
	}))
	defer srv.Close()

	r := New([]Provider{{Name: "t", URL: srv.URL}})
	for i := 0; i < 3; i++ {
		ip, err := r.Resolve("cached.example.com")
		if err != nil || ip != "5.5.5.5" {
			t.Fatalf("ip=%q err=%v", ip, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 server hit (rest cached), got %d", got)
	}
}

func TestAllProvidersFailReturnsError(t *testing.T) {
	r := New([]Provider{{Name: "x", URL: "http://127.0.0.1:0"}})
	if _, err := r.Resolve("nope.example.com"); err == nil {
		t.Fatal("expected error when all providers fail")
	}
}
