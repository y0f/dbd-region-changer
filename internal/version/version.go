// Package version derives PEP-440-ish display strings and SemVer strings from a structured version.
package version

import (
	"fmt"
	"strconv"
	"strings"
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
var Current = Info{Major: 1, Minor: 0, Patch: 1, Stage: Final, Num: 0}

// tag is set at link time; an unparsable value is ignored.
var tag string

func init() {
	if v, err := Parse(tag); err == nil {
		Current = v
	}
}

var stageNames = map[string]Stage{"dev": Dev, "alpha": Alpha, "beta": Beta, "rc": RC}

// Parse reads a SemVer string as produced by Semver, with an optional leading
// "v": "1.2.3", "v1.2.3" or "1.2.3-rc.4".
func Parse(s string) (Info, error) {
	core, pre, hasPre := strings.Cut(strings.TrimPrefix(s, "v"), "-")
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return Info{}, fmt.Errorf("version %q: want MAJOR.MINOR.PATCH", s)
	}
	var v [3]int
	for i, n := range nums {
		x, err := strconv.Atoi(n)
		if err != nil || x < 0 {
			return Info{}, fmt.Errorf("version %q: bad number %q", s, n)
		}
		v[i] = x
	}
	info := Info{Major: v[0], Minor: v[1], Patch: v[2], Stage: Final}
	if hasPre {
		name, num, _ := strings.Cut(pre, ".")
		st, ok := stageNames[name]
		n, err := strconv.Atoi(num)
		if !ok || err != nil || n < 0 {
			return Info{}, fmt.Errorf("version %q: bad pre-release %q", s, pre)
		}
		info.Stage, info.Num = st, n
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
