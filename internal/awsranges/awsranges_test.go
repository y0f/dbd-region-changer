package awsranges

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRegionCodes(t *testing.T) {
	body := `{"prefixes":[
		{"ip_prefix":"1.0.0.0/8","region":"us-east-1","service":"EC2"},
		{"ip_prefix":"2.0.0.0/8","region":"us-east-1","service":"AMAZON"},
		{"ip_prefix":"3.0.0.0/8","region":"GLOBAL","service":"ROUTE53"},
		{"ip_prefix":"4.0.0.0/8","region":"eu-central-1","service":"EC2"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	old := ipRangesURL
	ipRangesURL = srv.URL
	defer func() { ipRangesURL = old }()

	got, err := RegionCodes(srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"eu-central-1", "us-east-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v (deduped, GLOBAL dropped, sorted)", got, want)
	}
}

func TestRegionCodesNetworkError(t *testing.T) {
	old := ipRangesURL
	ipRangesURL = "http://127.0.0.1:0/nope"
	defer func() { ipRangesURL = old }()
	if _, err := RegionCodes(&http.Client{}); err == nil {
		t.Fatal("expected error")
	}
}
