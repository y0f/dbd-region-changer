package updater

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/y0f/dbd-region-changer/internal/version"
)

func TestNormalizeTagForms(t *testing.T) {
	cases := map[string]string{
		"1.1.0":     "1.1.0",
		"v1.1.0":    "1.1.0",
		"1.1.0rc1":  "1.1.0-rc.1",
		"1.1.0a1":   "1.1.0-alpha.1",
		"1.1.0.b2":  "1.1.0-beta.2",
		"1.1.0-rc3": "1.1.0-rc.3",
		"1.2rc4":    "1.2.0-rc.4",
		"1.2.dev0":  "1.2.0-dev.0",
		"1.2":       "1.2.0",
	}
	for tag, want := range cases {
		got, err := normalizeTag(tag)
		if err != nil || got != want {
			t.Errorf("normalizeTag(%q) = %q, %v; want %q", tag, got, err, want)
		}
	}
	for _, bad := range []string{"", "1", "garbage", "1.2.3.4", "1.2.0rc1x", "1.2.99999999999999999999", "1.2.0rc1234567890"} {
		if got, err := normalizeTag(bad); err == nil {
			t.Errorf("normalizeTag(%q) = %q, want error", bad, got)
		}
	}
}

// A release tagged with the app's own Display() string must compare correctly.
func TestDisplayStringIsAValidTag(t *testing.T) {
	for _, v := range []version.Info{
		{Major: 1, Minor: 2, Patch: 0, Stage: version.RC, Num: 4},
		{Major: 1, Minor: 2, Patch: 3, Stage: version.Alpha, Num: 1},
		{Major: 1, Minor: 2, Patch: 0, Stage: version.Dev, Num: 0},
		{Major: 1, Minor: 2, Patch: 0, Stage: version.Final},
	} {
		if got := check(v.Display(), v.Semver()).Code; got != StatusLatest {
			t.Errorf("check(%q, %q) = %d, want StatusLatest", v.Display(), v.Semver(), got)
		}
	}
}

func TestCheckLatestErrorsCarryReason(t *testing.T) {
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer limited.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer broken.Close()

	cases := []struct {
		name, url, wantErr string
	}{
		{"rate limited", limited.URL, "rate limit"},
		{"bad json", broken.URL, "unreadable"},
		{"unreachable", "http://127.0.0.1:1/x", "cannot reach GitHub"},
	}
	for _, c := range cases {
		r := CheckLatest(nil, c.url, "1.0.0")
		if r.Code != StatusError {
			t.Errorf("%s: Code=%d, want StatusError", c.name, r.Code)
		}
		if r.Remote != unknownRemote {
			t.Errorf("%s: Remote=%q, want %q (errors belong in Err)", c.name, r.Remote, unknownRemote)
		}
		if r.Err == nil || !strings.Contains(r.Err.Error(), c.wantErr) {
			t.Errorf("%s: Err=%v, want it to mention %q", c.name, r.Err, c.wantErr)
		}
	}
}

func TestCheckLatestSuccessHasNoErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"tag_name":"1.1.0rc1"}`))
	}))
	defer srv.Close()
	r := CheckLatest(nil, srv.URL, "1.0.0")
	if r.Code != StatusOutdated || r.Err != nil || r.Remote != "1.1.0rc1" {
		t.Errorf("got %+v, want StatusOutdated with Remote 1.1.0rc1 and nil Err", r)
	}
}
