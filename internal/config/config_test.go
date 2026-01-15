package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.AppName == "" || d.LogLevel == "" {
		t.Fatalf("默认配置无效: %+v", d)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	content := `{"appName":"myapp","logLevel":"DEBUG","port":22}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	if c.AppName != "myapp" || c.LogLevel != "DEBUG" || c.Port != 22 {
		t.Fatalf("配置解析不符合预期: %+v", c)
	}
}
