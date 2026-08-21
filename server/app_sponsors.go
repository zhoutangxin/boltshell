package main

import (
	"os"
	"path/filepath"
	"time"

	"boltshell/internal/license"
	"boltshell/internal/sponsors"
)

// SponsorSlotView 返回给前端的单个赞助位
type SponsorSlotView struct {
	SlotID string `json:"SlotID"`
	sponsors.Slot
}

// SponsorConfigView 赞助配置 + Pro 状态 + 已关闭位
type SponsorConfigView struct {
	Version         int               `json:"Version"`
	UpdatedAt       string            `json:"UpdatedAt"`
	CacheTTLSeconds int               `json:"CacheTTLSeconds"`
	ProUpgradeURL   string            `json:"ProUpgradeURL"`
	IsPro           bool              `json:"IsPro"`
	Slots           []SponsorSlotView `json:"Slots"`
	DismissedUntil  map[string]int64  `json:"DismissedUntil"`
}

func (a *App) ensureSponsorClient() {
	if a.sponsorClient != nil {
		return
	}
	dir := a.userDataDir
	if dir == "" {
		dir = "."
	}
	a.sponsorClient = sponsors.NewClient(sponsors.Options{
		RemoteURL: a.sponsorRemoteURL,
		LocalPath: a.sponsorLocalPath,
		CachePath: filepath.Join(dir, "sponsors.cache"),
	})
	a.sponsorDismiss = sponsors.NewDismissStore(filepath.Join(dir, "sponsor.state"))
}

func (a *App) isProLicensed() bool {
	return license.IsPro(a.proLicensedDev)
}

// GetSponsorConfig 拉取赞助位配置（带缓存）；Pro 用户不返回 Slots
func (a *App) GetSponsorConfig(forceRefresh bool) (SponsorConfigView, error) {
	a.ensureSponsorClient()
	cfg, err := a.sponsorClient.Load(forceRefresh)
	if err != nil {
		return SponsorConfigView{}, err
	}

	dismissed, _ := a.sponsorDismiss.Load()
	view := SponsorConfigView{
		Version:         cfg.Version,
		UpdatedAt:       cfg.UpdatedAt,
		CacheTTLSeconds: cfg.CacheTTLSeconds,
		ProUpgradeURL:   firstNonEmpty(cfg.ProUpgradeURL, a.remoteConfig.ProUpgradeURL),
		IsPro:           a.isProLicensed(),
		DismissedUntil:  dismissed,
		Slots:           []SponsorSlotView{},
	}

	if view.IsPro {
		return view, nil
	}

	now := time.Now().Unix()
	for id, slot := range cfg.Slots {
		if !slot.Enabled {
			continue
		}
		if until, ok := dismissed[id]; ok && until > now {
			continue
		}
		view.Slots = append(view.Slots, SponsorSlotView{SlotID: id, Slot: slot})
	}

	order := []string{"quick_connect_bottom", "sidebar_1", "sidebar_2"}
	ordered := make([]SponsorSlotView, 0, len(view.Slots))
	seen := map[string]bool{}
	for _, key := range order {
		for _, s := range view.Slots {
			if s.SlotID == key {
				ordered = append(ordered, s)
				seen[key] = true
			}
		}
	}
	for _, s := range view.Slots {
		if !seen[s.SlotID] {
			ordered = append(ordered, s)
		}
	}
	view.Slots = ordered
	return view, nil
}

// DismissSponsorSlot 暂时关闭赞助位
func (a *App) DismissSponsorSlot(slotID string, days int) error {
	a.ensureSponsorClient()
	if days <= 0 {
		days = 7
	}
	linkURL := ""
	cfgVer := 0
	if a.sponsorClient != nil {
		if cfg, err := a.sponsorClient.Load(false); err == nil {
			cfgVer = cfg.Version
			if slot, ok := cfg.Slots[slotID]; ok {
				linkURL = slot.LinkURL
			}
		}
	}
	if a.analytics != nil {
		a.analytics.TrackSponsor("dismiss", slotID, "dismiss", linkURL, cfgVer)
	}
	return a.sponsorDismiss.Dismiss(slotID, days)
}

// RefreshSponsorConfig 强制刷新远程配置（跳过内存缓存）
func (a *App) RefreshSponsorConfig() (SponsorConfigView, error) {
	return a.GetSponsorConfig(true)
}

// IsProLicensed 是否 Pro（去广告）
func (a *App) IsProLicensed() bool {
	return a.isProLicensed()
}

func resolveSponsorLocalPath(exeDir string) string {
	for _, p := range []string{
		filepath.Join(exeDir, "config", "sponsors.json"),
		filepath.Join(exeDir, "sponsors.json"),
		"config/sponsors.json",
		"config/sponsors.default.json",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
