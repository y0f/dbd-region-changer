// Package assets embeds binary resources (icons) into the executable.
package assets

import _ "embed"

//go:embed icon.png
var IconPNG []byte
