package sponsors

import (
	_ "embed"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed default.json
var embeddedDefault []byte

const defaultRemoteURL = "https://boltshell.com/config/sponsors.json"

// Slot 单个赞助位配置
type Slot struct {
	Enabled     bool   `json:"enabled"`
	Type        string `json:"type"` // banner | compact | self_promo
	Badge       string `json:"badge"`
	Title       string `json:"title"`
	Desc        string `json:"desc"`
	LinkURL     string `json:"linkUrl"`
	ImageURL    string `json:"imageUrl,omitempty"`
	DismissDays int    `json:"dismissDays,omitempty"`
}

// Config 远程/本地赞助配置
type Config struct {
	Version         int              `json:"version"`
	UpdatedAt       string           `json:"updatedAt"`
	CacheTTLSeconds int              `json:"cacheTTLSeconds"`
	ProUpgradeURL   string           `json:"proUpgradeUrl"`
	Slots           map[string]Slot  `json:"slots"`
}

// Client 拉取并缓存赞助配置
type Client struct {
	mu sync.Mutex

	remoteURL   string
	localPath   string
	cachePath   string
	httpClient  *http.Client
	cached      Config
	cachedAt    time.Time
	hasCache    bool
}

// Options 创建 Client 的参数
type Options struct {
	RemoteURL string
	LocalPath string
	CachePath string
}

// NewClient 创建赞助配置客户端
func NewClient(opt Options) *Client {
	url := strings.TrimSpace(opt.RemoteURL)
	if url == "" {
		url = defaultRemoteURL
	}
	return &Client{
		remoteURL: url,
		localPath: strings.TrimSpace(opt.LocalPath),
		cachePath: strings.TrimSpace(opt.CachePath),
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

// DefaultConfig 返回内置默认配置
func DefaultConfig() (Config, error) {
	var c Config
	if err := json.Unmarshal(embeddedDefault, &c); err != nil {
		return Config{}, err
	}
	normalizeConfig(&c)
	return c, nil
}

// Load 按优先级加载：内存缓存 → 远程 → 本地文件 → 磁盘缓存 → 内置默认
func (c *Client) Load(forceRefresh bool) (Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !forceRefresh && c.hasCache && !c.cacheExpired() {
		return c.cached, nil
	}

	if cfg, err := c.fetchRemote(); err == nil {
		c.storeCache(cfg)
		return cfg, nil
	}

	if c.localPath != "" {
		if cfg, err := loadFile(c.localPath); err == nil {
			c.storeCache(cfg)
			return cfg, nil
		}
	}

	if c.cachePath != "" {
		if cfg, err := loadDiskCache(c.cachePath); err == nil {
			c.storeCache(cfg)
			return cfg, nil
		}
	}

	def, err := DefaultConfig()
	if err != nil {
		return Config{}, err
	}
	c.storeCache(def)
	return def, nil
}

func (c *Client) cacheExpired() bool {
	ttl := c.cached.CacheTTLSeconds
	if ttl <= 0 {
		ttl = 21600
	}
	return time.Since(c.cachedAt) > time.Duration(ttl)*time.Second
}

func (c *Client) storeCache(cfg Config) {
	normalizeConfig(&cfg)
	c.cached = cfg
	c.cachedAt = time.Now()
	c.hasCache = true
	if c.cachePath != "" {
		_ = saveDiskCache(c.cachePath, cfg)
	}
}

func (c *Client) fetchRemote() (Config, error) {
	req, err := http.NewRequest(http.MethodGet, c.remoteURL, nil)
	if err != nil {
		return Config{}, err
	}
	req.Header.Set("User-Agent", "BoltShell/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Config{}, fmt.Errorf("sponsor config http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadFile(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type diskCache struct {
	FetchedAt int64  `json:"fetchedAt"`
	Config    Config `json:"config"`
}

func loadDiskCache(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var dc diskCache
	if err := json.Unmarshal(b, &dc); err != nil {
		return Config{}, err
	}
	return dc.Config, nil
}

func saveDiskCache(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dc := diskCache{FetchedAt: time.Now().Unix(), Config: cfg}
	b, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func normalizeConfig(c *Config) {
	if c.CacheTTLSeconds <= 0 {
		c.CacheTTLSeconds = 21600
	}
	if c.ProUpgradeURL == "" {
		c.ProUpgradeURL = "https://boltshell.com/pro"
	}
	if c.Slots == nil {
		c.Slots = map[string]Slot{}
	}
}

// DismissStore 记录用户暂时关闭的赞助位
type DismissStore struct {
	path string
}

func NewDismissStore(path string) *DismissStore {
	return &DismissStore{path: path}
}

type dismissFile struct {
	Until map[string]int64 `json:"until"`
	Sig   string           `json:"sig"`
}

func dismissSignKey() []byte {
	// 与机器无关的 app 级 pepper；防止用户手改 JSON 延长关闭时间（删文件则恢复展示）
	return []byte("boltshell-dismiss-v1")
}

func signDismiss(until map[string]int64) string {
	b, _ := json.Marshal(until)
	mac := hmac.New(sha256.New, dismissSignKey())
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *DismissStore) Load() (map[string]int64, error) {
	out := map[string]int64{}
	if s.path == "" {
		return out, nil
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	var df dismissFile
	if err := json.Unmarshal(b, &df); err != nil {
		return out, nil
	}
	if df.Until == nil || df.Sig == "" {
		return out, nil
	}
	if !hmac.Equal([]byte(df.Sig), []byte(signDismiss(df.Until))) {
		// 篡改或旧版明文文件：视为无效，赞助位恢复展示
		return out, nil
	}
	now := time.Now().Unix()
	for id, until := range df.Until {
		if until > now {
			out[id] = until
		}
	}
	return out, nil
}

func (s *DismissStore) Dismiss(slotID string, days int) error {
	if s.path == "" || slotID == "" {
		return nil
	}
	if days <= 0 {
		days = 7
	}
	m, err := s.Load()
	if err != nil {
		m = map[string]int64{}
	}
	m[slotID] = time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	df := dismissFile{Until: m, Sig: signDismiss(m)}
	b, err := json.Marshal(df)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}
