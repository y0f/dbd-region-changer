// Command mkicon generates assets/icon.png and build/icon.ico from a source image.
//
//	go run ./scripts/mkicon <source-image.png>
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/nfnt/resize"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mkicon <source-image>")
		os.Exit(2)
	}
	src, err := os.Open(os.Args[1])
	must(err)
	defer src.Close()

	img, _, err := image.Decode(src)
	must(err)

	must(writePNG("assets/icon.png", resize.Resize(256, 256, img, resize.Lanczos3)))

	// Multi-size Windows .ico (PNG-encoded entries, valid on Vista+).
	sizes := []uint{16, 32, 48, 64, 128, 256}
	frames := make([][]byte, len(sizes))
	for i, s := range sizes {
		var b bytes.Buffer
		must(png.Encode(&b, resize.Resize(s, s, img, resize.Lanczos3)))
		frames[i] = b.Bytes()
	}
	must(writeICO("build/icon.ico", sizes, frames))
	fmt.Println("wrote assets/icon.png and build/icon.ico")
}

func writePNG(path string, img image.Image) error {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func writeICO(path string, sizes []uint, frames [][]byte) error {
	var buf bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&buf, le, uint16(0))
	binary.Write(&buf, le, uint16(1)) // type: icon
	binary.Write(&buf, le, uint16(len(frames)))

	offset := 6 + 16*len(frames)
	for i, frame := range frames {
		dim := byte(sizes[i])
		if sizes[i] >= 256 {
			dim = 0 // 0 means 256 in the ICO format
		}
		buf.WriteByte(dim)
		buf.WriteByte(dim)
		buf.WriteByte(0)
		buf.WriteByte(0)
		binary.Write(&buf, le, uint16(1))
		binary.Write(&buf, le, uint16(32))
		binary.Write(&buf, le, uint32(len(frame)))
		binary.Write(&buf, le, uint32(offset))
		offset += len(frame)
	}
	for _, frame := range frames {
		buf.Write(frame)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkicon:", err)
		os.Exit(1)
	}
}
