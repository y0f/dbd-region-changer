package hostsfile

import (
	"reflect"
	"testing"
)

func TestListAliasesIgnoresCommentsAndNormalizesNames(t *testing.T) {
	h := &Hosts{Path: writeTemp(t, "  # ignored comment\n192.0.2.1 PRIMARY.example Alias.Example. # not-a-host\ninvalid broken\n")}
	got, err := h.List(nil)
	want := [][2]string{{"192.0.2.1", "primary.example"}, {"192.0.2.1", "alias.example"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, %v", got, err)
	}
}

func TestRemovePreservesUnrelatedAliasesAndComments(t *testing.T) {
	p := writeTemp(t, "  # target.example is a comment\r\n192.0.2.1 keep.example TARGET.example. other.example # note\r\n192.0.2.2 target.example # retained comment\r\n")
	if err := (&Hosts{Path: p}).Remove("target.example"); err != nil {
		t.Fatal(err)
	}
	want := "  # target.example is a comment\r\n192.0.2.1 keep.example other.example # note\r\n# retained comment\r\n"
	if got := read(t, p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRejectCommentAndUnicodeWhitespaceInHostname(t *testing.T) {
	for _, host := range []string{"x#y", "x\u2003y", "x\u0085y"} {
		p := writeTemp(t, "127.0.0.1 localhost\n")
		if err := (&Hosts{Path: p}).Save("192.0.2.1", host); err == nil {
			t.Fatalf("accepted %q", host)
		}
	}
}
