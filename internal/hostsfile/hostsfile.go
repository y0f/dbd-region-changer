// Package hostsfile reads and edits the OS hosts file; writes are in-place, not atomic temp+rename.
package hostsfile

import (
	"os"
	"strings"
)

type Entry struct {
	IP       string
	Hostname string
}

type Hosts struct {
	Path string
}

func New() *Hosts { return &Hosts{Path: defaultPath()} }

// List returns [ip, hostname] pairs for lines passing callback (nil accepts all); '#' lines and single-field lines are skipped.
func (h *Hosts) List(callback func(hostname string) bool) ([][2]string, error) {
	data, err := os.ReadFile(h.Path)
	if err != nil {
		return nil, err
	}
	var out [][2]string
	for _, raw := range strings.SplitAfter(string(data), "\n") {
		if strings.HasPrefix(raw, "#") {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) < 2 {
			continue
		}
		if callback == nil || callback(fields[1]) {
			out = append(out, [2]string{fields[0], fields[1]})
		}
	}
	return out, nil
}

// Save appends "ip hostname", prefixing a newline when the file's last line is non-blank. No trailing newline.
func (h *Hosts) Save(ip, hostname string) error {
	entry := ip + " " + hostname
	data, err := os.ReadFile(h.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if needsLeadingNewline(data) {
		entry = "\n" + entry
	}
	f, err := os.OpenFile(h.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// Remove drops every line whose second field equals hostname; shorter lines are kept verbatim.
func (h *Hosts) Remove(hostname string) error {
	data, err := os.ReadFile(h.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var b strings.Builder
	for _, line := range strings.SplitAfter(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[1] == hostname {
			continue
		}
		b.WriteString(line)
	}
	return os.WriteFile(h.Path, []byte(b.String()), 0o644)
}

func (h *Hosts) RemoveAll(hostnames []string) error {
	for _, hn := range hostnames {
		if err := h.Remove(hn); err != nil {
			return err
		}
	}
	return nil
}

// Apply saves every entry; caller must remove stale entries first (see internal/apply).
func (h *Hosts) Apply(entries []Entry) error {
	for _, e := range entries {
		if err := h.Save(e.IP, e.Hostname); err != nil {
			return err
		}
	}
	return nil
}

func needsLeadingNewline(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	parts := strings.SplitAfter(string(data), "\n")
	// SplitAfter yields a trailing "" when data ends in '\n'; drop it to examine the last real line.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return false
	}
	return strings.TrimSpace(parts[len(parts)-1]) != ""
}
