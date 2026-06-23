//go:build windows

// Package dbdpath locates the Dead by Daylight executable so firewall rules can
// be scoped to the game rather than the whole system.
package dbdpath

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const shippingExe = "DeadByDaylight-Win64-Shipping.exe"

// Find returns the DBD shipping exe path, or "" if not located.
func Find() string {
	for _, lib := range steamLibraries() {
		base := filepath.Join(lib, "steamapps", "common", "Dead by Daylight")
		direct := filepath.Join(base, "DeadByDaylight", "Binaries", "Win64", shippingExe)
		if isFile(direct) {
			return direct
		}
		if found := walkFor(base); found != "" {
			return found
		}
	}
	return ""
}

func walkFor(root string) string {
	var found string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), shippingExe) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func steamLibraries() []string {
	var libs []string
	if steam := steamPath(); steam != "" {
		libs = append(libs, steam)
		libs = append(libs, parseLibraryFolders(filepath.Join(steam, "steamapps", "libraryfolders.vdf"))...)
	}
	libs = append(libs,
		`C:\Program Files (x86)\Steam`,
		`C:\Program Files\Steam`,
		`C:\Steam`, `D:\Steam`, `D:\SteamLibrary`, `E:\SteamLibrary`,
	)
	return dedup(libs)
}

func steamPath() string {
	keys := []struct {
		root registry.Key
		path string
		val  string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
	}
	for _, k := range keys {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		s, _, err := key.GetStringValue(k.val)
		key.Close()
		if err == nil && s != "" {
			return filepath.Clean(s)
		}
	}
	return ""
}

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)

func parseLibraryFolders(vdf string) []string {
	data, err := os.ReadFile(vdf)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range vdfPath.FindAllStringSubmatch(string(data), -1) {
		out = append(out, strings.ReplaceAll(m[1], `\\`, `\`))
	}
	return out
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
