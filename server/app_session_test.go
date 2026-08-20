package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// newTestApp 构造一个不带 Wails ctx 的 App：emitEvent 会因 ctx==nil 静默跳过，
// 于是会话生命周期可以脱离真实 SSH 与前端单独测试。
func newTestApp(sessionIDs ...string) *App {
	a := &App{sessions: map[string]*terminalHolder{}}
	for _, id := range sessionIDs {
		a.sessions[id] = &terminalHolder{title: id}
	}
	return a
}

func (a *App) hasSession(id string) bool {
	a.termMu.Lock()
	defer a.termMu.Unlock()
	_, ok := a.sessions[id]
	return ok
}

func TestCloseSessionOnlyAffectsThatSession(t *testing.T) {
	a := newTestApp("s1", "s2", "s3")

	if err := a.CloseSession("s2"); err != nil {
		t.Fatalf("CloseSession 返回错误: %v", err)
	}

	if a.hasSession("s2") {
		t.Error("s2 应该已被移除")
	}
	for _, id := range []string{"s1", "s3"} {
		if !a.hasSession(id) {
			t.Errorf("%s 不该被 s2 的关闭波及", id)
		}
	}
}

func TestCloseSessionIsIdempotent(t *testing.T) {
	a := newTestApp("s1")

	for i := 0; i < 3; i++ {
		if err := a.CloseSession("s1"); err != nil {
			t.Fatalf("第 %d 次 CloseSession 返回错误: %v", i+1, err)
		}
	}
	// 前端关 Tab 后后端 Wait 协程还会再走一次收尾，不能 panic
	a.finalizeSession("s1")

	if a.hasSession("s1") {
		t.Error("s1 应该已被移除")
	}
}

func TestCloseUnknownSessionIsNoError(t *testing.T) {
	a := newTestApp()
	// 前端可能在收到 terminal-closed 之后才点关闭，这里必须容忍
	if err := a.CloseSession("nope"); err != nil {
		t.Errorf("关闭不存在的会话不应报错，得到: %v", err)
	}
}

func TestOperationsOnUnknownSessionReturnError(t *testing.T) {
	a := newTestApp("s1")

	if err := a.SendSessionInput("nope", "ls\n"); err == nil {
		t.Error("SendSessionInput 对未知会话应返回错误")
	}
	if err := a.ResizeSession("nope", 80, 24); err == nil {
		t.Error("ResizeSession 对未知会话应返回错误")
	}
}

func TestTerminalReadLoopFinalizesOnEOF(t *testing.T) {
	a := newTestApp("s1", "s2")

	a.terminalReadLoop("s1", strings.NewReader("welcome\n"), true)

	if a.hasSession("s1") {
		t.Error("stdout 读完后 s1 应该被收尾")
	}
	if !a.hasSession("s2") {
		t.Error("s1 的读循环结束不该影响 s2")
	}
}

func TestTerminalReadLoopNonStdoutDoesNotFinalize(t *testing.T) {
	a := newTestApp("s1")

	a.terminalReadLoop("s1", strings.NewReader("stderr noise"), false)

	if !a.hasSession("s1") {
		t.Error("非 stdout 的读循环结束不应关闭会话")
	}
}

func TestTerminalReadLoopNilReader(t *testing.T) {
	a := newTestApp("s1")
	a.terminalReadLoop("s1", nil, true)
	if !a.hasSession("s1") {
		t.Error("reader 为 nil 时应直接返回，不做收尾")
	}
}

func TestListActiveSessionsSkipsSessionsWithoutSFTP(t *testing.T) {
	a := newTestApp("s1", "s2")
	// 只有 SFTP 就绪的会话才能作为跨服务器传送的目标
	if got := len(a.ListActiveSessions()); got != 0 {
		t.Errorf("SFTP 未就绪时应为空，得到 %d 个", got)
	}
}

// 多窗口场景：大量会话同时开关，配合 -race 检查 sessions map 的并发安全。
func TestConcurrentSessionLifecycle(t *testing.T) {
	const n = 64
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("s%d", i)
	}
	a := newTestApp(ids...)

	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(4)
		go func() { defer wg.Done(); _ = a.CloseSession(id) }()
		go func() { defer wg.Done(); a.finalizeSession(id) }()
		go func() { defer wg.Done(); _ = a.ListActiveSessions() }()
		go func() { defer wg.Done(); _ = a.SendSessionInput(id, "x") }()
	}
	wg.Wait()

	a.termMu.Lock()
	left := len(a.sessions)
	a.termMu.Unlock()
	if left != 0 {
		t.Errorf("所有会话都应被清理，还剩 %d 个", left)
	}
}
