//go:build linux

package elevate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
)

func IsElevated() bool { return os.Geteuid() == 0 }

// WriteHosts re-execs the helper subcommand under pkexec (falling back to sudo), payload over stdin;
// the GUI stays unprivileged so only the helper touches the hosts file as root.
func WriteHosts(p Payload) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	arg := "--write-hosts"
	if p.Op == OpClear {
		arg = "--clear-hosts"
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if err := runElevated("pkexec", self, arg, data); err == nil {
		return nil
	}
	return runElevated("sudo", self, arg, data)
}

func runElevated(tool, self, arg string, stdin []byte) error {
	cmd := exec.Command(tool, self, arg)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
