package applog

import (
	"os"
	"strings"
	"testing"
)

func TestNewWritesToLogFile(t *testing.T) {
	l, err := New(false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(l.LogPath())
	defer l.Close()

	if l.LogPath() == "" {
		t.Fatal("empty log path")
	}
	l.Info("hello", "k", "v")

	data, err := os.ReadFile(l.LogPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("log message not written: %q", string(data))
	}
}

func TestDumpTo(t *testing.T) {
	l, err := New(false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(l.LogPath())
	defer l.Close()
	l.Info("dump me")

	dir := t.TempDir()
	if err := l.DumpTo(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 dumped file, got %d", len(entries))
	}
}
