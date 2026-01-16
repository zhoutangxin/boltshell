package config // 配置相关逻辑：读取 JSON 配置文件等

import (
	"encoding/json" // 标准库：JSON 编码/解码
	"os"            // 标准库：文件读写等
)

// Config 表示配置文件中的所有可配置项
type Config struct {
	AppName  string `json:"appName"`  // 应用名称（目前主要用于日志打印）
	LogLevel string `json:"logLevel"` // 日志级别：DEBUG/INFO/WARN/ERROR
	Port     int    `json:"port"`     // 连接端口（为 0 时使用默认 22 或其它逻辑）
	Host     string `json:"host"`     // 默认 SSH 主机
	User     string `json:"user"`     // 默认 SSH 用户名
	Password string `json:"password"` // 默认 SSH 密码
	DBPath   string `json:"dbPath"`   // 数据库文件路径（为空时使用默认 data.db）
}

// Default 返回一份默认配置，用于没有配置文件或配置不完整的情况
func Default() Config {
	return Config{
		AppName:  "ssh-go", // 默认应用名
		LogLevel: "INFO",   // 默认日志级别 INFO
		Port:     0,        // 端口 0 表示“未配置”，后续会用默认 22 或环境变量覆盖
		Host:     "",       // 默认不指定主机
		User:     "",       // 默认不指定用户
		Password: "",       // 默认不指定密码
		DBPath:   "",       // 默认不指定 DB 路径，后续会选择默认路径
	}
}

// Load 从指定路径读取 JSON 配置文件，并返回解析后的 Config
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path) // 读取整个文件内容
	if err != nil {
		return Config{}, err // 读文件失败，返回空配置和错误
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil { // 解析 JSON 到 Config 结构体
		return Config{}, err
	}
	// 一些字段如果为空，则使用合理的默认值
	if c.AppName == "" {
		c.AppName = "ssh-go"
	}
	if c.LogLevel == "" {
		c.LogLevel = "INFO"
	}
	return c, nil
}
