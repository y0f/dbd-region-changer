package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// appliedStateFile is where the region locked via the firewall is remembered,
// because the rules outlive the app ("" disables it). Replaceable in tests.
var appliedStateFile = func() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dbd-region-changer", "applied_region")
}

// saveApplied remembers code as the locked region; "" forgets it.
func saveApplied(code string) error {
	f := appliedStateFile()
	if f == "" {
		return nil
	}
	if code == "" {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	return os.WriteFile(f, []byte(code), 0o644)
}

// loadApplied returns the remembered region, or "" if none is saved or its
// rules cannot have survived: iptables rules are lost on reboot.
func loadApplied() string {
	f := appliedStateFile()
	if f == "" {
		return ""
	}
	fi, err := os.Stat(f)
	if err != nil {
		return ""
	}
	if runtime.GOOS == "linux" {
		if stat, err := os.ReadFile("/proc/stat"); err == nil {
			if boot, ok := parseBootTime(string(stat)); ok && fi.ModTime().Before(boot) {
				_ = os.Remove(f)
				return ""
			}
		}
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parseBootTime reads the "btime" line of /proc/stat.
func parseBootTime(procStat string) (time.Time, bool) {
	for _, line := range strings.Split(procStat, "\n") {
		if v, ok := strings.CutPrefix(line, "btime "); ok {
			sec, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			return time.Unix(sec, 0), true
		}
	}
	return time.Time{}, false
}
