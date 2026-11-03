// Package awsranges reads AWS's published ip-ranges.json to enumerate every
// region AWS runs in (so region discovery is not limited to a hardcoded subset).
//
// The file is several MB, so the derived region codes are cached on disk with
// the response's ETag/Last-Modified, and later launches send a conditional
// request that is answered with 304 Not Modified while nothing changed.
package awsranges

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ipRangesURL = "https://ip-ranges.amazonaws.com/ip-ranges.json"

// cachePath returns the file the region codes are cached in ("" disables the cache).
var cachePath = func() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dbd-region-changer", "ip-ranges-regions.json")
}

// otherPartitions are region prefixes outside the commercial AWS partition
// (China, GovCloud, European Sovereign Cloud). Game clients cannot reach
// GameLift there, so probing them only costs failed lookups.
var otherPartitions = []string{"cn-", "us-gov-", "us-iso", "eusc-"}

type rawRanges struct {
	Prefixes []struct {
		Region string `json:"region"`
	} `json:"prefixes"`
}

type cached struct {
	ETag         string   `json:"etag,omitempty"`
	LastModified string   `json:"last_modified,omitempty"`
	Codes        []string `json:"codes"`
}

// RegionCodes returns every commercial-partition region code in ip-ranges.json,
// sorted and deduped. When the download fails but an earlier result is cached,
// the cached codes are returned instead of an error.
func RegionCodes(client *http.Client) ([]string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	path := cachePath()
	prev := readCache(path)

	codes, fresh, err := fetch(client, prev)
	if err != nil {
		if prev != nil && len(prev.Codes) > 0 {
			return prev.Codes, nil
		}
		return nil, err
	}
	if fresh != nil {
		writeCache(path, fresh)
	}
	return codes, nil
}

// fetch performs a conditional GET. On 304 it returns prev's codes and a nil
// cache entry; on 200 it returns the parsed codes and the entry to store.
func fetch(client *http.Client, prev *cached) ([]string, *cached, error) {
	req, err := http.NewRequest(http.MethodGet, ipRangesURL, nil)
	if err != nil {
		return nil, nil, err
	}
	if prev != nil && len(prev.Codes) > 0 {
		if prev.ETag != "" {
			req.Header.Set("If-None-Match", prev.ETag)
		}
		if prev.LastModified != "" {
			req.Header.Set("If-Modified-Since", prev.LastModified)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		if prev != nil && len(prev.Codes) > 0 {
			return prev.Codes, nil, nil
		}
		return nil, nil, fmt.Errorf("ip-ranges: unexpected 304 without a cached copy")
	default:
		io.Copy(io.Discard, resp.Body)
		return nil, nil, fmt.Errorf("ip-ranges: HTTP %d", resp.StatusCode)
	}

	var raw rawRanges
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	var codes []string
	for _, p := range raw.Prefixes {
		if p.Region == "" || p.Region == "GLOBAL" || seen[p.Region] || otherPartition(p.Region) {
			continue
		}
		seen[p.Region] = true
		codes = append(codes, p.Region)
	}
	if len(codes) == 0 {
		return nil, nil, fmt.Errorf("ip-ranges: no regions in response")
	}
	sort.Strings(codes)
	return codes, &cached{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Codes:        codes,
	}, nil
}

func otherPartition(code string) bool {
	for _, p := range otherPartitions {
		if strings.HasPrefix(code, p) {
			return true
		}
	}
	return false
}

func readCache(path string) *cached {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c cached
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	return &c
}

// writeCache stores c best effort; a failure only costs a full download next time.
func writeCache(path string, c *cached) {
	if path == "" {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
