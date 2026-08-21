package analytics

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"boltshell/internal/version"
)

const (
	batchSize     = 100
	flushInterval = 5 * time.Minute
	httpTimeout   = 12 * time.Second
)

// Config 上报配置（来自 sponsors.remote.json）
type Config struct {
	AnalyticsURL string
	AppKey       string
	AppSecret    string
	UserDataDir  string
	IsPro        func() bool
	Enabled      bool // 初始开关；之后可用 SetEnabled 改
}

// Client 本地队列 + 批量 POST
type Client struct {
	cfg   Config
	queue *queueStore

	mu              sync.Mutex
	enabled         bool
	installID       string
	impressionSeen  map[string]struct{}
	sshConnectedOnce bool

	stopCh chan struct{}
	wg     sync.WaitGroup
	http   *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	id, err := InstallID(cfg.UserDataDir)
	if err != nil {
		return nil, err
	}
	c := &Client{
		cfg:            cfg,
		queue:          newQueueStore(cfg.UserDataDir),
		enabled:        cfg.Enabled,
		installID:      id,
		impressionSeen: make(map[string]struct{}),
		stopCh:         make(chan struct{}),
		http:           &http.Client{Timeout: httpTimeout},
	}
	// 磁盘偏好覆盖初始值
	c.enabled = loadEnabled(cfg.UserDataDir, cfg.Enabled)
	return c, nil
}

func (c *Client) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		// 启动稍后冲一次，避免和启动风暴抢网
		startup := time.NewTimer(20 * time.Second)
		defer startup.Stop()
		for {
			select {
			case <-c.stopCh:
				_ = c.Flush()
				return
			case <-startup.C:
				_ = c.Flush()
			case <-ticker.C:
				_ = c.Flush()
			}
		}
	}()
}

func (c *Client) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
	c.wg.Wait()
}

func (c *Client) SetEnabled(enabled bool) error {
	c.mu.Lock()
	c.enabled = enabled
	c.mu.Unlock()
	return saveEnabled(c.cfg.UserDataDir, enabled)
}

func (c *Client) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled
}

func (c *Client) InstallID() string {
	return c.installID
}

// Track 写入本地队列（不直接打 HTTP）
func (c *Client) Track(event string, props map[string]interface{}) {
	c.mu.Lock()
	enabled := c.enabled
	c.mu.Unlock()
	if !enabled {
		return
	}
	if props == nil {
		props = map[string]interface{}{}
	}
	isPro := false
	if c.cfg.IsPro != nil {
		isPro = c.cfg.IsPro()
	}
	// Pro 不报赞助事件；启动仍报
	if isPro && strings.HasPrefix(event, "sponsor_") {
		return
	}
	ev := Event{
		Schema:     1,
		Event:      event,
		Ts:         nowISO(),
		InstallID:  c.installID,
		AppVersion: version.Current(),
		OS:         mapOS(),
		IsPro:      isPro,
		Props:      props,
	}
	_ = c.queue.Append(ev)
	if c.queue.Len() >= batchSize {
		go func() { _ = c.Flush() }()
	}
}

// TrackAppLaunch 进程启动一次
func (c *Client) TrackAppLaunch() {
	c.Track("app_launch", map[string]interface{}{"ui": "gui"})
}

// TrackSSHConnected 可选：首次会话成功
func (c *Client) TrackSSHConnected() {
	c.mu.Lock()
	if c.sshConnectedOnce {
		c.mu.Unlock()
		return
	}
	c.sshConnectedOnce = true
	c.mu.Unlock()
	c.Track("ssh_connected", map[string]interface{}{})
}

// TrackSponsor 赞助位事件；impression 按 surfaceSession+slot+日去重
func (c *Client) TrackSponsor(kind, slotID, surfaceSession, linkURL string, configVersion int) {
	kind = strings.TrimSpace(kind)
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return
	}
	event := ""
	switch kind {
	case "impression":
		event = "sponsor_impression"
		day := time.Now().Format("2006-01-02")
		key := surfaceSession + "|" + slotID + "|" + day
		c.mu.Lock()
		if _, ok := c.impressionSeen[key]; ok {
			c.mu.Unlock()
			return
		}
		c.impressionSeen[key] = struct{}{}
		c.mu.Unlock()
	case "click":
		event = "sponsor_click"
	case "dismiss":
		event = "sponsor_dismiss"
	default:
		return
	}
	props := map[string]interface{}{
		"slotId":        slotID,
		"configVersion": configVersion,
	}
	if host := linkHostOnly(linkURL); host != "" {
		props["linkHost"] = host
	}
	c.Track(event, props)
}

type batchBody struct {
	BatchID string  `json:"batchId"`
	Events  []Event `json:"events"`
}

func (c *Client) Flush() error {
	c.mu.Lock()
	enabled := c.enabled
	urlStr := strings.TrimSpace(c.cfg.AnalyticsURL)
	appKey := strings.TrimSpace(c.cfg.AppKey)
	appSecret := strings.TrimSpace(c.cfg.AppSecret)
	c.mu.Unlock()
	if !enabled || urlStr == "" || appKey == "" || appSecret == "" {
		return nil
	}

	for {
		events, err := c.queue.Peek(batchSize)
		if err != nil || len(events) == 0 {
			return err
		}
		body := batchBody{
			BatchID: newBatchID(),
			Events:  events,
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if err := c.postSigned(urlStr, appKey, appSecret, raw); err != nil {
			return err
		}
		if err := c.queue.Drop(len(events)); err != nil {
			return err
		}
		if len(events) < batchSize {
			return nil
		}
	}
}

func (c *Client) postSigned(endpoint, appKey, appSecret string, body []byte) error {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	sum := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(ts + "\n" + bodyHash))
	sign := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BoltShell/"+version.Current())
	req.Header.Set("X-BoltShell-App-Key", appKey)
	req.Header.Set("X-BoltShell-Ts", ts)
	req.Header.Set("X-BoltShell-Sign", sign)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("analytics http %d", resp.StatusCode)
	}
	// 兼容管理台统一 JSON：{code:0} 才算成功，避免 200+业务失败仍丢队列
	var envelope struct {
		Code int `json:"code"`
	}
	if len(respBody) > 0 && json.Unmarshal(respBody, &envelope) == nil {
		if envelope.Code != 0 {
			return fmt.Errorf("analytics business code %d", envelope.Code)
		}
	}
	return nil
}

func mapOS() string {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return runtime.GOOS
	default:
		return "linux"
	}
}

func linkHostOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" || strings.ContainsAny(host, "/?#") {
		return ""
	}
	return host
}

func newBatchID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:]) + fmt.Sprintf("%d", time.Now().UnixNano()%1e6)
}
