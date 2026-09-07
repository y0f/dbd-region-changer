// Package updater checks GitHub for a newer release, normalizing the tag to SemVer before comparison.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/y0f/dbd-region-changer/internal/config"
	"github.com/y0f/dbd-region-changer/internal/version"
)

const (
	StatusLatest   = 0
	StatusOutdated = 1
	StatusFuture   = 2
	StatusError    = -1
)

type Result struct {
	Local  string
	Remote string
	Code   int
}

// tagRe matches MAJOR.MINOR.PATCH[.dev|a|b|rc N]; a leading "v" does NOT match (yields StatusError).
var tagRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:\.(dev|a|b|rc)(\d+)?)?$`)

func normalizeTag(tag string) (string, error) {
	m := tagRe.FindStringSubmatch(tag)
	if m == nil {
		return "", fmt.Errorf("tag %q is not MAJOR.MINOR.PATCH[.stageN]", tag)
	}
	base := fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3])
	num := m[5]
	if num == "" {
		num = "0"
	}
	switch m[4] {
	case "":
		return base, nil
	case "dev":
		return base + "-dev." + num, nil
	case "a":
		return base + "-alpha." + num, nil
	case "b":
		return base + "-beta." + num, nil
	case "rc":
		return base + "-rc." + num, nil
	}
	return "", fmt.Errorf("unknown stage in tag %q", tag)
}

func check(remoteTag, localSemver string) Result {
	res := Result{Local: localSemver, Remote: remoteTag}
	normalized, err := normalizeTag(remoteTag)
	if err != nil {
		res.Code = StatusError
		return res
	}
	remote, err := semver.NewVersion(normalized)
	if err != nil {
		res.Code = StatusError
		return res
	}
	local, err := semver.NewVersion(localSemver)
	if err != nil {
		res.Code = StatusError
		return res
	}
	switch {
	case local.LessThan(remote):
		res.Code = StatusOutdated
	case local.GreaterThan(remote):
		res.Code = StatusFuture
	default:
		res.Code = StatusLatest
	}
	return res
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

// CheckLatest fetches the latest release tag from url and compares it; any failure yields StatusError.
func CheckLatest(client *http.Client, url, localSemver string) Result {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Get(url)
	if err != nil {
		return Result{Local: localSemver, Remote: err.Error(), Code: StatusError}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{Local: localSemver, Code: StatusError}
	}
	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Result{Local: localSemver, Code: StatusError}
	}
	return check(rel.TagName, localSemver)
}

func Check(client *http.Client) Result {
	return CheckLatest(client, config.APILatestReleaseURL, version.Current.Semver())
}
