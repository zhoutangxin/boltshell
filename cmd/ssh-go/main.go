package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ssh-go/internal/config"
	"ssh-go/internal/db"
	"ssh-go/internal/gui"
	"ssh-go/internal/logging"
	"ssh-go/internal/sshclient"
)

func main() {
	cfgPath := flag.String("config", "", "配置文件路径（JSON）")
	verbose := flag.Bool("verbose", false, "开启详细日志")
	host := flag.String("host", "", "服务器地址或 IP")
	user := flag.String("user", "", "用户名")
	pass := flag.String("pass", "", "密码")
	port := flag.Int("port", 0, "端口，默认 22")
	cmd := flag.String("cmd", "", "连接后执行的命令（可选）")
	shell := flag.Bool("shell", true, "默认进入交互式 Shell，可置为 false 仅执行命令")
	flag.Parse()

	cfg := config.Default()
	if *cfgPath != "" {
		c, err := config.Load(*cfgPath)
		if err == nil {
			cfg = c
		} else {
			fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		}
	} else {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			if c, e := config.Load(filepath.Join(exeDir, "config.json")); e == nil {
				cfg = c
			} else if c2, e2 := config.Load("config.json"); e2 == nil {
				cfg = c2
			}
		} else if c2, e2 := config.Load("config.json"); e2 == nil {
			cfg = c2
		}
	}

	level := logging.Info
	switch cfg.LogLevel {
	case "DEBUG":
		level = logging.Debug
	case "WARN":
		level = logging.Warn
	case "ERROR":
		level = logging.Error
	default:
		if *verbose {
			level = logging.Debug
		}
	}

	logger := logging.New(os.Stdout, level)
	logger.Info("ssh-go 启动")
	logger.Info("配置: app=%s level=%s port=%d", cfg.AppName, cfg.LogLevel, cfg.Port)

	dbPath := firstNonEmpty(os.Getenv("DB_PATH"), cfg.DBPath)
	d, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库打开失败: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()
	if err := db.InitSchema(d); err != nil {
		fmt.Fprintf(os.Stderr, "初始化数据库失败: %v\n", err)
		os.Exit(1)
	}

	envHost := os.Getenv("SSH_HOST")
	envUser := os.Getenv("SSH_USER")
	envPass := os.Getenv("SSH_PASS")
	envPort := os.Getenv("SSH_PORT")

	h := firstNonEmpty(*host, firstNonEmpty(envHost, cfg.Host))
	u := firstNonEmpty(*user, firstNonEmpty(envUser, cfg.User))
	p := firstNonEmpty(*pass, firstNonEmpty(envPass, cfg.Password))
	pt := *port
	if pt == 0 {
		if envPort != "" {
			var ep int
			fmt.Sscanf(envPort, "%d", &ep)
			if ep > 0 {
				pt = ep
			}
		}
		if cfg.Port != 0 {
			pt = cfg.Port
		}
	}
	if h == "" || u == "" || p == "" {
		if err := gui.Start(d, logger); err != nil {
			fmt.Fprintf(os.Stderr, "GUI 异常: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *shell && *cmd == "" {
		if err := sshclient.Interactive(h, pt, u, p); err != nil {
			logger.Error("交互式会话失败: %v", err)
			os.Exit(1)
		}
		return
	}
	res, err := sshclient.Run(h, pt, u, p, *cmd)
	if err != nil {
		logger.Error("连接或执行失败: %v", err)
		if res.Stdout != "" {
			fmt.Fprintln(os.Stdout, res.Stdout)
		}
		if res.Stderr != "" {
			fmt.Fprintln(os.Stderr, res.Stderr)
		}
		os.Exit(1)
	}
	if res.Stdout != "" {
		fmt.Fprintln(os.Stdout, res.Stdout)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
