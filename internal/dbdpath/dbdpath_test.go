package dbdpath

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempConfigDir points os.UserConfigDir at a temp dir on every platform.
func useTempConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestIsGameExe(t *testing.T) {
	for _, p := range []string{
		filepath.Join("steam", "DeadByDaylight-Win64-Shipping.exe"),
		"/x/deadbydaylight-egs-shipping.exe",
		"DeadByDaylight-WinGDK-Shipping.exe",
	} {
		if !IsGameExe(p) {
			t.Errorf("IsGameExe(%q) = false", p)
		}
	}
	for _, p := range []string{"DeadByDaylight.exe", "EasyAntiCheat_EOS.exe", ""} {
		if IsGameExe(p) {
			t.Errorf("IsGameExe(%q) = true", p)
		}
	}
}

func TestSaveThenSaved(t *testing.T) {
	useTempConfigDir(t)
	if got := Saved(); got != "" {
		t.Fatalf("Saved() before Save = %q, want empty", got)
	}
	exe := filepath.Join(t.TempDir(), "DeadByDaylight-EGS-Shipping.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(exe); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Saved(); got != exe {
		t.Errorf("Saved() = %q, want %q", got, exe)
	}
	os.Remove(exe)
	if got := Saved(); got != "" {
		t.Errorf("Saved() after the exe was removed = %q, want empty", got)
	}
}

func TestSaveRejectsMissingOrDir(t *testing.T) {
	useTempConfigDir(t)
	if err := Save(filepath.Join(t.TempDir(), "nope.exe")); err == nil {
		t.Error("Save of a missing file succeeded")
	}
	if err := Save(t.TempDir()); err == nil {
		t.Error("Save of a directory succeeded")
	}
}
