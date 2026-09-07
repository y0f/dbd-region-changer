// Package elevate handles privilege for hosts writes: Windows is already elevated (manifest);
// Linux re-execs a headless helper subcommand via pkexec.
package elevate

import (
	"encoding/json"
	"os"

	"github.com/y0f/dbd-region-changer/internal/hostsfile"
)

const (
	OpWrite = "write"
	OpClear = "clear"
)

// Payload is the JSON contract passed to the elevated helper over stdin.
type Payload struct {
	Op              string            `json:"op"`
	RemoveHostnames []string          `json:"remove_hostnames"`
	Entries         []hostsfile.Entry `json:"entries"`
}

// applyPayload performs the remove-then-write described by p (Windows direct path and Linux helper).
func applyPayload(p Payload) error {
	h := hostsfile.New()
	if err := h.RemoveAll(p.RemoveHostnames); err != nil {
		return err
	}
	if p.Op == OpWrite {
		return h.Apply(p.Entries)
	}
	return nil
}

// HandleHelperSubcommand runs the elevated hosts write if argv requests it (Payload from stdin); handled=false means launch the GUI.
func HandleHelperSubcommand(args []string) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}
	var op string
	switch args[0] {
	case "--write-hosts":
		op = OpWrite
	case "--clear-hosts":
		op = OpClear
	default:
		return false, nil
	}
	var p Payload
	if derr := json.NewDecoder(os.Stdin).Decode(&p); derr != nil {
		return true, derr
	}
	p.Op = op
	return true, applyPayload(p)
}
