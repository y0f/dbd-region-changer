// Package awsranges reads AWS's published ip-ranges.json to enumerate every
// region AWS runs in (so region discovery is not limited to a hardcoded subset).
package awsranges

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

var ipRangesURL = "https://ip-ranges.amazonaws.com/ip-ranges.json"

type rawRanges struct {
	Prefixes []struct {
		IPPrefix string `json:"ip_prefix"`
		Region   string `json:"region"`
		Service  string `json:"service"`
	} `json:"prefixes"`
}

// RegionCodes fetches ip-ranges.json and returns every non-GLOBAL region code, sorted and deduped.
func RegionCodes(client *http.Client) ([]string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Get(ipRangesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw rawRanges
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var codes []string
	for _, p := range raw.Prefixes {
		if p.Region == "" || p.Region == "GLOBAL" || seen[p.Region] {
			continue
		}
		seen[p.Region] = true
		codes = append(codes, p.Region)
	}
	sort.Strings(codes)
	return codes, nil
}
