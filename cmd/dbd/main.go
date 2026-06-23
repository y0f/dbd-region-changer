// Command dbd is the entrypoint: launches the GUI, and on Linux also serves the elevated --write-hosts/--clear-hosts helper subcommands.
package main

import (
	"fmt"
	"os"

	"github.com/y0f/dbd-region-changer/internal/applog"
	"github.com/y0f/dbd-region-changer/internal/banner"
	"github.com/y0f/dbd-region-changer/internal/elevate"
	"github.com/y0f/dbd-region-changer/internal/singleinstance"
	"github.com/y0f/dbd-region-changer/internal/version"
	"github.com/y0f/dbd-region-changer/ui"
)

func main() {
	if handled, err := elevate.HandleHelperSubcommand(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "hosts write failed:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	debug, force := parseFlags(os.Args[1:])

	logger, err := applog.New(debug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log init failed:", err)
		os.Exit(1)
	}
	defer logger.Close()
	if debug {
		banner.Print(os.Stderr)
	}
	logger.Info("starting", "version", version.Current.Display())

	lock, lerr := singleinstance.Acquire(singleinstance.DefaultName)
	if lerr != nil {
		logger.Warn("single-instance lock failed", "err", lerr)
	} else {
		defer lock.Release()
		if lock.IsRunning() && !force {
			logger.Info("duplicate process detected")
			ui.FatalDialog("Duplicate processes", "Another process is running")
			return
		}
	}

	if !elevate.IsElevated() {
		logger.Warn("process is not elevated; hosts writes may require elevation")
	}

	ui.New(logger).Run()
}

func parseFlags(args []string) (debug, force bool) {
	for _, a := range args {
		switch a {
		case "-d", "--debug":
			debug = true
		case "-f":
			force = true
		}
	}
	return debug, force
}
