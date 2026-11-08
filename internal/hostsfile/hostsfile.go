// Package hostsfile reads and edits the OS hosts file. Each edit reads the file
// once and writes it back at most once, in place (a rename would drop the
// file's ACLs or SELinux label), and only when the content actually changes.
package hostsfile

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"unicode"
)

type Entry struct {
	IP       string
	Hostname string
}

type Hosts struct {
	Path string
}

func New() *Hosts { return &Hosts{Path: defaultPath()} }

// List returns every hostname, including aliases, with case and trailing dots
// normalized. Comments and malformed lines are ignored.
func (h *Hosts) List(callback func(hostname string) bool) ([][2]string, error) {
	data, err := os.ReadFile(h.Path)
	if err != nil {
		return nil, err
	}
	var out [][2]string
	for _, raw := range strings.SplitAfter(string(data), "\n") {
		body, _, _ := strings.Cut(raw, "#")
		fields := strings.Fields(body)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			continue
		}
		for _, alias := range fields[1:] {
			hostname := normalize(alias)
			if callback == nil || callback(hostname) {
				out = append(out, [2]string{fields[0], hostname})
			}
		}
	}
	return out, nil
}

// Save appends the line "ip hostname".
func (h *Hosts) Save(ip, hostname string) error {
	return h.Update(nil, []Entry{{IP: ip, Hostname: hostname}})
}

// Remove removes hostname, preserving other aliases and comments on its line.
func (h *Hosts) Remove(hostname string) error {
	return h.Update([]string{hostname}, nil)
}

// RemoveAll removes the named hosts, preserving unrelated aliases and comments.
func (h *Hosts) RemoveAll(hostnames []string) error {
	return h.Update(hostnames, nil)
}

// Apply appends every entry; caller must remove stale entries first (see Update and internal/apply).
func (h *Hosts) Apply(entries []Entry) error {
	return h.Update(nil, entries)
}

// Update removes matching hostnames and aliases, then appends
// entries, in a single read and at most one write. Appended lines use the
// file's line ending (CRLF if it already has any) and end with a newline, so a
// later append by another tool starts on its own line. An entry with an
// invalid IP or a hostname containing whitespace or control characters is
// rejected and nothing is written.
func (h *Hosts) Update(remove []string, entries []Entry) error {
	for _, e := range entries {
		if err := e.validate(); err != nil {
			return err
		}
	}
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

// validate rejects entries that would not form exactly one "ip hostname" line.
func (e Entry) validate() error {
	if net.ParseIP(e.IP) == nil {
		return fmt.Errorf("hostsfile: invalid IP %q", e.IP)
	}
	if e.Hostname == "" || strings.IndexFunc(e.Hostname, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '#' }) >= 0 {
		return fmt.Errorf("hostsfile: invalid hostname %q", e.Hostname)
	}
	return nil
}

func normalize(hostname string) string {
	return strings.ToLower(strings.TrimSuffix(hostname, "."))
}

func edit(data []byte, remove []string, entries []Entry) []byte {
	drop := make(map[string]bool, len(remove))
	for _, hn := range remove {
		drop[normalize(hn)] = true
	}
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	var b strings.Builder
	b.Grow(len(data))
	for _, line := range strings.SplitAfter(string(data), "\n") {
		body, comment, hasComment := strings.Cut(strings.TrimRight(line, "\r\n"), "#")
		fields := strings.Fields(body)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			b.WriteString(line)
			continue
		}
		kept := []string{fields[0]}
		for _, alias := range fields[1:] {
			if !drop[normalize(alias)] {
				kept = append(kept, alias)
			}
		}
		if len(kept) == len(fields) {
			b.WriteString(line)
			continue
		}
		if len(kept) > 1 {
			b.WriteString(strings.Join(kept, " "))
			if hasComment {
				b.WriteString(" ")
			}
		}
		if hasComment {
			b.WriteString("#" + comment)
		}
		if (len(kept) > 1 || hasComment) && strings.HasSuffix(line, "\n") {
			b.WriteString(eol)
		}
	}
	if len(entries) > 0 && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteString(eol)
	}
	for _, e := range entries {
		b.WriteString(e.IP + " " + e.Hostname + eol)
	}
	return []byte(b.String())
}
