// Package version derives PEP-440-ish display strings and SemVer strings from a structured version.
package version

import (
	"fmt"
	"regexp"
	"strconv"
)

// Stage is the dev/alpha/beta/rc/final release qualifier.
type Stage int

const (
	Dev Stage = iota
	Alpha
	Beta
	RC
	Final
)

type Info struct {
	Major, Minor, Patch int
	Stage               Stage
	Num                 int
}

// Current is the app version. Release builds override it from the git tag via
// -ldflags "-X github.com/y0f/dbd-region-changer/internal/version.tag=<tag>"
// (see scripts/build-*.sh/ps1); the literal is the fallback for plain builds.
var Current = Info{Major: 1, Minor: 0, Patch: 2, Stage: Final, Num: 0}

// tag is set at link time; an unparsable value is ignored.
var tag string

func init() {
	if v, err := Parse(tag); err == nil {
		Current = v
	}
}

var stageNames = map[string]Stage{"dev": Dev, "a": Alpha, "alpha": Alpha, "b": Beta, "beta": Beta, "rc": RC}

// versionRe matches an optional "v", MAJOR.MINOR[.PATCH] and an optional stage
// in either SemVer or PEP-440 form, so every release tag the updater accepts
// and every string Semver or Display produces parses: 1.2.3, v1.2.3,
// 1.2.0-rc.4, 1.2rc4, 1.2.0.rc1, 1.2.0-b2, 1.2.dev0. A missing patch or stage
// number is 0. Each number is at most 9 digits so it always fits an int (even
// 32-bit); the updater and the build-script tag gates use the same bound.
var versionRe = regexp.MustCompile(`^v?(\d{1,9})\.(\d{1,9})(?:\.(\d{1,9}))?(?:[.-]?(dev|alpha|beta|a|b|rc)\.?(\d{1,9})?)?$`)

// Parse reads a version or release tag: the SemVer form produced by Semver,
// the PEP-440 form produced by Display, or any release tag form the updater
// accepts, each with an optional leading "v".
func Parse(s string) (Info, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return Info{}, fmt.Errorf("version %q: want MAJOR.MINOR[.PATCH][stageN]", s)
	}
	var v [4]int
	for i, n := range []string{m[1], m[2], m[3], m[5]} {
		if n == "" {
			continue
		}
		x, err := strconv.Atoi(n)
		if err != nil {
			return Info{}, fmt.Errorf("version %q: bad number %q", s, n)
		}
		v[i] = x
	}
	info := Info{Major: v[0], Minor: v[1], Patch: v[2], Stage: Final}
	if m[4] != "" {
		info.Stage, info.Num = stageNames[m[4]], v[3]
	}
	return info, nil
}

// Display returns the human-facing version: 2 parts when patch==0 else 3, plus a stage suffix (none for final).
func (i Info) Display() string {
	parts := 3
	if i.Patch == 0 {
		parts = 2
	}
	var v string
	if parts == 2 {
		v = fmt.Sprintf("%d.%d", i.Major, i.Minor)
	} else {
		v = fmt.Sprintf("%d.%d.%d", i.Major, i.Minor, i.Patch)
	}
	switch i.Stage {
	case Dev:
		v += ".dev" + strconv.Itoa(i.Num)
	case Final:
	default:
		m := map[Stage]string{Alpha: "a", Beta: "b", RC: "rc"}
		v += m[i.Stage] + strconv.Itoa(i.Num)
	}
	return v
}

// Semver returns MAJOR.MINOR.PATCH plus a -dev.N/-alpha.N/-beta.N/-rc.N pre-release for non-final stages.
func (i Info) Semver() string {
	v := fmt.Sprintf("%d.%d.%d", i.Major, i.Minor, i.Patch)
	switch i.Stage {
	case Dev:
		v += fmt.Sprintf("-dev.%d", i.Num)
	case Alpha:
		v += fmt.Sprintf("-alpha.%d", i.Num)
	case Beta:
		v += fmt.Sprintf("-beta.%d", i.Num)
	case RC:
		v += fmt.Sprintf("-rc.%d", i.Num)
	case Final:
	}
	return v
}
