package sponsors

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// 发版前请同步 config/sponsors.remote.json → internal/sponsors/remote.embed.json，再 wails build
//
//go:embed remote.embed.json
var embeddedRemoteConfig []byte

const defaultBaseURL = "http://47.108.138.168"

type RemoteConfig struct {
	RemoteURL     string `json:"remoteURL"`
	ReleaseURL    string `json:"releaseURL"`
	ProUpgradeURL string `json:"proUpgradeURL"`
}

// RemoteConfigSearchPaths 查找 sponsors.remote.json（exe 旁 config/ 可覆盖内置）
func RemoteConfigSearchPaths(exeDir string) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	dir := exeDir
	for i := 0; i < 6; i++ {
		add(filepath.Join(dir, "config", "sponsors.remote.json"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	add("config/sponsors.remote.json")
	return paths
}

// LoadRemoteConfig：外部文件优先 → 编译时内置（正式版单 exe 无需附带 config/）
func LoadRemoteConfig(paths ...string) RemoteConfig {
	for _, p := range paths {
		if cfg := loadRemoteConfigFile(p); isRemoteConfigValid(cfg) {
			normalizeRemoteConfig(&cfg)
			return cfg
		}
	}
	if cfg := loadRemoteConfigBytes(embeddedRemoteConfig); isRemoteConfigValid(cfg) {
		normalizeRemoteConfig(&cfg)
		return cfg
	}
	return RemoteConfig{}
}

// LoadRemoteURL 兼容旧调用
func LoadRemoteURL(paths ...string) string {
	return LoadRemoteConfig(paths...).RemoteURL
}

func loadRemoteConfigFile(path string) RemoteConfig {
	path = strings.TrimSpace(path)
	if path == "" {
		return RemoteConfig{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return RemoteConfig{}
	}
	return loadRemoteConfigBytes(b)
}

func loadRemoteConfigBytes(b []byte) RemoteConfig {
	var rc RemoteConfig
	if err := json.Unmarshal(b, &rc); err != nil {
		return RemoteConfig{}
	}
	normalizeRemoteConfig(&rc)
	return rc
}

func isRemoteConfigValid(c RemoteConfig) bool {
	return c.RemoteURL != "" || c.ReleaseURL != "" || c.ProUpgradeURL != ""
}

func normalizeRemoteConfig(c *RemoteConfig) {
	c.RemoteURL = strings.TrimSpace(c.RemoteURL)
	c.ReleaseURL = strings.TrimSpace(c.ReleaseURL)
	c.ProUpgradeURL = strings.TrimSpace(c.ProUpgradeURL)
	if c.ProUpgradeURL == "" {
		c.ProUpgradeURL = defaultBaseURL + "/#pricing"
	}
}
