// Package hostsfile reads and edits the OS hosts file. Each edit reads the file
// once and writes it back at most once, in place (a rename would drop the
// file's ACLs or SELinux label), and only when the content actually changes.
package hostsfile

import (
	"bytes"
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

// Save appends the line "ip hostname".
func (h *Hosts) Save(ip, hostname string) error {
	return h.Update(nil, []Entry{{IP: ip, Hostname: hostname}})
}

// Remove drops every line whose second field equals hostname; shorter lines are kept verbatim.
func (h *Hosts) Remove(hostname string) error {
	return h.Update([]string{hostname}, nil)
}

// RemoveAll drops every line whose second field is one of hostnames.
func (h *Hosts) RemoveAll(hostnames []string) error {
	return h.Update(hostnames, nil)
}

// Apply appends every entry; caller must remove stale entries first (see Update and internal/apply).
func (h *Hosts) Apply(entries []Entry) error {
	return h.Update(nil, entries)
}

// Update removes every line whose second field is one of remove, then appends
// entries, in a single read and at most one write. Appended lines use the
// file's line ending (CRLF if it already has any) and end with a newline, so a
// later append by another tool starts on its own line.
func (h *Hosts) Update(remove []string, entries []Entry) error {
	data, err := os.ReadFile(h.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out := edit(data, remove, entries)
	if bytes.Equal(out, data) {
		return nil // also covers a missing file with nothing to add
	}
	perm := os.FileMode(0o644)
	if fi, serr := os.Stat(h.Path); serr == nil {
		perm = fi.Mode().Perm()
	}
	return os.WriteFile(h.Path, out, perm)
}

func edit(data []byte, remove []string, entries []Entry) []byte {
	drop := make(map[string]bool, len(remove))
	for _, hn := range remove {
		drop[hn] = true
	}
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	var b strings.Builder
	b.Grow(len(data))
	for _, line := range strings.SplitAfter(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && drop[fields[1]] {
			continue
		}
		b.WriteString(line)
	}
	if len(entries) > 0 && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteString(eol)
	}
	for _, e := range entries {
		b.WriteString(e.IP + " " + e.Hostname + eol)
	}
	return []byte(b.String())
}
