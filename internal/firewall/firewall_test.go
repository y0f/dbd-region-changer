package firewall

import "testing"

func TestChunk(t *testing.T) {
	cidrs := make([]string, 450)
	for i := range cidrs {
		cidrs[i] = "1.2.3.0/24"
	}
	groups := chunk(cidrs, 200)
	if len(groups) != 3 {
		t.Fatalf("got %d groups want 3", len(groups))
	}
	if len(groups[0]) != 200 || len(groups[1]) != 200 || len(groups[2]) != 50 {
		t.Fatalf("bad group sizes: %d %d %d", len(groups[0]), len(groups[1]), len(groups[2]))
	}
}

func TestChunkEmpty(t *testing.T) {
	if groups := chunk(nil, 200); len(groups) != 0 {
		t.Fatalf("expected no groups, got %d", len(groups))
	}
}
