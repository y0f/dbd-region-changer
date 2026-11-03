// Package region provides the GameLift region catalog. Membership comes from
// the AWS region codes in ip-ranges.json, kept when their GameLift ping beacon
// answers, plus the curated built-in list, which is always kept so a lost probe
// or a leftover firewall block never hides a core region. Pretty names come
// from a static map.
package region

import "github.com/y0f/dbd-region-changer/internal/config"

type Server struct {
	Code   string
	Pretty string
}

// Endpoint returns gamelift.{code}.amazonaws.com.
func (s Server) Endpoint() string {
	e, _ := config.BuildGameliftHosts(s.Code)
	return e
}

// Dualstack returns gamelift-ping.{code}.api.aws.
func (s Server) Dualstack() string {
	_, d := config.BuildGameliftHosts(s.Code)
	return d
}

// Label is the dropdown string "{pretty} ({code})".
func (s Server) Label() string {
	return s.Pretty + " (" + s.Code + ")"
}
