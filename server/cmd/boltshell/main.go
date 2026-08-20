package main // 程序入口包，生成的可执行文件从这里的 main 函数启动

import (
	"flag"          // 标准库：解析命令行参数
	"fmt"           // 标准库：格式化输出
	"os"            // 标准库：环境变量、标准输入输出、退出码等
	"path/filepath" // 标准库：路径拼接与解析

	"boltshell/internal/config"    // 本项目：配置加载（JSON）
	"boltshell/internal/db"        // 本项目：SQLite 数据库操作
	"boltshell/internal/logging"   // 本项目：简单日志封装
	"boltshell/internal/sshclient" // 本项目：SSH 连接和命令执行
)

func main() { // 主函数：根据参数决定启动 GUI 还是直接 SSH
	// 命令行参数定义（使用 flag 包）
	cfgPath := flag.String("config", "", "配置文件路径（JSON）")               // -config 指定配置文件路径
	verbose := flag.Bool("verbose", false, "开启详细日志")                   // -verbose 为 true 时使用 DEBUG 日志
	host := flag.String("host", "", "服务器地址或 IP")                       // -host 直连服务器地址
	user := flag.String("user", "", "用户名")                             // -user SSH 登录用户名
	pass := flag.String("pass", "", "密码")                              // -pass SSH 密码
	port := flag.Int("port", 0, "端口，默认 22")                            // -port 端口，不指定则后面用默认 22
	cmd := flag.String("cmd", "", "连接后执行的命令（可选）")                      // -cmd 指定要执行的远程命令
	shell := flag.Bool("shell", true, "默认进入交互式 Shell，可置为 false 仅执行命令") // -shell 控制是否进入交互式 shell
	flag.Parse()                                                       // 解析命令行参数，把值填入上面的指针中

	cfg := config.Default() // 从默认配置开始（可选项为空）
	if *cfgPath != "" {     // 如果用户通过 -config 显式指定了配置文件
		c, err := config.Load(*cfgPath) // 尝试从该路径读取配置
		if err == nil {
			cfg = c // 读取成功则覆盖默认配置
		} else {
			fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err) // 失败则打印错误，但继续使用默认配置
		}
	} else { // 未指定 -config 时，从默认位置寻找 config.json
		if exePath, err := os.Executable(); err == nil { // 获取当前可执行文件的路径
			exeDir := filepath.Dir(exePath) // 可执行文件所在目录
			// 优先从可执行文件所在目录加载 config.json
			if c, e := config.Load(filepath.Join(exeDir, "config.json")); e == nil {
				cfg = c
			} else if c2, e2 := config.Load("config.json"); e2 == nil { // 其次从当前工作目录加载 config.json
				cfg = c2
			}
		} else if c2, e2 := config.Load("config.json"); e2 == nil { // 获取可执行路径失败时，直接尝试当前目录
			cfg = c2
		}
	}

	// 根据配置和 -verbose 参数决定日志级别
	level := logging.Info // 默认 INFO 级别
	switch cfg.LogLevel {
	case "DEBUG":
		level = logging.Debug
	case "WARN":
		level = logging.Warn
	case "ERROR":
		level = logging.Error
	default:
		if *verbose { // 如果配置里没写，但命令行 -verbose=true，则强制用 DEBUG
			level = logging.Debug
		}
	}

	logger := logging.New(os.Stdout, level)                                         // 创建日志记录器，输出到标准输出
	logger.Info("BoltShell 启动")                                                     // 打一条启动日志
	logger.Info("配置: app=%s level=%s port=%d", cfg.AppName, cfg.LogLevel, cfg.Port) // 打当前配置概要

	// 打开数据库（连接信息存这里）
	dbPath := firstNonEmpty(os.Getenv("DB_PATH"), cfg.DBPath) // DB_PATH 环境变量优先，其次配置文件中的 DBPath
	d, err := db.Open(dbPath)                                 // 打开（或创建）SQLite 数据库
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库打开失败: %v\n", err)
		os.Exit(1) // 无法操作数据库就直接退出
	}
	defer d.Close()                          // main 结束前关闭数据库连接
	if err := db.InitSchema(d); err != nil { // 确保表结构已经创建好
		fmt.Fprintf(os.Stderr, "初始化数据库失败: %v\n", err)
		os.Exit(1)
	}

	// 从环境变量中读取 SSH 相关默认配置
	envHost := os.Getenv("SSH_HOST")
	envUser := os.Getenv("SSH_USER")
	envPass := os.Getenv("SSH_PASS")
	envPort := os.Getenv("SSH_PORT")

	// 计算最终要使用的 host/user/pass：
	// 优先级：命令行参数 > 环境变量 > 配置文件
	h := firstNonEmpty(*host, firstNonEmpty(envHost, cfg.Host))
	u := firstNonEmpty(*user, firstNonEmpty(envUser, cfg.User))
	p := firstNonEmpty(*pass, firstNonEmpty(envPass, cfg.Password))

	// 端口计算逻辑：命令行 > 环境变量 > 配置文件 > 默认 22
	pt := *port // 先用命令行 -port
	if pt == 0 {
		if envPort != "" { // 如果环境变量里有 SSH_PORT
			var ep int
			fmt.Sscanf(envPort, "%d", &ep) // 尝试解析为整数
			if ep > 0 {                    // 正数才有效
				pt = ep
			}
		}
		if cfg.Port != 0 { // 如果配置文件中指定了端口
			pt = cfg.Port
		}
	}

	// 未指定连接参数时提示使用 Wails 桌面客户端（旧 Gio GUI 已移除）
	if h == "" || u == "" || p == "" {
		fmt.Fprintln(os.Stderr, "未指定 SSH 连接参数。")
		fmt.Fprintln(os.Stderr, "请使用 Wails 桌面客户端 BoltShell.exe，或通过参数连接：")
		fmt.Fprintln(os.Stderr, "  boltshell -host 192.168.1.10 -user root -pass xxx")
		os.Exit(1)
	}

	// 如果指定 shell 模式并且没有指定 cmd，则进入交互式 shell（类似 ssh 命令）
	if *shell && *cmd == "" {
		if err := sshclient.Interactive(h, pt, u, p); err != nil { // 进入交互式 SSH 会话
			logger.Error("交互式会话失败: %v", err)
			os.Exit(1)
		}
		return
	}

	// 否则认为是非交互模式：直接执行指定 cmd（如果 cmd 为空，则只测试连接）
	res, err := sshclient.Run(h, pt, u, p, *cmd)
	if err != nil {
		logger.Error("连接或执行失败: %v", err)
		if res.Stdout != "" { // 如果远端有标准输出，先打印出来
			fmt.Fprintln(os.Stdout, res.Stdout)
		}
		if res.Stderr != "" { // 如果远端有错误输出，也打印出来
			fmt.Fprintln(os.Stderr, res.Stderr)
		}
		os.Exit(1) // 返回非零退出码
	}
	if res.Stdout != "" {
		fmt.Fprintln(os.Stdout, res.Stdout) // 正常执行成功时，只打印标准输出
	}
}

func firstNonEmpty(a, b string) string { // 返回第一个非空字符串；如果 a 为空则返回 b
	if a != "" {
		return a
	}
	return b
}
