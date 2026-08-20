package main

import (
	"errors"
	"os"
	"path/filepath"

	"boltshell/internal/sponsors"
	"boltshell/internal/updater"
	"boltshell/internal/version"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ensureUpdaterClient() {
	if a.remoteConfig.ReleaseURL == "" {
		exeDir := "."
		if exePath, err := os.Executable(); err == nil {
			exeDir = filepath.Dir(exePath)
		}
		a.remoteConfig = sponsors.LoadRemoteConfig(sponsors.RemoteConfigSearchPaths(exeDir)...)
		a.sponsorRemoteURL = a.remoteConfig.RemoteURL
	}
	a.updaterClient = updater.NewClient(a.remoteConfig.ReleaseURL)
}

// GetAppVersion 返回当前客户端版本
func (a *App) GetAppVersion() string {
	return version.Current()
}

// CheckForUpdate 检查是否有新版本
func (a *App) CheckForUpdate() updater.CheckResult {
	a.ensureUpdaterClient()
	return a.updaterClient.Check()
}

// ApplyUpdate 下载并安装最新版本（Windows 自动替换 exe 并重启）
func (a *App) ApplyUpdate(downloadURL string) error {
	a.ensureUpdaterClient()
	url := downloadURL
	if url == "" {
		check := a.updaterClient.Check()
		if check.CheckError != "" {
			return errors.New(check.CheckError)
		}
		if !check.HasUpdate {
			return errors.New("已是最新版本")
		}
		url = check.DownloadURL
	}
	if err := updater.Apply(url); err != nil {
		return err
	}
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
	return nil
}
