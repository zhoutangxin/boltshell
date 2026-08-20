package appdata

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dir 返回 BoltShell 用户数据目录（非 exe 旁，避免用户直接改文件）
// Windows: %LOCALAPPDATA%\BoltShell
// macOS:   ~/Library/Application Support/BoltShell
// Linux:   ~/.config/boltshell
func Dir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.Getenv("APPDATA")
		}
	case "darwin":
		base = os.Getenv("HOME")
		if base != "" {
			base = filepath.Join(base, "Library", "Application Support")
		}
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = os.Getenv("HOME")
			if base != "" {
				base = filepath.Join(base, ".config")
			}
		}
	}
	if base == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	}
	dir := filepath.Join(base, "BoltShell")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// IsDevMode 开发调试模式：仅此时允许 config.proLicensed / 明文绕过
func IsDevMode() bool {
	v := os.Getenv("BOLTSHELL_DEV")
	return v == "1" || v == "true" || v == "TRUE"
}
