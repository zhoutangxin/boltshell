package analytics

import "testing"

func TestLinkHostOnly(t *testing.T) {
	if got := linkHostOnly("https://example.com/path?q=1"); got != "example.com" {
		t.Fatalf("got %q", got)
	}
	if got := linkHostOnly("example.org"); got != "example.org" {
		t.Fatalf("got %q", got)
	}
	if got := linkHostOnly(""); got != "" {
		t.Fatalf("empty got %q", got)
	}
}

func TestInstallIDPersist(t *testing.T) {
	dir := t.TempDir()
	id1, err := loadOrCreateInstallID(dir)
	if err != nil || id1 == "" {
		t.Fatalf("first: %v %q", err, id1)
	}
	id2, err := loadOrCreateInstallID(dir)
	if err != nil || id2 != id1 {
		t.Fatalf("second: %v %q want %q", err, id2, id1)
	}
}
