package main

import (
	"testing"
	"time"
)

// TestBrowsePoolEvictStale 验证空闲超时的 browse 连接会被清理
func TestBrowsePoolEvictStale(t *testing.T) {
	closed := false
	a := &App{
		browsePool: map[string]*browsePoolEntry{
			"conn-1": {
				sftp:     nil,
				cleanup:  func() { closed = true },
				lastUsed: time.Now().Add(-browsePoolIdleTTL - time.Second),
			},
		},
	}

	a.browseMu.Lock()
	a.evictStaleBrowseConnsLocked()
	a.browseMu.Unlock()

	if !closed {
		t.Fatal("expected stale browse connection to be cleaned up")
	}
	if len(a.browsePool) != 0 {
		t.Fatalf("expected empty pool, got %d entries", len(a.browsePool))
	}
}

// TestBrowsePoolKeepFresh 验证未过期的连接不会被清理
func TestBrowsePoolKeepFresh(t *testing.T) {
	closed := false
	a := &App{
		browsePool: map[string]*browsePoolEntry{
			"conn-1": {
				sftp:     nil,
				cleanup:  func() { closed = true },
				lastUsed: time.Now(),
			},
		},
	}

	a.browseMu.Lock()
	a.evictStaleBrowseConnsLocked()
	a.browseMu.Unlock()

	if closed {
		t.Fatal("fresh browse connection should not be cleaned up")
	}
	if len(a.browsePool) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(a.browsePool))
	}
}
