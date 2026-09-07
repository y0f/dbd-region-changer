package region

import "testing"

func containsCode(servers []Server, code string) bool {
	for _, s := range servers {
		if s.Code == code {
			return true
		}
	}
	return false
}

func TestBuiltinIncludesHongKongAndAllActive(t *testing.T) {
	got := Builtin()
	for _, want := range []string{
		"ap-east-1", "ap-northeast-1", "ap-northeast-2", "ap-south-1",
		"ap-southeast-1", "ap-southeast-2", "ca-central-1", "eu-central-1",
		"eu-west-1", "eu-west-2", "sa-east-1", "us-east-1", "us-east-2",
		"us-west-1", "us-west-2",
	} {
		if !containsCode(got, want) {
			t.Errorf("builtin missing %s", want)
		}
	}
}

func TestServerHostnamesAndLabel(t *testing.T) {
	r := Server{Code: "us-east-1", Pretty: "US East (N. Virginia)"}
	if r.Endpoint() != "gamelift.us-east-1.amazonaws.com" {
		t.Errorf("endpoint=%q", r.Endpoint())
	}
	if r.Dualstack() != "gamelift-ping.us-east-1.api.aws" {
		t.Errorf("dualstack=%q", r.Dualstack())
	}
	if r.Label() != "US East (N. Virginia) (us-east-1)" {
		t.Errorf("label=%q", r.Label())
	}
}
