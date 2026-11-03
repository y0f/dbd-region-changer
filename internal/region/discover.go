package region

import "sort"

// probeAttempts is how often a beacon is probed before its region counts as
// down, so a single lost UDP datagram does not drop a live region.
const probeAttempts = 2

// Discover builds the region list from AWS region codes: every built-in region,
// plus each other code whose GameLift beacon answers probe. Built-in regions
// are kept without probing, so a lost probe or a leftover firewall block from
// an earlier session can never hide them (or exclude them from the block).
// The result is sorted by code.
func Discover(codes []string, probe func(Server) bool) []Server {
	known := make(map[string]bool, len(builtin))
	for _, s := range builtin {
		known[s.Code] = true
	}
	var toProbe []string
	for _, c := range codes {
		if !known[c] {
			known[c] = true // also dedupes codes
			toProbe = append(toProbe, c)
		}
	}

	type res struct {
		s  Server
		ok bool
	}
	ch := make(chan res, len(toProbe))
	for _, c := range toProbe {
		go func(c string) {
			s := Server{Code: c, Pretty: PrettyName(c)}
			ok := false
			for i := 0; i < probeAttempts && !ok; i++ {
				ok = probe(s)
			}
			ch <- res{s, ok}
		}(c)
	}
	out := Builtin()
	for range toProbe {
		if r := <-ch; r.ok {
			out = append(out, r.s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

var prettyNames = map[string]string{
	"af-south-1":     "Africa (Cape Town)",
	"ap-east-1":      "Asia Pacific (Hong Kong)",
	"ap-east-2":      "Asia Pacific (Taipei)",
	"ap-northeast-1": "Asia Pacific (Tokyo)",
	"ap-northeast-2": "Asia Pacific (Seoul)",
	"ap-northeast-3": "Asia Pacific (Osaka)",
	"ap-south-1":     "Asia Pacific (Mumbai)",
	"ap-south-2":     "Asia Pacific (Hyderabad)",
	"ap-southeast-1": "Asia Pacific (Singapore)",
	"ap-southeast-2": "Asia Pacific (Sydney)",
	"ap-southeast-3": "Asia Pacific (Jakarta)",
	"ap-southeast-4": "Asia Pacific (Melbourne)",
	"ap-southeast-5": "Asia Pacific (Malaysia)",
	"ap-southeast-6": "Asia Pacific (New Zealand)",
	"ap-southeast-7": "Asia Pacific (Thailand)",
	"ca-central-1":   "Canada (Central)",
	"ca-west-1":      "Canada West (Calgary)",
	"eu-central-1":   "Europe (Frankfurt)",
	"eu-central-2":   "Europe (Zurich)",
	"eu-north-1":     "Europe (Stockholm)",
	"eu-south-1":     "Europe (Milan)",
	"eu-south-2":     "Europe (Spain)",
	"eu-west-1":      "Europe (Ireland)",
	"eu-west-2":      "Europe (London)",
	"eu-west-3":      "Europe (Paris)",
	"il-central-1":   "Israel (Tel Aviv)",
	"me-central-1":   "Middle East (UAE)",
	"me-south-1":     "Middle East (Bahrain)",
	"mx-central-1":   "Mexico (Central)",
	"sa-east-1":      "South America (Sao Paulo)",
	"us-east-1":      "US East (N. Virginia)",
	"us-east-2":      "US East (Ohio)",
	"us-west-1":      "US West (N. California)",
	"us-west-2":      "US West (Oregon)",
}

// PrettyName returns a label for a region code, falling back to the raw code.
func PrettyName(code string) string {
	if n, ok := prettyNames[code]; ok {
		return n
	}
	return code
}
