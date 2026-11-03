package updater

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/y0f/dbd-region-changer/internal/version"
)

// The release build scripts refuse tags that version.Parse or the updater
// would reject. Their gates are copied into RE2 here (they only use syntax
// bash ERE, .NET and RE2 read alike) and must agree with both on every sample.
func TestBuildScriptTagGatesMatchParseAndUpdater(t *testing.T) {
	gates := map[string]*regexp.Regexp{
		"../../scripts/build-linux.sh":    regexp.MustCompile(`\[\[ \$tag =~ (\S+) \]\]`),
		"../../scripts/build-windows.ps1": regexp.MustCompile(`\$tag -cmatch '([^']+)'`),
	}
	tags := []string{
		"1.2", "1.2.3", "v1.2.3", "1.2rc4", "1.2.0.rc1", "1.2.0-b2", "1.2.dev0", "1.1.0a1", "1.2.0rc", "10.20.30-rc7",
		// version.Parse accepts these, the updater does not.
		"1.2.0-rc.4", "1.2.0-alpha.1", "1.2.0beta2", "1.2.0rc.1",
		// Numbers up to 9 digits always fit an int; longer ones may overflow.
		"999999999.999999999.999999999rc999999999",
		"1.2.99999999999999999999", "99999999999999999999.2", "1.99999999999999999999",
		"1.2.0rc99999999999999999999", "1.2.0-b1234567890", "1234567890.0.0",
		// Neither accepts these.
		"", "1", "1.2.3.4", "garbage", "1.2.0rc1x", "V1.2", "1.2.0-RC1", "1.2.3-", "\uff11.2.3", "1.\u0662.3",
	}
	for path, find := range gates {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m := find.FindSubmatch(data)
		if m == nil {
			t.Fatalf("%s: tag gate not found", path)
		}
		if strings.Contains(string(m[1]), `\d`) {
			t.Errorf("%s: use [0-9], not \\d, which .NET also matches for non-ASCII digits", path)
		}
		gate := regexp.MustCompile(string(m[1]))
		for _, tag := range tags {
			_, perr := version.Parse(tag)
			_, uerr := normalizeTag(tag)
			want := perr == nil && uerr == nil
			if got := gate.MatchString(tag); got != want {
				t.Errorf("%s accepts %q = %v, want %v (Parse err %v, updater err %v)", path, tag, got, want, perr, uerr)
			}
		}
	}
}
