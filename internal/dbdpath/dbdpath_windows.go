//go:build windows

package dbdpath

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Find returns the DBD shipping exe path, or "" if not located. A path saved
// with Save wins; otherwise Steam, Epic and Xbox app install folders are searched.
func Find() string {
	if p := Saved(); p != "" {
		return p
	}
	for _, base := range installDirs() {
		for _, name := range gameExes {
			if direct := filepath.Join(base, "DeadByDaylight", "Binaries", "Win64", name); isFile(direct) {
				return direct
			}
		}
		if found := walkFor(base); found != "" {
			return found
		}
	}
	return ""
}

// installDirs lists candidate DBD install folders across storefronts.
func installDirs() []string {
	var dirs []string
	for _, lib := range steamLibraries() {
		dirs = append(dirs, filepath.Join(lib, "steamapps", "common", "Dead by Daylight"))
	}
	dirs = append(dirs, epicInstalls(`C:\ProgramData\Epic\UnrealEngineLauncher\LauncherInstalled.dat`)...)
	dirs = append(dirs, `C:\Program Files\Epic Games\DeadByDaylight`)
	for _, drive := range []string{"C", "D", "E"} {
		dirs = append(dirs, drive+`:\XboxGames\Dead by Daylight`)
	}
	return dedup(dirs)
}

// epicInstalls returns the Epic Games Launcher install folders that look like DBD.
func epicInstalls(manifest string) []string {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil
	}
	var m struct {
		InstallationList []struct {
			InstallLocation string
		}
	}
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	var out []string
	for _, e := range m.InstallationList {
		name := strings.ToLower(strings.ReplaceAll(filepath.Base(e.InstallLocation), " ", ""))
		if strings.Contains(name, "deadbydaylight") {
			out = append(out, filepath.Clean(e.InstallLocation))
		}
	}
	return out
}

func walkFor(root string) string {
	var found string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && IsGameExe(d.Name()) {
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
