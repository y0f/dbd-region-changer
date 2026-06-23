package updater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckStatuses(t *testing.T) {
	cases := []struct {
		remote, local string
		want          int
	}{
		{"1.3.0", "1.2.0", StatusOutdated},
		{"1.2.0", "1.2.0", StatusLatest},
		{"1.1.0", "1.2.0", StatusFuture},
		{"v1.3.0", "1.2.0", StatusError},        // leading v fails the regex
		{"1.3.0.rc1", "1.2.0", StatusOutdated},  // pre-release normalization
		{"1.2.0", "1.2.0-rc.1", StatusOutdated}, // local rc is older than final
		{"garbage", "1.2.0", StatusError},
	}
	for _, c := range cases {
		if got := check(c.remote, c.local).Code; got != c.want {
			t.Errorf("check(%q,%q)=%d want %d", c.remote, c.local, got, c.want)
		}
	}
}

func TestCheckLatestNetworkErrorIsMinusOne(t *testing.T) {
	r := CheckLatest(&http.Client{}, "http://127.0.0.1:0/nope", "1.2.0")
	if r.Code != StatusError {
		t.Fatalf("want StatusError, got %d", r.Code)
	}
}

func TestCheckLatestParsesTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"tag_name":"1.5.0"}`)
	}))
	defer srv.Close()
	r := CheckLatest(srv.Client(), srv.URL, "1.2.0")
	if r.Code != StatusOutdated {
		t.Fatalf("want StatusOutdated, got %d (remote=%q)", r.Code, r.Remote)
	}
}

func TestCheckLatestNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if r := CheckLatest(srv.Client(), srv.URL, "1.2.0"); r.Code != StatusError {
		t.Fatalf("want StatusError, got %d", r.Code)
	}
}
