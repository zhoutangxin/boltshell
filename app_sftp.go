package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"shelllite/internal/sftpclient"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
