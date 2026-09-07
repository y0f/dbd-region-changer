// Package region provides the GameLift region catalog. The built-in list is the source of
// membership; scraping the AWS docs only refreshes pretty names, and any failure falls back to built-in.
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
