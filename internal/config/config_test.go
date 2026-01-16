package config // 与 config.go 同包，测试配置相关函数

import (
	"os"            // 标准库：文件写入
	"path/filepath" // 标准库：路径拼接
	"testing"       // 标准库：单元测试框架
)

// TestDefault 测试 Default 返回的配置是否包含必要字段
func TestDefault(t *testing.T) {
	d := Default()                           // 获取默认配置
	if d.AppName == "" || d.LogLevel == "" { // 必须至少有 AppName 和 LogLevel
		t.Fatalf("默认配置无效: %+v", d) // 测试失败时打印详细配置
	}
}

// TestLoad 测试 Load 函数是否能正确读取并解析 JSON 配置
func TestLoad(t *testing.T) {
	dir := t.TempDir()                                                // 为本次测试创建一个临时目录
	path := filepath.Join(dir, "cfg.json")                            // 在该目录下构造配置文件路径
	content := `{"appName":"myapp","logLevel":"DEBUG","port":22}`     // 测试用 JSON 内容
	if err := os.WriteFile(path, []byte(content), 0644); err != nil { // 写入测试文件
		t.Fatalf("写入测试配置失败: %v", err)
	}
	c, err := Load(path) // 通过 Load 读取配置
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	// 检查关键字段是否按预期解析
	if c.AppName != "myapp" || c.LogLevel != "DEBUG" || c.Port != 22 {
		t.Fatalf("配置解析不符合预期: %+v", c)
	}
}
