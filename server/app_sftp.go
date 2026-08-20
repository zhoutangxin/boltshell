// SFTP 文件操作与跨服务器传送（Wails API）。
// 已连接会话复用 terminalHolder.sftp；BrowseConnectionDir 临时 Dial 目标服务器。
package main

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"boltshell/internal/db"
	"boltshell/internal/sftpclient"
	"boltshell/internal/sshclient"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SessionInfo 已连接会话的简要信息（供前端选择目标服务器）
type SessionInfo struct {
	ID    string `json:"ID"`
	Title string `json:"Title"`
}

// ListActiveSessions 返回当前所有已连接的会话列表
func (a *App) ListActiveSessions() []SessionInfo {
	a.termMu.Lock()
	defer a.termMu.Unlock()
	out := make([]SessionInfo, 0, len(a.sessions))
	for id, h := range a.sessions {
		if h != nil && h.sftp != nil {
			out = append(out, SessionInfo{ID: id, Title: h.title})
		}
	}
	return out
}

func (a *App) getSFTP(sessionID string) (*sftpclient.Client, error) {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	a.termMu.Unlock()
	if h == nil || h.sftp == nil {
		return nil, fmt.Errorf("会话不存在或未就绪 SFTP")
	}
	return h.sftp, nil
}

// GetRemoteHome 返回远端初始目录（一般为 home）
func (a *App) GetRemoteHome(sessionID string) (string, error) {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return "", err
	}
	return c.HomeDir()
}

// ListRemoteDir 列出远端目录
func (a *App) ListRemoteDir(sessionID string, remotePath string) ([]sftpclient.RemoteEntry, error) {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return nil, err
	}
	return c.ListDir(remotePath)
}

// MkdirRemote 创建远端目录
func (a *App) MkdirRemote(sessionID string, remotePath string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	return c.Mkdir(remotePath)
}

// RemoveRemote 删除远端文件或空目录
func (a *App) RemoveRemote(sessionID string, remotePath string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	return c.Remove(remotePath)
}

// RenameRemote 重命名远端路径
func (a *App) RenameRemote(sessionID string, oldPath string, newPath string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	return c.Rename(oldPath, newPath)
}

// ReadRemoteFile 读取远端文件内容（用于在线编辑，限 2MB）
func (a *App) ReadRemoteFile(sessionID string, remotePath string) (string, error) {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return "", err
	}
	data, err := c.ReadFile(remotePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteRemoteFile 将内容写回远端文件
func (a *App) WriteRemoteFile(sessionID string, remotePath string, content string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	return c.WriteFile(remotePath, []byte(content))
}

// TransferBetweenServers 从源会话传输到目标会话（都已连接）
func (a *App) TransferBetweenServers(srcSessionID string, srcPath string, dstSessionID string, dstDir string) error {
	if srcSessionID == dstSessionID {
		return fmt.Errorf("源和目标不能是同一个会话")
	}
	srcSFTP, err := a.getSFTP(srcSessionID)
	if err != nil {
		return fmt.Errorf("源服务器 SFTP 未就绪: %w", err)
	}
	dstSFTP, err := a.getSFTP(dstSessionID)
	if err != nil {
		return fmt.Errorf("目标服务器 SFTP 未就绪: %w", err)
	}
	return a.doTransfer(srcSFTP, dstSFTP, srcPath, dstDir)
}

// TransferToConnection 从源会话传输到目标连接（按连接 ID，临时建 SFTP）
// taskID 由前端传入，用于推送进度
func (a *App) TransferToConnection(srcSessionID string, srcPath string, dstConnID string, dstDir string, taskID string) error {
	emit := func(msg string) {
		runtime.EventsEmit(a.ctx, "transfer-log", msg)
	}
	emitProgress := func(total, transferred int64) {
		runtime.EventsEmit(a.ctx, "srv-transfer-progress", map[string]interface{}{
			"TaskID":      taskID,
			"Total":       total,
			"Transferred": transferred,
		})
	}

	srcSFTP, err := a.getSFTP(srcSessionID)
	if err != nil {
		return fmt.Errorf("源服务器 SFTP 未就绪: %w", err)
	}

	conn, err := db.GetByID(a.db, dstConnID)
	if err != nil {
		return fmt.Errorf("找不到目标连接: %w", err)
	}

	emit(fmt.Sprintf("正在连接目标服务器 %s:%d …", conn.Host, conn.Port))
	sshClient, err := sshclient.Dial(conn.Host, conn.Port, conn.User, conn.Password)
	if err != nil {
		return fmt.Errorf("连接目标服务器失败（%s:%d）: %w", conn.Host, conn.Port, err)
	}
	defer sshClient.Close()
	emit(fmt.Sprintf("已连接 %s", conn.Host))

	dstSFTP, err := sftpclient.NewFromSSH(sshClient)
	if err != nil {
		return fmt.Errorf("目标 SFTP 初始化失败: %w", err)
	}
	defer dstSFTP.Close()

	// 先计算总大小
	totalSize, _ := a.calcRemoteSize(srcSFTP, srcPath)
	if totalSize > 0 {
		emitProgress(totalSize, 0)
	}

	emit(fmt.Sprintf("开始传输 %s → %s", path.Base(srcPath), dstDir))
	var transferred int64
	err = a.doTransferWithLog(srcSFTP, dstSFTP, srcPath, dstDir, emit, func(n int64) {
		transferred += n
		emitProgress(totalSize, transferred)
	})
	if err != nil {
		return err
	}
	emitProgress(totalSize, totalSize)
	emit("传输完成 ✓")
	return nil
}

func (a *App) calcRemoteSize(c *sftpclient.Client, remotePath string) (int64, error) {
	info, err := c.Stat(remotePath)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	entries, err := c.ListDir(remotePath)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if e.IsDir {
			sub, _ := a.calcRemoteSize(c, e.Path)
			total += sub
		} else {
			total += e.Size
		}
	}
	return total, nil
}

func (a *App) dialConnectionSFTP(connID string) (*sftpclient.Client, func(), error) {
	conn, err := db.GetByID(a.db, connID)
	if err != nil {
		return nil, nil, fmt.Errorf("找不到连接: %w", err)
	}
	sshClient, err := sshclient.Dial(conn.Host, conn.Port, conn.User, conn.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("连接失败（%s:%d）: %w", conn.Host, conn.Port, err)
	}
	sc, err := sftpclient.NewFromSSH(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, fmt.Errorf("SFTP 初始化失败: %w", err)
	}
	cleanup := func() {
		sc.Close()
		sshClient.Close()
	}
	return sc, cleanup, nil
}

// GetConnectionHome 返回目标服务器 home 目录（复用 browse 连接池）
func (a *App) GetConnectionHome(connID string) (string, error) {
	sc, err := a.getBrowseSFTP(connID)
	if err != nil {
		return "", err
	}
	return sc.HomeDir()
}

// BrowseConnectionDir 列出目标服务器指定目录下的子文件夹（复用 browse 连接池）
func (a *App) BrowseConnectionDir(connID string, remotePath string) ([]sftpclient.RemoteEntry, error) {
	sc, err := a.getBrowseSFTP(connID)
	if err != nil {
		return nil, err
	}

	entries, err := sc.ListDir(remotePath)
	if err != nil {
		return nil, err
	}
	// 只保留目录
	dirs := make([]sftpclient.RemoteEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		}
	}
	return dirs, nil
}

type emitFn func(string)
type progressFn func(int64)

func noProgress(int64) {}

func (a *App) doTransfer(srcSFTP, dstSFTP *sftpclient.Client, srcPath, dstDir string) error {
	return a.doTransferWithLog(srcSFTP, dstSFTP, srcPath, dstDir, func(string) {}, noProgress)
}

func (a *App) doTransferWithLog(srcSFTP, dstSFTP *sftpclient.Client, srcPath, dstDir string, emit emitFn, onProgress progressFn) error {
	info, err := srcSFTP.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("无法访问源路径: %w", err)
	}
	baseName := path.Base(srcPath)
	dstPath := path.Join(dstDir, baseName)
	if info.IsDir() {
		return a.transferDirBetween(srcSFTP, dstSFTP, srcPath, dstPath, emit, onProgress)
	}
	return a.transferFileBetween(srcSFTP, dstSFTP, srcPath, dstPath, emit, onProgress)
}

func (a *App) transferFileBetween(src, dst *sftpclient.Client, srcPath, dstPath string, emit emitFn, onProgress progressFn) error {
	emit(fmt.Sprintf("  传文件 %s", path.Base(srcPath)))
	reader, err := src.OpenRead(srcPath)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %w", err)
	}
	defer reader.Close()

	writer, err := dst.OpenWrite(dstPath)
	if err != nil {
		return fmt.Errorf("写入目标文件失败: %w", err)
	}
	defer writer.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if _, wErr := writer.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("传输失败: %w", wErr)
			}
			onProgress(int64(n))
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("传输失败: %w", readErr)
		}
	}
	return nil
}

func (a *App) transferDirBetween(src, dst *sftpclient.Client, srcDir, dstDir string, emit emitFn, onProgress progressFn) error {
	emit(fmt.Sprintf("  创建目录 %s", dstDir))
	if err := dst.Mkdir(dstDir); err != nil {
		// 目录可能已存在，忽略
	}
	entries, err := src.ListDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}
	for _, e := range entries {
		sp := e.Path
		dp := path.Join(dstDir, e.Name)
		if e.IsDir {
			if err := a.transferDirBetween(src, dst, sp, dp, emit, onProgress); err != nil {
				return err
			}
		} else {
			if err := a.transferFileBetween(src, dst, sp, dp, emit, onProgress); err != nil {
				return err
			}
		}
	}
	return nil
}

// PickLocalFile 选择本机文件（上传用）
func (a *App) PickLocalFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要上传的文件",
	})
}

// PickLocalDir 选择本机文件夹（上传用）
func (a *App) PickLocalDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要上传的文件夹",
	})
}

// PickSaveFile 选择本机保存路径（下载用）
func (a *App) PickSaveFile(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存到",
		DefaultFilename: defaultName,
	})
}

// UploadToRemote 上传本机文件到远端
func (a *App) UploadToRemote(sessionID string, localPath string, remotePath string) error {
	if strings.TrimSpace(localPath) == "" {
		return fmt.Errorf("未选择本地文件")
	}
	if strings.TrimSpace(remotePath) == "" {
		base := filepath.Base(localPath)
		remotePath = base
	}
	return a.uploadWithProgress(sessionID, localPath, remotePath)
}

// DownloadFromRemote 从远端下载到本机
func (a *App) DownloadFromRemote(sessionID string, remotePath string, localPath string) error {
	if strings.TrimSpace(localPath) == "" {
		return fmt.Errorf("未选择保存路径")
	}
	return a.downloadWithProgress(sessionID, remotePath, localPath)
}
