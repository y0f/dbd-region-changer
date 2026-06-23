package banner

import (
	"strings"
	"testing"
)

func TestStringHasSignature(t *testing.T) {
	if !strings.Contains(String(), "by y0f") {
		t.Fatal("banner missing y0f signature")
	}
}

func TestPrintWritesArt(t *testing.T) {
	var b strings.Builder
	Print(&b)
	if !strings.Contains(b.String(), "dbd region changer") {
		t.Fatal("Print did not write banner")
	}
}
