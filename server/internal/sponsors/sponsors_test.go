package sponsors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version < 1 {
		t.Fatalf("expected version >= 1, got %d", cfg.Version)
	}
	if len(cfg.Slots) == 0 {
		t.Fatal("expected default slots")
	}
	if _, ok := cfg.Slots["quick_connect_bottom"]; !ok {
		t.Fatal("missing quick_connect_bottom slot")
	}
}

func TestDismissStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sponsor.state")
	store := NewDismissStore(path)

	if err := store.Dismiss("sidebar_1", 7); err != nil {
		t.Fatal(err)
	}
	m, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if m["sidebar_1"] <= 0 {
		t.Fatal("expected dismiss until timestamp")
	}
	// 篡改文件后签名失效，应视为未关闭
	raw, _ := os.ReadFile(path)
	tampered := strings.ReplaceAll(string(raw), `"sidebar_1"`, `"sidebar_1_x"`)
	_ = os.WriteFile(path, []byte(tampered), 0o600)
	m2, _ := store.Load()
	if len(m2) != 0 {
		t.Fatal("tampered dismiss file should be ignored")
	}
}

func TestLoadLocalFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sponsors.json")
	if err := os.WriteFile(p, embeddedDefault, 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewClient(Options{LocalPath: p, CachePath: filepath.Join(dir, "cache.json")})
	cfg, err := c.Load(true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProUpgradeURL == "" {
		t.Fatal("expected pro upgrade url")
	}
}
