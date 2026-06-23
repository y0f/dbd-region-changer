package region

// builtin is the curated set of regions DBD runs GameLift fleets in; the sole source of membership.
var builtin = []Server{
	{Code: "ap-east-1", Pretty: "Asia Pacific (Hong Kong)"},
	{Code: "ap-northeast-1", Pretty: "Asia Pacific (Tokyo)"},
	{Code: "ap-northeast-2", Pretty: "Asia Pacific (Seoul)"},
	{Code: "ap-south-1", Pretty: "Asia Pacific (Mumbai)"},
	{Code: "ap-southeast-1", Pretty: "Asia Pacific (Singapore)"},
	{Code: "ap-southeast-2", Pretty: "Asia Pacific (Sydney)"},
	{Code: "ca-central-1", Pretty: "Canada (Central)"},
	{Code: "eu-central-1", Pretty: "Europe (Frankfurt)"},
	{Code: "eu-west-1", Pretty: "Europe (Ireland)"},
	{Code: "eu-west-2", Pretty: "Europe (London)"},
	{Code: "sa-east-1", Pretty: "South America (Sao Paulo)"},
	{Code: "us-east-1", Pretty: "US East (N. Virginia)"},
	{Code: "us-east-2", Pretty: "US East (Ohio)"},
	{Code: "us-west-1", Pretty: "US West (N. California)"},
	{Code: "us-west-2", Pretty: "US West (Oregon)"},
}

// Builtin returns a copy of the curated region list.
func Builtin() []Server {
	out := make([]Server, len(builtin))
	copy(out, builtin)
	return out
}
