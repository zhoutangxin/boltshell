package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"boltshell/internal/config"
	"boltshell/internal/db"
	"boltshell/internal/logging"
	"boltshell/internal/sftpclient"
	"boltshell/internal/sshclient"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	db        *sql.DB
	logger    *logging.Logger
	termMu    sync.Mutex
	sessions  map[string]*terminalHolder
}

func NewApp() *App {
	return &App{
		sessions: make(map[string]*terminalHolder),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.initBackend(); err != nil {
		// runtime.Log* 不会中断程序，这里至少把错误打印到控制台
		runtime.LogError(a.ctx, fmt.Sprintf("backend init failed: %v", err))
	}
}

type terminalHolder struct {
	connID   string
	title    string
	term     *sshclient.TerminalSession
	sftp     *sftpclient.Client
	finalize sync.Once
}

func (a *App) initBackend() error {
	// 读取配置：你现有的 config.json 规则这里复用
	cfg := config.Default()
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if c, err2 := config.Load(filepath.Join(exeDir, "config.json")); err2 == nil {
			cfg = c
		} else if c2, e2 := config.Load("config.json"); e2 == nil {
			cfg = c2
		}
	} else if c, err := config.Load("config.json"); err == nil {
		cfg = c
	}

	dbPath := firstNonEmptyFromConfigEnv(cfg)
	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	a.db = d
	if err := db.InitSchema(a.db); err != nil {
		_ = a.db.Close()
		return err
	}

	// 日志级别
	level := logging.Info
	switch cfg.LogLevel {
	case "DEBUG":
		level = logging.Debug
	case "WARN":
		level = logging.Warn
	case "ERROR":
		level = logging.Error
	}
	a.logger = logging.New(os.Stdout, level)
	a.logger.Info("BoltShell (wails) backend init ok")
	return nil
}

func firstNonEmptyFromConfigEnv(cfg config.Config) string {
	// 兼容 cmd/boltshell/main.go 的逻辑：环境变量优先其次 config
	if v := os.Getenv("DB_PATH"); v != "" {
		return v
	}
	return cfg.DBPath
}

// ListConnections: 获取连接列表
func (a *App) ListConnections(includeDeleted bool, groupFilter string) ([]db.Connection, error) {
	if a.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	return db.ListConnections(a.db, includeDeleted, groupFilter)
}

// AddConnection: 新增一条连接记录
func (a *App) AddConnection(name string, host string, port int, user string, password string, groupName string, enabled bool) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("db not initialized")
	}
	if host == "" || user == "" || password == "" {
		return "", fmt.Errorf("缺少必填项")
	}
	p := port
	if p < 0 {
		p = 0
	}

	id := db.NewID()
	en := 0
	if enabled {
		en = 1
	}

	err := db.InsertConnection(a.db, db.Connection{
		ID:        id,
		Name:      name,
		Host:      host,
		Port:      p,
		User:      user,
		Password:  password,
		GroupName: groupName,
		Enabled:   en,
		Deleted:   0,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpdateConnection: 修改已有连接
func (a *App) UpdateConnection(id string, name string, host string, port int, user string, password string, groupName string, enabled bool) error {
	if a.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if id == "" {
		return fmt.Errorf("缺少连接 ID")
	}
	if host == "" || user == "" || password == "" {
		return fmt.Errorf("缺少必填项")
	}
	p := port
	if p < 0 {
		p = 0
	}
	en := 0
	if enabled {
		en = 1
	}
	return db.UpdateConnection(a.db, db.Connection{
		ID:        id,
		Name:      name,
		Host:      host,
		Port:      p,
		User:      user,
		Password:  password,
		GroupName: groupName,
		Enabled:   en,
	})
}

// SetDeleted: 设置逻辑删除状态（0=正常，1=已删除）
func (a *App) SetDeleted(connID string, deleted bool) error {
	if a.db == nil {
		return fmt.Errorf("db not initialized")
	}
	d := 0
	if deleted {
		d = 1
	}
	return db.SetDeleted(a.db, connID, d)
}

// StartSession: 创建一个 SSH 终端会话，并开始异步推送输出到前端
func (a *App) StartSession(connID string) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("db not initialized")
	}

	c, err := db.GetByID(a.db, connID)
	if err != nil {
		return "", err
	}
	if c.Deleted == 1 {
		return "", fmt.Errorf("连接已删除")
	}
	if c.Enabled == 0 {
		return "", fmt.Errorf("连接未启用")
	}

	// 每次连接都创建独立会话，支持多 Tab 同时保持
	term, err := sshclient.NewTerminalSession(c.Host, c.Port, c.User, c.Password, 120, 32)
	if err != nil {
		return "", err
	}

	sid := db.NewID()
	title := c.Name
	if title == "" {
		title = fmt.Sprintf("%s:%d", c.Host, c.Port)
	}

	a.termMu.Lock()
	h := &terminalHolder{
		connID: connID,
		title:  title,
		term:   term,
	}
	if sftpClient, sftpErr := sftpclient.NewFromSSH(term.SSHClient()); sftpErr == nil {
		h.sftp = sftpClient
	} else if a.logger != nil {
		a.logger.Info("sftp init failed id=%s err=%v", sid, sftpErr)
	}
	a.sessions[sid] = h
	a.termMu.Unlock()

	if a.logger != nil {
		a.logger.Info("session started id=%s host=%s:%d", sid, c.Host, c.Port)
	}

	go a.terminalReadLoop(sid, term.Stdout, true)
	if term.Stderr != nil {
		go a.terminalReadLoop(sid, term.Stderr, false)
	}
	go func() {
		err := term.Wait()
		if a.logger != nil {
			a.logger.Info("session wait done id=%s err=%v", sid, err)
		}
		a.finalizeSession(sid)
	}()
	return sid, nil
}

func (a *App) finalizeSession(sessionID string) {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	if h == nil {
		a.termMu.Unlock()
		return
	}
	a.termMu.Unlock()

	h.finalize.Do(func() {
		a.termMu.Lock()
		delete(a.sessions, sessionID)
		a.termMu.Unlock()
		if h.term != nil {
			_ = h.term.Close()
		}
		if h.sftp != nil {
			_ = h.sftp.Close()
		}
		runtime.EventsEmit(a.ctx, "terminal-closed", sessionID)
	})
}

func (a *App) terminalReadLoop(sessionID string, r io.Reader, isStdout bool) {
	if r == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			text := decodeRemote(chunk)
			if a.logger != nil {
				a.logger.Debug("terminal-output id=%s n=%d", sessionID, n)
			}
			runtime.EventsEmit(a.ctx, "terminal-output", sessionID, text)
		}
		if err != nil {
			if isStdout {
				a.finalizeSession(sessionID)
			}
			return
		}
	}
}

// ResizeSession: 前端 xterm 尺寸变化时同步远端 PTY
func (a *App) ResizeSession(sessionID string, cols int, rows int) error {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	a.termMu.Unlock()
	if h == nil || h.term == nil {
		return fmt.Errorf("session not found")
	}
	return h.term.Resize(cols, rows)
}

// SendSessionInput: 前端把 xterm.js 的输入数据写回 SSH
func (a *App) SendSessionInput(sessionID string, data string) error {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	a.termMu.Unlock()
	if h == nil || h.term == nil || h.term.Stdin == nil {
		return fmt.Errorf("session not found")
	}
	_, err := h.term.Stdin.Write([]byte(data))
	return err
}

// CloseSession: 主动关闭终端
func (a *App) CloseSession(sessionID string) error {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	if h == nil {
		a.termMu.Unlock()
		return nil
	}
	a.termMu.Unlock()
	var err error
	h.finalize.Do(func() {
		a.termMu.Lock()
		delete(a.sessions, sessionID)
		a.termMu.Unlock()
		if h.term != nil {
			err = h.term.Close()
		}
		if h.sftp != nil {
			_ = h.sftp.Close()
		}
	})
	return err
}

func decodeRemote(b []byte) string {
	// 复用你 Gio 里对 GB18030 -> UTF-8 的兜底逻辑，让终端显示尽量正常
	if isUTF8(b) {
		return string(b)
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err == nil && isUTF8(decoded) {
		return string(decoded)
	}
	return string(b)
}

func isUTF8(b []byte) bool {
	// 简单判定：如果存在非法序列会返回错误。这里用 Go 的标准 utf8 校验够用。
	// 为了避免再引入额外依赖，直接用内置 unicode/utf8。
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		b = b[size:]
	}
	return true
}
