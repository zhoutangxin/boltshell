// 远端系统信息采集：SSH 执行 shell 脚本解析 CPU/内存/磁盘/进程。
package main

import (
	"fmt"
	"strconv"
	"strings"

	"boltshell/internal/sshclient"
)

// SysProcInfo 进程占用摘要
type SysProcInfo struct {
	MemKB   int64   `json:"MemKB"`
	CPUPct  float64 `json:"CPUPct"`
	Command string  `json:"Command"`
}

// SysInfo 远端主机系统信息
type SysInfo struct {
	CPUPercent float64       `json:"CPUPercent"`
	MemTotal   int64         `json:"MemTotal"`
	MemUsed    int64         `json:"MemUsed"`
	SwapTotal  int64         `json:"SwapTotal"`
	SwapUsed   int64         `json:"SwapUsed"`
	DiskTotal  int64         `json:"DiskTotal"`
	DiskUsed   int64         `json:"DiskUsed"`
	DiskFree   int64         `json:"DiskFree"`
	DiskPath   string        `json:"DiskPath"`
	Processes  []SysProcInfo `json:"Processes"`
}

const sysInfoScript = `sh -c 'read cpu a b c d e f g h i j < /proc/stat; idle1=$((d+e)); total1=$((a+b+c+d+e+f+g+h)); sleep 1; read cpu a b c d e f g h i j < /proc/stat; idle2=$((d+e)); total2=$((a+b+c+d+e+f+g+h)); dt=$((total2-total1)); di=$((idle2-idle1)); if [ "$dt" -gt 0 ]; then cpu=$((100*(dt-di)/dt)); else cpu=0; fi; echo CPU:$cpu; free -b | awk "/Mem:/ {printf \"MEM:%s:%s:%s\\n\", \$2,\$3,\$4} /Swap:/ {printf \"SWAP:%s:%s\\n\", \$2,\$3}"; df -B1 / 2>/dev/null | tail -1 | awk "{printf \"DISK:%s:%s:%s:%s\\n\", \$2,\$3,\$4,\$6}"; ps -eo rss,pcpu,comm --no-headers --sort=-rss 2>/dev/null | head -5 | while read rss pcpu comm; do echo "PROC:$rss:$pcpu:$comm"; done'`

// GetSessionSysInfo 通过当前 SSH 会话采集远端系统信息
func (a *App) GetSessionSysInfo(sessionID string) (SysInfo, error) {
	a.termMu.Lock()
	h := a.sessions[sessionID]
	a.termMu.Unlock()
	if h == nil || h.term == nil {
		return SysInfo{}, fmt.Errorf("session not found")
	}
	client := h.term.SSHClient()
	if client == nil {
		return SysInfo{}, fmt.Errorf("ssh client not ready")
	}
	res, err := sshclient.RunOnClient(client, sysInfoScript)
	if err != nil && strings.TrimSpace(res.Stdout) == "" {
		return SysInfo{}, err
	}
	return parseSysInfoOutput(res.Stdout), nil
}

func parseSysInfoOutput(out string) SysInfo {
	info := SysInfo{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "CPU":
			info.CPUPercent = parseFloat(val)
		case "MEM":
			fields := strings.Split(val, ":")
			if len(fields) >= 3 {
				info.MemTotal = parseInt64(fields[0])
				info.MemUsed = parseInt64(fields[1])
			}
		case "SWAP":
			fields := strings.Split(val, ":")
			if len(fields) >= 2 {
				info.SwapTotal = parseInt64(fields[0])
				info.SwapUsed = parseInt64(fields[1])
			}
		case "DISK":
			fields := strings.Split(val, ":")
			if len(fields) >= 4 {
				info.DiskTotal = parseInt64(fields[0])
				info.DiskUsed = parseInt64(fields[1])
				info.DiskFree = parseInt64(fields[2])
				info.DiskPath = fields[3]
			}
		case "PROC":
			fields := strings.SplitN(val, ":", 3)
			if len(fields) >= 3 {
				info.Processes = append(info.Processes, SysProcInfo{
					MemKB:   parseInt64(fields[0]),
					CPUPct:  parseFloat(fields[1]),
					Command: fields[2],
				})
			}
		}
	}
	return info
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func parseFloat(s string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return n
}
