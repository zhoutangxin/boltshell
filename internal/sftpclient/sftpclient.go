package sftpclient

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// RemoteEntry 远端文件/目录条目
type RemoteEntry struct {
	Name    string `json:"Name"`
	Path    string `json:"Path"`
	Size    int64  `json:"Size"`
	IsDir   bool   `json:"IsDir"`
	ModTime int64  `json:"ModTime"`
	Mode    string `json:"Mode"`
	Owner   string `json:"Owner"`
}

type Client struct {
	client *sftp.Client
}

func NewFromSSH(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil {
		return nil, fmt.Errorf("ssh client is nil")
	}
	c, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	return &Client{client: c}, nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func cleanRemotePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return "."
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func joinRemote(dir, name string) string {
	if dir == "" || dir == "." || dir == "/" {
		if name == ".." {
			return "/"
		}
		return path.Clean("/" + name)
	}
	return path.Clean(dir + "/" + name)
}

const maxEditSize = 2 * 1024 * 1024 // 2MB

// Stat 获取远端文件/目录信息
func (c *Client) Stat(remotePath string) (os.FileInfo, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("sftp not ready")
	}
	return c.client.Stat(cleanRemotePath(remotePath))
}

// OpenRead 打开远端文件用于读取（调用方需 Close）
func (c *Client) OpenRead(remotePath string) (io.ReadCloser, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("sftp not ready")
	}
	return c.client.Open(cleanRemotePath(remotePath))
}

// OpenWrite 打开远端文件用于写入（调用方需 Close）
func (c *Client) OpenWrite(remotePath string) (io.WriteCloser, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("sftp not ready")
	}
	return c.client.OpenFile(cleanRemotePath(remotePath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

// ReadFile 读取远端文件全部内容（限制大小，适用于在线编辑）
func (c *Client) ReadFile(remotePath string) ([]byte, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("sftp not ready")
	}
	p := cleanRemotePath(remotePath)
	info, err := c.client.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("无法访问文件: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("目标是目录，无法编辑")
	}
	if info.Size() > maxEditSize {
		return nil, fmt.Errorf("文件过大（%d 字节，上限 2MB），请下载后编辑", info.Size())
	}
	f, err := c.client.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteFile 将内容写入远端文件
func (c *Client) WriteFile(remotePath string, data []byte) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	p := cleanRemotePath(remotePath)
	f, err := c.client.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return wrapTransferErr("写入", p, err)
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return wrapTransferErr("写入", p, err)
	}
	return nil
}

// HomeDir 获取远端用户 home 目录
func (c *Client) HomeDir() (string, error) {
	wd, err := c.client.Getwd()
	if err == nil && wd != "" {
		return wd, nil
	}
	return "/", nil
}

// ListDir 列出目录内容
func (c *Client) ListDir(remotePath string) ([]RemoteEntry, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("sftp not ready")
	}
	dir := cleanRemotePath(remotePath)
	entries, err := c.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]RemoteEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		full := joinRemote(dir, name)
		mode := e.Mode()
		out = append(out, RemoteEntry{
			Name:    name,
			Path:    full,
			Size:    e.Size(),
			IsDir:   e.IsDir(),
			ModTime: e.ModTime().Unix(),
			Mode:    mode.String(),
			Owner:   "-",
		})
	}
	return out, nil
}

func (c *Client) Mkdir(remotePath string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	return c.client.Mkdir(cleanRemotePath(remotePath))
}

func (c *Client) Remove(remotePath string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	p := cleanRemotePath(remotePath)
	info, err := c.client.Stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return c.client.RemoveDirectory(p)
	}
	return c.client.Remove(p)
}

func (c *Client) Rename(oldPath, newPath string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	return c.client.Rename(cleanRemotePath(oldPath), cleanRemotePath(newPath))
}

// ProgressFunc 传输进度回调：已传输字节数、总字节数（总为 0 表示未知）
type ProgressFunc func(transferred, total int64)

type progressReader struct {
	r           io.Reader
	total       int64
	transferred int64
	onProgress  ProgressFunc
	mu          sync.Mutex
	lastEmit    time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.mu.Lock()
		p.transferred += int64(n)
		done, total := p.transferred, p.total
		shouldEmit := p.onProgress != nil && (time.Since(p.lastEmit) >= 120*time.Millisecond || err == io.EOF)
		if shouldEmit {
			p.lastEmit = time.Now()
		}
		cb := p.onProgress
		p.mu.Unlock()
		if shouldEmit && cb != nil {
			cb(done, total)
		}
	}
	return n, err
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress ProgressFunc) error {
	pr := &progressReader{r: src, total: total, onProgress: onProgress}
	_, err := io.Copy(dst, pr)
	if onProgress != nil {
		pr.mu.Lock()
		done := pr.transferred
		pr.mu.Unlock()
		onProgress(done, total)
	}
	return err
}

func wrapTransferErr(action, remotePath string, err error) error {
	if err == nil {
		return nil
	}
	p := cleanRemotePath(remotePath)
	msg := err.Error()
	hint := ""
	if strings.Contains(msg, "FX_FAILURE") || strings.Contains(msg, "Failure") {
		hint = "（常见原因：磁盘已满、目录只读、无写入权限，请换目录或执行 df -h 检查）"
	}
	return fmt.Errorf("%s %s 失败%s: %w", action, p, hint, err)
}

func (c *Client) openRemoteForWrite(remotePath string) (*sftp.File, error) {
	p := cleanRemotePath(remotePath)
	parent := path.Dir(p)
	if parent != p && parent != "." {
		if info, err := c.client.Stat(parent); err != nil {
			return nil, fmt.Errorf("目标目录不存在: %s", parent)
		} else if !info.IsDir() {
			return nil, fmt.Errorf("目标路径不是目录: %s", parent)
		}
	}
	f, err := c.client.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (c *Client) Upload(localPath, remotePath string, onProgress ProgressFunc) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return c.uploadDir(localPath, remotePath, onProgress)
	}
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()
	total := info.Size()
	dst, err := c.openRemoteForWrite(remotePath)
	if err != nil {
		return wrapTransferErr("上传", remotePath, err)
	}
	defer dst.Close()
	if err := copyWithProgress(dst, src, total, onProgress); err != nil {
		return wrapTransferErr("上传", remotePath, err)
	}
	return nil
}

func localDirTotalSize(localPath string) (int64, error) {
	var total int64
	err := filepath.Walk(localPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func (c *Client) mkdirRemoteAll(remotePath string) error {
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "/" || remotePath == "." {
		return nil
	}
	if _, err := c.client.Stat(remotePath); err == nil {
		return nil
	}
	parent := path.Dir(remotePath)
	if parent != remotePath {
		if err := c.mkdirRemoteAll(parent); err != nil {
			return err
		}
	}
	if err := c.client.Mkdir(remotePath); err != nil {
		if _, statErr := c.client.Stat(remotePath); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) uploadDir(localPath, remotePath string, onProgress ProgressFunc) error {
	localPath = filepath.Clean(localPath)
	remotePath = cleanRemotePath(remotePath)
	if err := c.mkdirRemoteAll(remotePath); err != nil {
		return err
	}
	total, err := localDirTotalSize(localPath)
	if err != nil {
		return err
	}
	dt := &dirTransfer{total: total, onProgress: onProgress}
	if onProgress != nil {
		onProgress(0, total)
	}
	err = filepath.Walk(localPath, func(lp string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(localPath, lp)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rp := joinRemote(remotePath, filepath.ToSlash(rel))
		if info.IsDir() {
			return c.mkdirRemoteAll(rp)
		}
		return c.uploadFileTracked(lp, rp, dt)
	})
	if err != nil {
		return err
	}
	dt.flush()
	return nil
}

func (c *Client) uploadFileTracked(localPath, remotePath string, dt *dirTransfer) error {
	if err := c.mkdirRemoteAll(path.Dir(cleanRemotePath(remotePath))); err != nil {
		return err
	}
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := c.openRemoteForWrite(remotePath)
	if err != nil {
		return wrapTransferErr("上传", remotePath, err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, &countingReader{r: src, dt: dt})
	if err != nil {
		return wrapTransferErr("上传", remotePath, err)
	}
	return nil
}

func (c *Client) Download(remotePath, localPath string, onProgress ProgressFunc) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("sftp not ready")
	}
	remotePath = cleanRemotePath(remotePath)
	info, err := c.client.Stat(remotePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return c.downloadDir(remotePath, localPath, onProgress)
	}
	return c.downloadFile(remotePath, localPath, info.Size(), onProgress)
}

func (c *Client) downloadFile(remotePath, localPath string, total int64, onProgress ProgressFunc) error {
	if total <= 0 {
		if info, err := c.client.Stat(remotePath); err == nil {
			total = info.Size()
		}
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	src, err := c.client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	return copyWithProgress(dst, src, total, onProgress)
}

type dirTransfer struct {
	total       int64
	transferred int64
	onProgress  ProgressFunc
	mu          sync.Mutex
	lastEmit    time.Time
}

func (dt *dirTransfer) add(n int64) {
	if n <= 0 {
		return
	}
	dt.mu.Lock()
	dt.transferred += n
	done, total := dt.transferred, dt.total
	shouldEmit := dt.onProgress != nil && time.Since(dt.lastEmit) >= 120*time.Millisecond
	if shouldEmit {
		dt.lastEmit = time.Now()
	}
	cb := dt.onProgress
	dt.mu.Unlock()
	if shouldEmit && cb != nil {
		cb(done, total)
	}
}

func (dt *dirTransfer) flush() {
	dt.mu.Lock()
	done, total := dt.transferred, dt.total
	cb := dt.onProgress
	dt.mu.Unlock()
	if cb != nil {
		cb(done, total)
	}
}

type countingReader struct {
	r io.Reader
	dt *dirTransfer
}

func (cr *countingReader) Read(b []byte) (int, error) {
	n, err := cr.r.Read(b)
	if n > 0 {
		cr.dt.add(int64(n))
	}
	return n, err
}

func (c *Client) dirTotalSize(remotePath string) (int64, error) {
	entries, err := c.client.ReadDir(remotePath)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		full := joinRemote(remotePath, e.Name())
		if e.IsDir() {
			sub, err := c.dirTotalSize(full)
			if err != nil {
				return 0, err
			}
			total += sub
		} else {
			total += e.Size()
		}
	}
	return total, nil
}

func (c *Client) downloadDir(remotePath, localPath string, onProgress ProgressFunc) error {
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return err
	}
	total, err := c.dirTotalSize(remotePath)
	if err != nil {
		return err
	}
	dt := &dirTransfer{total: total, onProgress: onProgress}
	if onProgress != nil {
		onProgress(0, total)
	}
	if err := c.walkDownloadDir(remotePath, localPath, dt); err != nil {
		return err
	}
	dt.flush()
	return nil
}

func (c *Client) walkDownloadDir(remotePath, localPath string, dt *dirTransfer) error {
	entries, err := c.client.ReadDir(remotePath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		rp := joinRemote(remotePath, name)
		lp := filepath.Join(localPath, name)
		if e.IsDir() {
			if err := os.MkdirAll(lp, 0o755); err != nil {
				return err
			}
			if err := c.walkDownloadDir(rp, lp, dt); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(lp), 0o755); err != nil {
			return err
		}
		src, err := c.client.Open(rp)
		if err != nil {
			return err
		}
		dst, err := os.Create(lp)
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(dst, &countingReader{r: src, dt: dt})
		src.Close()
		dst.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// FormatModTime 供前端显示
func FormatModTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).Format("2006/01/02 15:04")
}
