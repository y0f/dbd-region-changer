// Package applog writes a timestamped log file; debug mode also mirrors to a console.
package applog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrSameDir is returned when dumping into the log's own directory.
var ErrSameDir = errors.New("cannot use the same directory as the source")

type Logger struct {
	*slog.Logger
	dir  string
	path string
	file *os.File
}

// New opens a log file in a fresh temp dir; debug mode also mirrors to a console.
func New(debug bool) (*Logger, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "dcreg_tmp_")
	if err != nil {
		return nil, err
	}
	ts := time.Now().Format("2006-01-02 15.04.05")
	path := filepath.Join(dir, fmt.Sprintf("[%s]LogFile_dcreg.log", ts))
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	var w io.Writer = f
	switch {
	case debug:
		openConsole()
		w = io.MultiWriter(f, os.Stderr)
	case stdoutAttached():
		w = io.MultiWriter(f, os.Stdout)
	}

	logger := slog.New(&lineHandler{w: w, mu: &sync.Mutex{}})
	return &Logger{Logger: logger, dir: dir, path: path, file: f}, nil
}

func (l *Logger) LogPath() string { return l.path }

// DumpTo copies the log into dir, or returns ErrSameDir if dir is the source.
func (l *Logger) DumpTo(dir string) error {
	dst := filepath.Join(dir, filepath.Base(l.path))
	absDst, _ := filepath.Abs(dst)
	absSrc, _ := filepath.Abs(l.path)
	if absDst == absSrc {
		return ErrSameDir
	}
	data, err := os.ReadFile(l.path)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (l *Logger) Close() error { return l.file.Close() }

// Cleanup closes the log file and removes its temp directory.
func (l *Logger) Cleanup() {
	if l.file != nil {
		l.file.Close()
	}
	if l.dir != "" {
		os.RemoveAll(l.dir)
	}
}

func stdoutAttached() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// lineHandler formats records as "[DBDRegion-Debug][LEVEL] time message k=v".
type lineHandler struct {
	w  io.Writer
	mu *sync.Mutex
}

func (h *lineHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	fmt.Fprintf(&b, "[DBDRegion-Debug][%s]%s %s",
		levelName(r.Level), r.Time.Format("2006-01-02T15:04:05"), r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *lineHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *lineHandler) WithGroup(string) slog.Handler      { return h }

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARNING"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}
