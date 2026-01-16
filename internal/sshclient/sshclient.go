package sshclient

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Result struct {
	Stdout string
	Stderr string
}

type TerminalSession struct {
	client  *ssh.Client
	session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader
}

func Interactive(host string, port int, user, pass string) error {
	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port))
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	w, h := 80, 24
	if szW, szH, e := term.GetSize(int(os.Stdin.Fd())); e == nil {
		if szW > 0 {
			w = szW
		}
		if szH > 0 {
			h = szH
		}
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := s.RequestPty("xterm-256color", h, w, modes); err != nil {
		return err
	}
	s.Stdin = os.Stdin
	s.Stdout = os.Stdout
	s.Stderr = os.Stderr
	restore := setRaw()
	defer restore()
	if err := s.Shell(); err != nil {
		return err
	}
	return s.Wait()
}

func NewTerminalSession(host string, port int, user, pass string, width, height int) (*TerminalSession, error) {
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port))
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	s, err := c.NewSession()
	if err != nil {
		c.Close()
		return nil, err
	}
	stdin, err := s.StdinPipe()
	if err != nil {
		s.Close()
		c.Close()
		return nil, err
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	stderr, err := s.StderrPipe()
	if err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := s.RequestPty("xterm-256color", height, width, modes); err != nil {
		stdin.Close()
		s.Close()
		c.Close()
		return nil, err
	}
	go func() { _ = s.Shell() }()
	return &TerminalSession{
		client:  c,
		session: s,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

func (t *TerminalSession) Close() error {
	if t == nil {
		return nil
	}
	if t.session != nil {
		_ = t.session.Close()
	}
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}

func setRaw() func() {
	fd := int(os.Stdin.Fd())
	oldState, err := term.GetState(fd)
	if err != nil {
		return func() {}
	}
	if _, err := term.MakeRaw(fd); err != nil {
		return func() {}
	}
	return func() { _ = term.Restore(fd, oldState) }
}

func Connect(host string, port int, user, pass string) error {
	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("参数不完整")
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port))
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Run("true"); err != nil {
		return err
	}
	return nil
}

func Run(host string, port int, user, pass, cmd string) (Result, error) {
	if host == "" || user == "" || pass == "" {
		return Result{}, fmt.Errorf("参数不完整")
	}
	if cmd == "" {
		if err := Connect(host, port, user, pass); err != nil {
			return Result{}, err
		}
		return Result{Stdout: "连接成功", Stderr: ""}, nil
	}
	addr := fmt.Sprintf("%s:%d", host, portIfZero(port))
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return Result{}, err
	}
	defer c.Close()
	s, err := c.NewSession()
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

func portIfZero(p int) int {
	if p == 0 {
		return 22
	}
	return p
}
