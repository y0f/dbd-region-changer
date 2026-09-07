// Package version derives PEP-440-ish display strings and SemVer strings from a structured version.
package version

import (
	"fmt"
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

// Current is the app version.
var Current = Info{Major: 1, Minor: 0, Patch: 0, Stage: Final, Num: 0}

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
