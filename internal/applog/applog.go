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
	stop chan struct{}
}

// tempPrefix names the per-run log directories under os.TempDir().
const tempPrefix = "dcreg_tmp_"

// staleAge is how long a leftover log directory (from a crash, kill or logoff)
// is kept before New sweeps it.
const staleAge = 72 * time.Hour

// keepAliveEvery is how often a running Logger refreshes its dir's mtime, so a
// long-idle instance's dir never looks stale to another instance's sweep.
var keepAliveEvery = time.Hour

// New opens a log file in a fresh temp dir; debug mode also mirrors to a console.
// It also removes log dirs left behind by earlier runs that did not exit cleanly.
func New(debug bool) (*Logger, error) {
	sweepStale(os.TempDir(), time.Now().Add(-staleAge))
	dir, err := os.MkdirTemp(os.TempDir(), tempPrefix)
	if err != nil {
		return nil, err
	}
	ts := time.Now().Format("2006-01-02 15.04.05")
	path := filepath.Join(dir, fmt.Sprintf("[%s]LogFile_dcreg.log", ts))
	f, err := os.Create(path)
	if err != nil {
		os.RemoveAll(dir)
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
	stop := make(chan struct{})
	go keepAlive(dir, time.NewTicker(keepAliveEvery), stop)
	return &Logger{Logger: logger, dir: dir, path: path, file: f, stop: stop}, nil
}

// keepAlive touches dir on every tick until stop is closed. The app may log
// nothing for days, so the log file's own mtime cannot be relied on.
func keepAlive(dir string, t *time.Ticker, stop <-chan struct{}) {
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			now := time.Now()
			os.Chtimes(dir, now, now)
		}
	}
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

// Close closes the log file and removes its temp directory. It is safe to call
// more than once.
func (l *Logger) Close() error {
	var err error
	if l.stop != nil {
		close(l.stop)
		l.stop = nil
	}
	if l.file != nil {
		err = l.file.Close()
		l.file = nil
	}
	if l.dir != "" {
		if rerr := os.RemoveAll(l.dir); rerr != nil && err == nil {
			err = rerr
		}
		l.dir = ""
	}
	return err
}

// Cleanup is Close without the error, for use in defers and shutdown hooks.
func (l *Logger) Cleanup() { _ = l.Close() }

// sweepStale removes tempPrefix dirs in root whose newest entry is older than
// cutoff. A running instance's dir is never older than keepAliveEvery, since
// its Logger keeps touching it.
func sweepStale(root string, cutoff time.Time) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if newestModTime(dir).Before(cutoff) {
			os.RemoveAll(dir)
		}
	}
}

func newestModTime(dir string) time.Time {
	var newest time.Time
	if fi, err := os.Stat(dir); err == nil {
		newest = fi.ModTime()
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

func stdoutAttached() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// lineHandler formats records as "[DBDRegion-Debug][LEVEL] time message k=v".
// Attributes added through With are kept preformatted in attrs; WithGroup
// prefixes later keys with "group.".
type lineHandler struct {
	w      io.Writer
	mu     *sync.Mutex
	attrs  string
	prefix string
}

func (h *lineHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	fmt.Fprintf(&b, "[DBDRegion-Debug][%s]%s %s",
		levelName(r.Level), r.Time.Format("2006-01-02T15:04:05"), r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, h.prefix, a)
		return true
	})
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	var b strings.Builder
	b.WriteString(h.attrs)
	for _, a := range attrs {
		writeAttr(&b, h.prefix, a)
	}
	h2 := *h
	h2.attrs = b.String()
	return &h2
}

func (h *lineHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.prefix = h.prefix + name + "."
	return &h2
}

// writeAttr appends " prefix+key=value", flattening groups into dotted keys.
func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			writeAttr(b, prefix, ga)
		}
		return
	}
	fmt.Fprintf(b, " %s%s=%v", prefix, a.Key, a.Value.Any())
}

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
