package main

import (
	"strings"

	"boltshell/internal/analytics"
)

// TrackSponsorEvent 前端埋点入口：impression / click / dismiss
// surfaceSession 用于同一 UI 态内曝光去重（如 qc-<mountId> / sb-<sessionId>）
func (a *App) TrackSponsorEvent(kind, slotID, surfaceSession, linkURL string, configVersion int) {
	if a.analytics == nil {
		return
	}
	a.analytics.TrackSponsor(kind, slotID, surfaceSession, linkURL, configVersion)
}

// GetAnalyticsEnabled 是否开启匿名统计
func (a *App) GetAnalyticsEnabled() bool {
	if a.analytics == nil {
		return true
	}
	return a.analytics.Enabled()
}

// SetAnalyticsEnabled 设置「帮助改进产品（匿名统计）」开关
func (a *App) SetAnalyticsEnabled(enabled bool) error {
	if a.analytics == nil {
		return nil
	}
	return a.analytics.SetEnabled(enabled)
}

// FlushAnalytics 立即冲刷本地队列（调试用）
func (a *App) FlushAnalytics() error {
	if a.analytics == nil {
		return nil
	}
	return a.analytics.Flush()
}

// GetInstallID 匿名安装 ID（调试/关于页）
func (a *App) GetInstallID() string {
	if a.analytics == nil {
		return ""
	}
	return a.analytics.InstallID()
}

func (a *App) initAnalytics() {
	dir := a.userDataDir
	if dir == "" {
		dir = "."
	}
	cfg := analytics.Config{
		AnalyticsURL: strings.TrimSpace(a.remoteConfig.AnalyticsURL),
		AppKey:       strings.TrimSpace(a.remoteConfig.AnalyticsAppKey),
		AppSecret:    strings.TrimSpace(a.remoteConfig.AnalyticsAppSecret),
		UserDataDir:  dir,
		IsPro:        a.isProLicensed,
		Enabled:      true, // 默认开；用户可关；磁盘 prefs 可覆盖
	}
	client, err := analytics.NewClient(cfg)
	if err != nil {
		if a.logger != nil {
			a.logger.Info("analytics init skipped: %v", err)
		}
		return
	}
	a.analytics = client
	a.analytics.Start()
	a.analytics.TrackAppLaunch()
}
