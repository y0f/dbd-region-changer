// Package config holds endpoints, DNS providers, and the GameLift hostname builders.
package config

import "fmt"

const (
	// GameliftDocsURL is scraped for region discovery (best-effort; see internal/region).
	GameliftDocsURL = "https://docs.aws.amazon.com/general/latest/gr/gamelift.html"

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
	{Name: "quad9", URL: "https://dns.quad9.net/dns-query"},
}

// BuildGameliftHosts returns the .amazonaws.com endpoint and .api.aws dualstack hostnames for a region.
func BuildGameliftHosts(region string) (endpoint, dualstack string) {
	endpoint = fmt.Sprintf("gamelift.%s.amazonaws.com", region)
	dualstack = fmt.Sprintf("gamelift-ping.%s.api.aws", region)
	return endpoint, dualstack
}
