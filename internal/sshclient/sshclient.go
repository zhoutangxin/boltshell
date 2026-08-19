package sshclient

import (
	"bytes" // 用于缓冲命令执行的标准输出和错误输出
	"fmt"   // 字符串格式化和错误信息构造
	"io"    // 定义通用的 Reader / Writer 接口
	"net"   // TCP 连接与 KeepAlive
	"os"    // 访问标准输入输出、文件描述符等
	"time"  // 超时时间设置

	"golang.org/x/crypto/ssh" // SSH 客户端库
	"golang.org/x/term"       // 终端模式控制（raw 模式）
)

type Result struct { // Run 函数执行命令后的结果
	Stdout string // 标准输出内容
	Stderr string // 标准错误内容
}

type TerminalSession struct { // GUI 使用的 SSH 终端会话结构
	client  *ssh.Client    // SSH 客户端连接对象
	session *ssh.Session   // 当前交互 Session
	Stdin   io.WriteCloser // 远端终端的标准输入（写入这里即可发送到远端）
	Stdout  io.Reader      // 远端终端的标准输出（从这里读取返回内容）
	Stderr  io.Reader      // 远端终端的标准错误输出
}

func Interactive(host string, port int, user, pass string) error { // 在当前控制台中打开交互式 SSH shell
	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port)) // 若端口为 0 则使用默认 22 端口
	cfg := &ssh.ClientConfig{                            // SSH 连接配置
		User:            user,                                 // 用户名
		Auth:            []ssh.AuthMethod{ssh.Password(pass)}, // 使用密码认证
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),          // 不校验主机公钥（简单起见）
		Timeout:         10 * time.Second,                     // 连接超时时间 10 秒
	}
	c, err := dialSSH(addr, cfg) // 建立到远端的 SSH 连接（含 TCP KeepAlive）
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSession() // 在连接上创建一个新的会话
	if err != nil {
		return err
	}
	defer s.Close()
	w, h := 80, 24                                                 // 默认终端宽高（列数、行数）
	if szW, szH, e := term.GetSize(int(os.Stdin.Fd())); e == nil { // 如果能获取当前控制台尺寸则优先使用
		if szW > 0 {
			w = szW
		}
		if szH > 0 {
			h = szH
		}
	}
	modes := ssh.TerminalModes{ // 终端模式配置
		ssh.ECHO:          1,     // 开启回显
		ssh.TTY_OP_ISPEED: 14400, // 输入波特率
		ssh.TTY_OP_OSPEED: 14400, // 输出波特率
	}
	if err := s.RequestPty("xterm-256color", h, w, modes); err != nil { // 请求一个伪终端（PTY）
		return err
	}
	s.Stdin = os.Stdin                // 会话标准输入绑定到本地 stdin
	s.Stdout = os.Stdout              // 会话标准输出绑定到本地 stdout
	s.Stderr = os.Stderr              // 会话标准错误绑定到本地 stderr
	restore := setRaw()               // 把当前控制台切换到 raw 模式
	defer restore()                   // 函数退出时恢复原来的终端模式
	if err := s.Shell(); err != nil { // 启动远端 shell
		return err
	}
	return s.Wait()
}

func NewTerminalSession(host string, port int, user, pass string, width, height int) (*TerminalSession, error) { // 为 GUI 创建一个可读写的终端会话
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port)) // 远端地址
	cfg := &ssh.ClientConfig{                            // SSH 客户端配置
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := dialSSH(addr, cfg) // 建立 SSH 连接（含 TCP KeepAlive）
	if err != nil {
		return nil, err
	}
	s, err := c.NewSession() // 创建一个新的会话
	if err != nil {
		c.Close()
		return nil, err
	}
	stdin, err := s.StdinPipe() // 获取远端会话的标准输入管道
	if err != nil {
		s.Close()
		c.Close()
		return nil, err
	}
	stdout, err := s.StdoutPipe() // 获取远端会话的标准输出管道
	if err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	stderr, err := s.StderrPipe() // 获取远端会话的标准错误管道
	if err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	modes := ssh.TerminalModes{ // 设置终端模式
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := s.RequestPty("xterm-256color", height, width, modes); err != nil { // 申请伪终端
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	// Shell() 必须同步启动成功后再返回。若放到 goroutine，后续 Wait() 会抢跑并立刻关掉会话。
	if err := s.Shell(); err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	return &TerminalSession{
		client:  c,
		session: s,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

// SSHClient 返回底层 SSH 连接，供 SFTP 等同连接复用
func (t *TerminalSession) SSHClient() *ssh.Client {
	if t == nil {
		return nil
	}
	return t.client
}

func (t *TerminalSession) Close() error { // 关闭终端会话及其底层 SSH 连接
	if t == nil {
		return nil
	}
	if t.session != nil { // 先尝试关闭 Session
		_ = t.session.Close()
	}
	if t.client != nil { // 再关闭底层客户端连接
		return t.client.Close()
	}
	return nil
}

// Wait 阻塞直到远端 shell 会话结束
func (t *TerminalSession) Wait() error {
	if t == nil || t.session == nil {
		return nil
	}
	return t.session.Wait()
}

// Resize 同步远端 PTY 尺寸（切换 Tab / 窗口缩放时调用）
func (t *TerminalSession) Resize(cols, rows int) error {
	if t == nil || t.session == nil {
		return fmt.Errorf("session not ready")
	}
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	return t.session.WindowChange(rows, cols)
}

func dialSSH(addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func setRaw() func() { // 把当前控制台切换到 raw 模式，并返回恢复函数
	fd := int(os.Stdin.Fd())           // 标准输入的文件描述符
	oldState, err := term.GetState(fd) // 记录当前终端状态，稍后用于恢复
	if err != nil {
		return func() {}
	}
	if _, err := term.MakeRaw(fd); err != nil { // 切换到 raw 模式（不做行缓冲和本地编辑）
		return func() {}
	}
	return func() { _ = term.Restore(fd, oldState) } // 返回一个闭包，在需要时恢复原状态
}

func Connect(host string, port int, user, pass string) error { // 只测试是否能成功建立 SSH 连接
	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port)) // 远端地址
	cfg := &ssh.ClientConfig{                            // 构造 SSH 配置
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := dialSSH(addr, cfg) // 建立 SSH 连接（含 TCP KeepAlive）
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSession() // 打开一个会话
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Run("true"); err != nil { // 简单执行一个永远成功的命令用于验证
		return err
	}
	return nil
}

// RunOnClient 在已有 SSH 连接上执行命令（不占用交互式 shell 会话）
func RunOnClient(client *ssh.Client, cmd string) (Result, error) {
	if client == nil {
		return Result{}, fmt.Errorf("client nil")
	}
	if cmd == "" {
		return Result{}, fmt.Errorf("命令为空")
	}
	s, err := client.NewSession()
	if err != nil {
		return Result{}, err
	}
	defer s.Close()
	var outBuf, errBuf bytes.Buffer
	s.Stdout = &outBuf
	s.Stderr = &errBuf
	if err := s.Run(cmd); err != nil {
		return Result{Stdout: outBuf.String(), Stderr: errBuf.String()}, err
	}
	return Result{Stdout: outBuf.String(), Stderr: errBuf.String()}, nil
}

func Run(host string, port int, user, pass, cmd string) (Result, error) { // 在远端执行单条命令并返回输出
	if host == "" || user == "" || pass == "" {
		return Result{}, fmt.Errorf("参数不完整")
	}
	if cmd == "" { // 如果命令为空，仅测试连接是否正常
		if err := Connect(host, port, user, pass); err != nil {
			return Result{}, err
		}
		return Result{Stdout: "连接成功", Stderr: ""}, nil
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port)) // 远端地址
	cfg := &ssh.ClientConfig{                            // SSH 配置
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg) // 建立连接
	if err != nil {
		return Result{}, err
	}
	defer c.Close()
	s, err := c.NewSession() // 打开会话
	if err != nil {
		return Result{}, err
	}
	defer s.Close()
	var outBuf, errBuf bytes.Buffer // 使用 bytes.Buffer 累积标准输出和错误输出
	s.Stdout = &outBuf
	s.Stderr = &errBuf
	if err := s.Run(cmd); err != nil { // 执行命令，如果出错也返回已获取到的输出
		return Result{Stdout: outBuf.String(), Stderr: errBuf.String()}, err
	}
	return Result{Stdout: outBuf.String(), Stderr: errBuf.String()}, nil
}

func portIfZero(p int) int { // 如果端口为 0，返回默认 SSH 端口 22
	if p == 0 {
		return 22
	}
	return p
}
