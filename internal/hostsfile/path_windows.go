//go:build windows

package hostsfile

import (
	"os"
	"path/filepath"
)

// defaultPath honors %SystemRoot%, falling back to C:\Windows.
func defaultPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "drivers", "etc", "hosts")
}
