package analytics

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const installFileName = "install.id"

var (
	installOnce sync.Once
	installID   string
	installErr  error
)

// InstallID 读取或生成匿名安装 ID（UUID 风格文本，重装会变）
func InstallID(userDataDir string) (string, error) {
	installOnce.Do(func() {
		installID, installErr = loadOrCreateInstallID(userDataDir)
	})
	return installID, installErr
}

func loadOrCreateInstallID(userDataDir string) (string, error) {
	path := filepath.Join(userDataDir, installFileName)
	if b, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(b))
		if id != "" {
			return id, nil
		}
	}
	id, err := newInstallID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func newInstallID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// UUID v4 变体位
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexStr := hex.EncodeToString(b[:])
	return hexStr[0:8] + "-" + hexStr[8:12] + "-" + hexStr[12:16] + "-" + hexStr[16:20] + "-" + hexStr[20:32], nil
}
