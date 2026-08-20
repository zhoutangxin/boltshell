package main

import (
	"time"

	"boltshell/internal/sftpclient"
)

// browsePoolIdleTTL 传送对话框浏览目录时，临时 SSH 连接空闲多久后关闭
const browsePoolIdleTTL = 60 * time.Second

// browsePoolEntry 缓存的目标服务器 SFTP 连接（仅用于 BrowseConnectionDir / GetConnectionHome）
type browsePoolEntry struct {
	sftp     *sftpclient.Client
	cleanup  func()
	lastUsed time.Time
}

// getBrowseSFTP 从连接池获取或新建 SFTP 客户端，同一 connID 在 TTL 内复用 SSH 连接。
func (a *App) getBrowseSFTP(connID string) (*sftpclient.Client, error) {
	a.browseMu.Lock()
	a.evictStaleBrowseConnsLocked()
	if e, ok := a.browsePool[connID]; ok {
		e.lastUsed = time.Now()
		sc := e.sftp
		a.browseMu.Unlock()
		return sc, nil
	}
	a.browseMu.Unlock()

	sc, cleanup, err := a.dialConnectionSFTP(connID)
	if err != nil {
		return nil, err
	}

	a.browseMu.Lock()
	// dial 期间可能有并发请求，关闭重复连接只保留最新
	if old, ok := a.browsePool[connID]; ok {
		old.cleanup()
	}
	a.browsePool[connID] = &browsePoolEntry{
		sftp:     sc,
		cleanup:  cleanup,
		lastUsed: time.Now(),
	}
	a.browseMu.Unlock()
	return sc, nil
}

func (a *App) evictStaleBrowseConnsLocked() {
	now := time.Now()
	for id, e := range a.browsePool {
		if now.Sub(e.lastUsed) > browsePoolIdleTTL {
			e.cleanup()
			delete(a.browsePool, id)
		}
	}
}
