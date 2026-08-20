// 本地上传/下载进度事件与系统文件夹对话框。
package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"boltshell/internal/db"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TransferEvent 文件传输进度事件（推送到前端）
type TransferEvent struct {
	TaskID      string `json:"TaskID"`
	SessionID   string `json:"SessionID"`
	Kind        string `json:"Kind"`
	FileName    string `json:"FileName"`
	Source      string `json:"Source"`
	Dest        string `json:"Dest"`
	Total       int64  `json:"Total"`
	Transferred int64  `json:"Transferred"`
	Status      string `json:"Status"`
	Error       string `json:"Error"`
}

func (a *App) emitTransfer(ev TransferEvent) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "transfer-update", ev)
}

func (a *App) uploadWithProgress(sessionID, localPath, remotePath string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	taskID := db.NewID()
	fileName := filepath.Base(localPath)
	var lastTotal, lastDone int64
	emit := func(status string, errMsg string) {
		ev := TransferEvent{
			TaskID: taskID, SessionID: sessionID, Kind: "upload",
			FileName: fileName, Source: localPath, Dest: remotePath,
			Total: lastTotal, Transferred: lastDone, Status: status, Error: errMsg,
		}
		if status == "done" && lastTotal > 0 {
			ev.Transferred = lastTotal
		}
		a.emitTransfer(ev)
	}
	emit("running", "")
	err = c.Upload(localPath, remotePath, func(done, total int64) {
		lastDone, lastTotal = done, total
		emit("running", "")
	})
	if err != nil {
		emit("error", err.Error())
		return err
	}
	emit("done", "")
	return nil
}

func (a *App) downloadWithProgress(sessionID, remotePath, localPath string) error {
	c, err := a.getSFTP(sessionID)
	if err != nil {
		return err
	}
	taskID := db.NewID()
	fileName := filepath.Base(remotePath)
	var lastTotal, lastDone int64
	emit := func(status string, errMsg string) {
		ev := TransferEvent{
			TaskID: taskID, SessionID: sessionID, Kind: "download",
			FileName: fileName, Source: remotePath, Dest: localPath,
			Total: lastTotal, Transferred: lastDone, Status: status, Error: errMsg,
		}
		if status == "done" && lastTotal > 0 {
			ev.Transferred = lastTotal
		}
		a.emitTransfer(ev)
	}
	emit("running", "")
	err = c.Download(remotePath, localPath, func(done, total int64) {
		lastDone, lastTotal = done, total
		emit("running", "")
	})
	if err != nil {
		emit("error", err.Error())
		return err
	}
	emit("done", "")
	return nil
}

// PickDownloadDir 选择默认下载目录
func (a *App) PickDownloadDir() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择默认下载目录",
	})
}

// OpenLocalFolder 在资源管理器中打开本机目录
func (a *App) OpenLocalFolder(dirPath string) error {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return fmt.Errorf("目录为空")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", dirPath).Start()
	case "darwin":
		return exec.Command("open", dirPath).Start()
	default:
		return exec.Command("xdg-open", dirPath).Start()
	}
}
