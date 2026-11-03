//go:build windows

package dbdpath

import (
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

func TestInstallDirsCoverEpicAndXbox(t *testing.T) {
	dirs := installDirs()
	for _, want := range []string{`C:\Program Files\Epic Games\DeadByDaylight`, `C:\XboxGames\Dead by Daylight`} {
		if !slices.Contains(dirs, want) {
			t.Errorf("installDirs() missing %q: %v", want, dirs)
		}
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
