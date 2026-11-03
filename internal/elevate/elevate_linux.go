//go:build linux

package elevate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
)

func IsElevated() bool { return os.Geteuid() == 0 }

// WriteHosts re-execs the helper subcommand under pkexec, payload over stdin;
// the GUI stays unprivileged so only the helper touches the hosts file as root.
// sudo is used only when pkexec is not installed: a pkexec error means the user
// cancelled, authorization was refused, or the helper itself failed, and none
// of those should trigger a second elevation attempt.
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

	err = runElevated("pkexec", self, arg, data)
	if errors.Is(err, exec.ErrNotFound) {
		return runElevated("sudo", self, arg, data)
	}
	return err
}

func runElevated(tool, self, arg string, stdin []byte) error {
	cmd := exec.Command(tool, self, arg)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
