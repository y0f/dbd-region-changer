//go:build windows

package dbdpath

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkForMatchesEveryStorefrontExe(t *testing.T) {
	for _, name := range gameExes {
		root := t.TempDir()
		bin := filepath.Join(root, "DeadByDaylight", "Binaries", "Win64")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(bin, name)
		if err := os.WriteFile(want, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := walkFor(root); got != want {
			t.Errorf("walkFor = %q, want %q", got, want)
		}
	}
}

func TestFindPrefersSavedPath(t *testing.T) {
	useTempConfigDir(t)
	exe := filepath.Join(t.TempDir(), "DeadByDaylight-WinGDK-Shipping.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(exe); err != nil {
		t.Fatal(err)
	}
	if got := Find(); got != exe {
		t.Errorf("Find() = %q, want saved %q", got, exe)
	}
}

// useTempInstallDirs points every storefront location Find searches into a
// fresh temp dir and returns it, so a real install cannot leak into the test.
func useTempInstallDirs(t *testing.T) string {
	useTempConfigDir(t)
	root := t.TempDir()
	oldSteam, oldManifest, oldEpic, oldXbox := steamLibs, epicManifest, epicDefault, xboxDirs
	t.Cleanup(func() { steamLibs, epicManifest, epicDefault, xboxDirs = oldSteam, oldManifest, oldEpic, oldXbox })
	steamLibs = func() []string { return []string{filepath.Join(root, "Steam")} }
	epicManifest = filepath.Join(root, "LauncherInstalled.dat")
	epicDefault = filepath.Join(root, "Epic Games", "DeadByDaylight")
	xboxDirs = []string{filepath.Join(root, "D", "XboxGames", "Dead by Daylight")}
	return root
}

func writeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindLocatesStorefrontInstalls(t *testing.T) {
	for name, rel := range map[string][]string{
		"steam":        {"Steam", "steamapps", "common", "Dead by Daylight", "DeadByDaylight", "Binaries", "Win64", "DeadByDaylight-Win64-Shipping.exe"},
		"epic default": {"Epic Games", "DeadByDaylight", "DeadByDaylight", "Binaries", "Win64", "DeadByDaylight-EGS-Shipping.exe"},
		"xbox":         {"D", "XboxGames", "Dead by Daylight", "Content", "DeadByDaylight", "Binaries", "WinGDK", "DeadByDaylight-WinGDK-Shipping.exe"},
	} {
		t.Run(name, func(t *testing.T) {
			root := useTempInstallDirs(t)
			if got := Find(); got != "" {
				t.Fatalf("Find() with nothing installed = %q", got)
			}
			want := filepath.Join(append([]string{root}, rel...)...)
			writeExe(t, want)
			if got := Find(); got != want {
				t.Errorf("Find() = %q, want %q", got, want)
			}
		})
	}
}

func TestFindLocatesEpicManifestInstall(t *testing.T) {
	root := useTempInstallDirs(t)
	dir := filepath.Join(root, "Games", "Dead by Daylight")
	data, err := json.Marshal(map[string]any{"InstallationList": []map[string]string{{"InstallLocation": dir}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epicManifest, data, 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "DeadByDaylight", "Binaries", "Win64", "DeadByDaylight-EGS-Shipping.exe")
	writeExe(t, want)
	if got := Find(); got != want {
		t.Errorf("Find() = %q, want %q", got, want)
	}
}

func TestEpicInstallsReadsManifest(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "LauncherInstalled.dat")
	data := `{"InstallationList":[` +
		`{"InstallLocation":"D:\\Games\\DeadByDaylight","AppName":"x"},` +
		`{"InstallLocation":"D:\\Games\\Fortnite","AppName":"y"}]}`
	if err := os.WriteFile(manifest, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := epicInstalls(manifest)
	if want := []string{`D:\Games\DeadByDaylight`}; !slices.Equal(got, want) {
		t.Errorf("epicInstalls = %v, want %v", got, want)
	}
}
