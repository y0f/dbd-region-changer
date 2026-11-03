// Package config holds endpoints, DNS providers, and the GameLift hostname builders.
package config

import "fmt"

const (
	GithubRepoURL       = "https://github.com/y0f/dbd-region-changer"
	UpdateURL           = "https://github.com/y0f/dbd-region-changer/releases/latest"
	APILatestReleaseURL = "https://api.github.com/repos/y0f/dbd-region-changer/releases/latest"
	RepoOwner           = "y0f"
	RepoName            = "dbd-region-changer"

	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)

type DNSProvider struct {
	Name string
	URL  string
}

// DNSProviders are tried in order with failover.
var DNSProviders = []DNSProvider{
	{Name: "cloudflare", URL: "https://cloudflare-dns.com/dns-query"},
	// Quad9 is not usable here: its /dns-query only speaks RFC 8484 wire format.
	{Name: "google", URL: "https://dns.google/resolve"},
}

// BuildGameliftHosts returns the .amazonaws.com endpoint and .api.aws dualstack hostnames for a region.
func BuildGameliftHosts(region string) (endpoint, dualstack string) {
	endpoint = fmt.Sprintf("gamelift.%s.amazonaws.com", region)
	dualstack = fmt.Sprintf("gamelift-ping.%s.api.aws", region)
	return endpoint, dualstack
}
