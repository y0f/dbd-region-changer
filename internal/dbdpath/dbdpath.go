// Package dbdpath locates the Dead by Daylight executable so firewall rules can
// be scoped to the game rather than the whole system, and remembers a path the
// user picked by hand.
package dbdpath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// gameExes are the shipping binaries of the Steam, Epic Games Store and
// Microsoft Store / Game Pass builds. The DeadByDaylight.exe launcher stub is
// deliberately not listed: rules scoped to it miss the game's traffic.
var gameExes = []string{
	"DeadByDaylight-Win64-Shipping.exe",
	"DeadByDaylight-EGS-Shipping.exe",
	"DeadByDaylight-WinGDK-Shipping.exe",
}

// IsGameExe reports whether path names one of the known DBD shipping binaries.
func IsGameExe(path string) bool {
	base := filepath.Base(path)
	for _, name := range gameExes {
		if strings.EqualFold(base, name) {
			return true
		}
	}
	return false
}

// savedFile is where a manually chosen path is stored.
func savedFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dbd-region-changer", "dbd_path"), nil
}

// Save remembers path so later Find calls return it first.
func Save(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return errors.New("DBD path must be a file, not a folder")
	}
	f, err := savedFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	return os.WriteFile(f, []byte(path), 0o644)
}

// Saved returns the remembered path if it still points at a file, else "".
func Saved() string {
	f, err := savedFile()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(data))
	if p == "" || !isFile(p) {
		return ""
	}
	return p
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
