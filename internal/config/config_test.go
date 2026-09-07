package config

import "testing"

func TestBuildGameliftHosts(t *testing.T) {
	e, d := BuildGameliftHosts("us-east-1")
	if e != "gamelift.us-east-1.amazonaws.com" {
		t.Errorf("endpoint=%q", e)
	}
	if d != "gamelift-ping.us-east-1.api.aws" {
		t.Errorf("dualstack=%q", d)
	}

	e2, d2 := BuildGameliftHosts("ap-east-1")
	if e2 != "gamelift.ap-east-1.amazonaws.com" {
		t.Errorf("hk endpoint=%q", e2)
	}
	if d2 != "gamelift-ping.ap-east-1.api.aws" {
		t.Errorf("hk dualstack=%q", d2)
	}
}

func TestDNSProviders(t *testing.T) {
	if len(DNSProviders) != 2 {
		t.Fatalf("want 2 providers, got %d", len(DNSProviders))
	}
	if DNSProviders[0].Name != "cloudflare" || DNSProviders[1].Name != "quad9" {
		t.Errorf("provider order wrong: %+v", DNSProviders)
	}
}
