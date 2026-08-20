package sponsors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRemoteConfigEmbedded(t *testing.T) {
	cfg := LoadRemoteConfig("/nonexistent/sponsors.remote.json")
	if cfg.ReleaseURL == "" {
		t.Fatal("expected embedded releaseURL")
	}
	if cfg.RemoteURL == "" {
		t.Fatal("expected embedded remoteURL")
	}
}

func TestLoadRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sponsors.remote.json")
	content := `{
  "remoteURL":"http://192.168.1.1/config/sponsors.json",
  "releaseURL":"http://192.168.1.1/config/release.json",
  "proUpgradeURL":"http://192.168.1.1/pro"
}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadRemoteConfig(p)
	if cfg.RemoteURL != "http://192.168.1.1/config/sponsors.json" {
		t.Fatalf("remoteURL got %q", cfg.RemoteURL)
	}
	if cfg.ReleaseURL != "http://192.168.1.1/config/release.json" {
		t.Fatalf("releaseURL got %q", cfg.ReleaseURL)
	}
	if cfg.ProUpgradeURL != "http://192.168.1.1/pro" {
		t.Fatalf("proUpgradeURL got %q", cfg.ProUpgradeURL)
	}
}

func TestLoadRemoteURL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sponsors.remote.json")
	if err := os.WriteFile(p, []byte(`{"remoteURL":"http://192.168.1.1/config/sponsors.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadRemoteURL(p); got != "http://192.168.1.1/config/sponsors.json" {
		t.Fatalf("got %q", got)
	}
}
