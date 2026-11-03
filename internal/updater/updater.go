// Package updater checks GitHub for a newer release, normalizing the tag to SemVer before comparison.
package updater

import (
	"encoding/json"
	"errors"
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

// unknownRemote is shown as the latest version when the release could not be fetched.
const unknownRemote = "unknown"

type Result struct {
	Local  string
	Remote string // latest release tag, or "unknown" if it could not be fetched
	Code   int
	Err    error // why the check failed; nil unless Code is StatusError
}

// tagRe matches an optional "v", MAJOR.MINOR[.PATCH], and an optional PEP-440
// stage with or without a separator: 1.2.0, v1.2.0, 1.2rc4, 1.2.0.rc1, 1.2.0-b2, 1.2.dev0.
// Numbers are capped at 9 digits, the bound version.Parse uses to fit an int.
var tagRe = regexp.MustCompile(`^v?(\d{1,9})\.(\d{1,9})(?:\.(\d{1,9}))?(?:[.-]?(dev|a|b|rc)(\d{1,9})?)?$`)

func normalizeTag(tag string) (string, error) {
	m := tagRe.FindStringSubmatch(tag)
	if m == nil {
		return "", fmt.Errorf("tag %q is not MAJOR.MINOR[.PATCH][stageN]", tag)
	}
	patch := m[3]
	if patch == "" {
		patch = "0"
	}
	base := fmt.Sprintf("%s.%s.%s", m[1], m[2], patch)
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
		res.Code, res.Err = StatusError, err
		return res
	}
	remote, err := semver.NewVersion(normalized)
	if err != nil {
		res.Code, res.Err = StatusError, err
		return res
	}
	local, err := semver.NewVersion(localSemver)
	if err != nil {
		res.Code, res.Err = StatusError, err
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

// CheckLatest fetches the latest release tag from url and compares it; any
// failure yields StatusError with Remote "unknown" and the reason in Err.
func CheckLatest(client *http.Client, url, localSemver string) Result {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	fail := func(err error) Result {
		return Result{Local: localSemver, Remote: unknownRemote, Code: StatusError, Err: err}
	}
	resp, err := client.Get(url)
	if err != nil {
		return fail(fmt.Errorf("cannot reach GitHub: %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fail(errors.New("GitHub API rate limit reached, try again later"))
		}
		return fail(fmt.Errorf("GitHub API returned %s", resp.Status))
	}
	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fail(fmt.Errorf("unreadable GitHub response: %w", err))
	}
	return check(rel.TagName, localSemver)
}

func Check(client *http.Client) Result {
	return CheckLatest(client, config.APILatestReleaseURL, version.Current.Semver())
}
