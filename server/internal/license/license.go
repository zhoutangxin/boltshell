package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"boltshell/internal/appdata"
)

const licenseFileName = "license.dat"

var errBadLicense = errors.New("invalid license")

type payload struct {
	Email       string `json:"email"`
	Plan        string `json:"plan"`
	ExpiresAt   int64  `json:"expiresAt"`
	IssuedAt    int64  `json:"issuedAt"`
	MachineHint string `json:"machineHint,omitempty"`
}

type licenseFile struct {
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}

// IsPro 是否 Pro（去广告 + 付费功能）
func IsPro(devConfigFlag bool) bool {
	if appdata.IsDevMode() {
		if v := strings.TrimSpace(os.Getenv("BOLTSHELL_PRO")); v == "1" || strings.EqualFold(v, "true") {
			return true
		}
		return devConfigFlag
	}
	ok, _ := verifyFile()
	return ok
}

func verifyFile() (bool, error) {
	dir, err := appdata.Dir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(dir, licenseFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var lf licenseFile
	if err := json.Unmarshal(b, &lf); err != nil {
		return false, errBadLicense
	}
	if lf.Payload == "" || lf.Sig == "" {
		return false, errBadLicense
	}
	if !hmac.Equal([]byte(lf.Sig), []byte(signPayload(lf.Payload))) {
		return false, errBadLicense
	}
	raw, err := base64.StdEncoding.DecodeString(lf.Payload)
	if err != nil {
		return false, errBadLicense
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false, errBadLicense
	}
	if p.Plan != "pro" && p.Plan != "team" {
		return false, errBadLicense
	}
	if p.ExpiresAt > 0 && time.Now().Unix() > p.ExpiresAt {
		return false, nil
	}
	return true, nil
}

func signPayload(payloadB64 string) string {
	mac := hmac.New(sha256.New, licensePepper())
	mac.Write([]byte(payloadB64))
	return hexEncode(mac.Sum(nil))
}

func licensePepper() []byte {
	return []byte("boltshell-license-v1-pepper-change-before-pro-launch")
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}

// SaveLicenseForDev 仅 BOLTSHELL_DEV=1 时可用于本地测试 Pro
func SaveLicenseForDev(plan string) error {
	if !appdata.IsDevMode() {
		return errors.New("only available in BOLTSHELL_DEV mode")
	}
	p := payload{
		Email:     "dev@local",
		Plan:      plan,
		ExpiresAt: 0,
		IssuedAt:  time.Now().Unix(),
	}
	raw, _ := json.Marshal(p)
	payloadB64 := base64.StdEncoding.EncodeToString(raw)
	lf := licenseFile{Payload: payloadB64, Sig: signPayload(payloadB64)}
	b, _ := json.MarshalIndent(lf, "", "  ")
	dir, err := appdata.Dir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, licenseFileName), b, 0o600)
}
