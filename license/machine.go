package license

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// 机器指纹：CPU ID + 主网卡 MAC。
// 授权码 / 授权文件都绑定这个指纹，换机器就失效。

var (
	cpuOnce sync.Once
	cpuVal  string
	cpuErr  error

	macOnce sync.Once
	macVal  string
	macErr  error
)

// Fingerprint 返回本机指纹：CPU ID 与主网卡 MAC。
func Fingerprint() (cpu, mac string, err error) {
	cpu, err = CPUID()
	if err != nil {
		return "", "", err
	}
	mac, err = PrimaryMAC()
	if err != nil {
		return "", "", err
	}
	return cpu, mac, nil
}

// MachineID 本机机器码，形如 3F2A-91C4-7B08-DE55。
func MachineID() (string, error) {
	cpu, mac, err := Fingerprint()
	if err != nil {
		return "", err
	}
	return MachineIDFrom(cpu, mac), nil
}

// MachineID 由 CPU / MAC 直接算机器码（授权中心用同一套算法）。
func MachineIDFrom(cpu, mac string) string {
	sum := sha256sum(normalize(cpu) + "|" + normalize(mac))
	hex := strings.ToUpper(fmt.Sprintf("%x", sum[:8]))
	return hex[0:4] + "-" + hex[4:8] + "-" + hex[8:12] + "-" + hex[12:16]
}

// Hostname 主机名，激活时一起上报，方便授权中心认机器。
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}

// CPUID 读取 CPU 序列号（同一台机器重启后不变）。
func CPUID() (string, error) {
	cpuOnce.Do(func() { cpuVal, cpuErr = detectCPUID() })
	return cpuVal, cpuErr
}

func detectCPUID() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// wmic 快，Win11 部分版本已移除，所以留 CIM 兜底。
		if out, ok := runCmd(4*time.Second, "wmic", "cpu", "get", "ProcessorId", "/value"); ok {
			if v := parseKeyValue(out, "ProcessorId"); v != "" {
				return v, nil
			}
		}
		if out, ok := runCmd(6*time.Second, "powershell", "-NoProfile", "-NonInteractive",
			"-Command", "(Get-CimInstance Win32_Processor | Select-Object -First 1).ProcessorId"); ok {
			if v := firstNonEmptyLine(out); v != "" {
				return v, nil
			}
		}
		// 最后退路：注册表里的 CPU 标识。同型号机器会相同，属降级方案。
		if out, ok := runCmd(4*time.Second, "reg", "query",
			`HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "/v", "Identifier"); ok {
			if v := parseKeyValue(out, "Identifier"); v != "" {
				return strings.ReplaceAll(v, " ", ""), nil
			}
		}
		return "", errors.New("读取 CPU ID 失败（wmic / CIM / 注册表均不可用）")

	case "linux":
		if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			fields := map[string]string{}
			for _, line := range strings.Split(string(b), "\n") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
			if v := fields["Serial"]; v != "" {
				return v, nil
			}
		}
		if b, err := os.ReadFile("/etc/machine-id"); err == nil {
			if v := firstNonEmptyLine(string(b)); v != "" {
				return v, nil
			}
		}
		return "", errors.New("读取 CPU ID 失败（/proc/cpuinfo 与 /etc/machine-id 均不可用）")

	case "darwin":
		if out, ok := runCmd(4*time.Second, "sh", "-c",
			`ioreg -rd1 -c IOPlatformExpertDevice | awk -F\" '/IOPlatformUUID/{print $4}'`); ok {
			if v := firstNonEmptyLine(out); v != "" {
				return v, nil
			}
		}
		return "", errors.New("读取 CPU ID 失败")

	default:
		return "", fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}
}

// PrimaryMAC 取主网卡 MAC：优先「物理网卡 + 有 IPv4 地址」，同时排除虚拟网卡。
func PrimaryMAC() (string, error) {
	macOnce.Do(func() { macVal, macErr = detectPrimaryMAC() })
	return macVal, macErr
}

type macCandidate struct {
	mac   string
	score int
	index int
}

func detectPrimaryMAC() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	cands := make([]macCandidate, 0, len(ifaces))
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(ifc.HardwareAddr) != 6 {
			continue
		}
		mac := strings.ToUpper(ifc.HardwareAddr.String())
		if mac == "00:00:00:00:00:00" {
			continue
		}

		score := 10
		if isVirtualNIC(ifc.Name) {
			score -= 100
		}
		if ifc.Flags&net.FlagUp != 0 {
			score += 3
		}
		if hasIPv4(ifc) {
			score += 6
		}
		cands = append(cands, macCandidate{mac: mac, score: score, index: ifc.Index})
	}

	if len(cands) == 0 {
		return "", errors.New("找不到可用的网卡 MAC")
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].index < cands[j].index
	})
	return cands[0].mac, nil
}

// AllMACs 列出所有候选网卡 MAC，排查「换了网卡导致授权失效」时用。
func AllMACs() []string {
	ifaces, _ := net.Interfaces()
	out := make([]string, 0, len(ifaces))
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 || len(ifc.HardwareAddr) != 6 {
			continue
		}
		out = append(out, fmt.Sprintf("%s(%s)", strings.ToUpper(ifc.HardwareAddr.String()), ifc.Name))
	}
	return out
}

func isVirtualNIC(name string) bool {
	n := strings.ToLower(name)
	for _, kw := range []string{
		"vmware", "virtualbox", "vbox", "hyper-v", "vethernet", "virtual",
		"tap", "tun", "docker", "br-", "veth", "wsl", "loopback",
		"bluetooth", "npcap", "zerotier", "tailscale", "wireguard", "vpn",
	} {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

func hasIPv4(ifc net.Interface) bool {
	addrs, err := ifc.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if v4 := ipnet.IP.To4(); v4 != nil && !v4.IsLoopback() && !v4.IsLinkLocalUnicast() {
				return true
			}
		}
	}
	return false
}

// ========================================
// 小工具
// ========================================

func runCmd(timeout time.Duration, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = hiddenWindow()
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// parseKeyValue 兼容 `Key=Value`（wmic /value）与 `Key    Value`（reg query）两种输出。
func parseKeyValue(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), key) {
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), key))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "REG_SZ"))
			if rest != "" {
				return rest
			}
		}
	}
	return ""
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line != "" {
			return line
		}
	}
	return ""
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToUpper(strings.TrimSpace(s))), "")
}
