package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"boltshell/internal/version"
)

// Release 远程 release.json
type Release struct {
	Version      string `json:"version"`
	BuildNumber  int    `json:"buildNumber"`
	ReleaseNotes string `json:"releaseNotes"`
	DownloadURL  string `json:"downloadURL"`
	Mandatory    bool   `json:"mandatory"`
	PublishedAt  string `json:"publishedAt"`
}

// CheckResult 版本检查结果
type CheckResult struct {
	CurrentVersion  string `json:"CurrentVersion"`
	LatestVersion   string `json:"LatestVersion"`
	HasUpdate       bool   `json:"HasUpdate"`
	ReleaseNotes    string `json:"ReleaseNotes"`
	DownloadURL     string `json:"DownloadURL"`
	Mandatory       bool   `json:"Mandatory"`
	PublishedAt     string `json:"PublishedAt"`
	CheckError      string `json:"CheckError,omitempty"`
}

// Client 拉取 release.json 并执行升级
type Client struct {
	releaseURL string
	httpClient *http.Client
}

func NewClient(releaseURL string) *Client {
	return &Client{
		releaseURL: strings.TrimSpace(releaseURL),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Check() CheckResult {
	out := CheckResult{CurrentVersion: version.Current()}
	if c.releaseURL == "" {
		out.CheckError = "未配置 releaseURL"
		return out
	}
	rel, err := c.fetchRelease()
	if err != nil {
		out.CheckError = err.Error()
		return out
	}
	out.LatestVersion = strings.TrimSpace(rel.Version)
	out.ReleaseNotes = rel.ReleaseNotes
	out.DownloadURL = strings.TrimSpace(rel.DownloadURL)
	out.Mandatory = rel.Mandatory
	out.PublishedAt = rel.PublishedAt
	out.HasUpdate = out.LatestVersion != "" && Compare(out.CurrentVersion, out.LatestVersion) < 0
	return out
}

func (c *Client) fetchRelease() (Release, error) {
	req, err := http.NewRequest(http.MethodGet, c.releaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("User-Agent", "BoltShell/"+version.Current())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return Release{}, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, err
	}
	if strings.TrimSpace(rel.Version) == "" {
		return Release{}, fmt.Errorf("release.json 缺少 version")
	}
	return rel, nil
}

// Apply 下载安装包并启动替换流程（Windows）
func Apply(downloadURL string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("当前仅支持 Windows 自动升级")
	}
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return fmt.Errorf("缺少下载地址")
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(filepath.Base(exePath)), "-dev") {
		return fmt.Errorf("开发模式不支持自动升级，请使用 wails build 打包正式版")
	}

	tmpDir := os.TempDir()
	newExe := filepath.Join(tmpDir, "BoltShell-update.exe")
	if err := downloadFile(cDefaultHTTP(), downloadURL, newExe); err != nil {
		return err
	}
	if err := validateWindowsExe(newExe); err != nil {
		_ = os.Remove(newExe)
		return err
	}

	backupExe := exePath + ".bak"
	batPath := filepath.Join(tmpDir, "boltshell-upgrade.bat")
	// 不在 bat 内自删（否则会报「找不到批处理文件」）
	script := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
copy /Y "%s" "%s" >nul
copy /Y "%s" "%s" >nul
start "" "%s"
`, exePath, backupExe, newExe, exePath, exePath)
	if err := os.WriteFile(batPath, []byte(script), 0o644); err != nil {
		return err
	}

	cmd := exec.Command("cmd", "/C", batPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

func cDefaultHTTP() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute}
}

func downloadFile(client *http.Client, url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "BoltShell/"+version.Current())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download http %d", resp.StatusCode)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") {
		return fmt.Errorf("下载地址返回了网页而非安装包，请检查服务器 releases 目录")
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 512*1024*1024))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}

const minExeSize = 512 * 1024 // 512KB，防止把 HTML 页面当成 exe

func validateWindowsExe(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() < minExeSize {
		return fmt.Errorf("下载文件过小(%d bytes)，可能不是有效的安装包，请检查服务器 releases 目录", info.Size())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(b) < 2 || b[0] != 'M' || b[1] != 'Z' {
		return fmt.Errorf("下载文件不是有效的 Windows 可执行文件，请检查 downloadURL 是否指向真实 exe")
	}
	return nil
}

// Compare 语义化版本比较：-1 当前旧，0 相同，1 当前新
func Compare(current, latest string) int {
	c := parseParts(current)
	l := parseParts(latest)
	for i := 0; i < 3; i++ {
		if c[i] < l[i] {
			return -1
		}
		if c[i] > l[i] {
			return 1
		}
	}
	return 0
}

func parseParts(v string) [3]int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	parts := strings.SplitN(v, ".", 3)
	out := [3]int{}
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}
