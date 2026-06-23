package region

import "sort"

// Discover builds the region list from AWS region codes, keeping only those
// whose GameLift beacon answers probe. Falls back to the built-in list when no
// codes are given or nothing responds.
func Discover(codes []string, probe func(Server) bool) []Server {
	type res struct {
		s  Server
		ok bool
	}
	ch := make(chan res, len(codes))
	for _, c := range codes {
		go func(c string) {
			s := Server{Code: c, Pretty: PrettyName(c)}
			ch <- res{s, probe(s)}
		}(c)
	}
	var live []Server
	for range codes {
		if r := <-ch; r.ok {
			live = append(live, r.s)
		}
	}
	if len(live) == 0 {
		return Builtin()
	}
	sort.Slice(live, func(i, j int) bool { return live[i].Code < live[j].Code })
	return live
}

var prettyNames = map[string]string{
	"af-south-1":     "Africa (Cape Town)",
	"ap-east-1":      "Asia Pacific (Hong Kong)",
	"ap-northeast-1": "Asia Pacific (Tokyo)",
	"ap-northeast-2": "Asia Pacific (Seoul)",
	"ap-northeast-3": "Asia Pacific (Osaka)",
	"ap-south-1":     "Asia Pacific (Mumbai)",
	"ap-south-2":     "Asia Pacific (Hyderabad)",
	"ap-southeast-1": "Asia Pacific (Singapore)",
	"ap-southeast-2": "Asia Pacific (Sydney)",
	"ap-southeast-3": "Asia Pacific (Jakarta)",
	"ap-southeast-4": "Asia Pacific (Melbourne)",
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
