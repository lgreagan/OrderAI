// Package license 提供简易机器绑定授权。
//
// 思路：把服务器「CPU ID + 主网卡 MAC」拼成机器指纹，用内置密钥做 HMAC-SHA256，
// 取摘要前 80bit 编成 ORDAI1-XXXX-XXXX-XXXX-XXXX 形式的授权码。
// 授权码写死在 main.go，启动时用本机指纹现算一遍做比对，不匹配就拒绝启动。
//
// 注意：这是「防呆级」授权，只防把程序连同数据库拷到另一台机器直接跑，
// 不防逆向（源码都在手上，改掉 main.go 里的校验就绕过了）。
package license

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
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

// 授权码格式：ORDAI1-A1B2-C3D4-E5F6-G7H8
const (
	codePrefix = "ORDAI1"
	// Crockford Base32：去掉 I / L / O / U，避免和 1 / 0 看混。
	alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// MismatchError 授权码与本机指纹不匹配。
type MismatchError struct {
	CPU string
	MAC string
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf("授权码与本机不匹配（CPU=%s MAC=%s）", e.CPU, e.MAC)
}

// ========================================
// 机器指纹
// ========================================

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

// MachineLine 单行指纹，便于日志与人工核对。
func MachineLine() string {
	cpu, mac, err := Fingerprint()
	if err != nil {
		return "读取机器指纹失败: " + err.Error()
	}
	return fmt.Sprintf("CPU=%s MAC=%s", cpu, mac)
}

// CPUID 读取 CPU 序列号（同一台机器重启后不变）。
func CPUID() (string, error) {
	cpuOnce.Do(func() { cpuVal, cpuErr = detectCPUID() })
	return cpuVal, cpuErr
}

func detectCPUID() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// wmic 快，Win11 部分版本已移除，所以留 PowerShell CIM 兜底。
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
		return "", errors.New("读取 CPU ID 失败（wmic / PowerShell / 注册表均不可用）")

	case "linux":
		if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			fields := map[string]string{}
			for _, line := range strings.Split(string(b), "\n") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
			for _, k := range []string{"Serial", "processor"} {
				if v := fields[k]; v != "" && k == "Serial" {
					return v, nil
				}
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

// AllMACs 列出所有候选网卡 MAC，用于换机器时人工挑选。
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
// 授权码
// ========================================

// Code 用密钥和机器指纹生成授权码。
func Code(secret, cpu, mac string) string {
	msg := normalize(cpu) + "|" + normalize(mac)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(msg))
	sum := h.Sum(nil) // 32 字节，取前 10 字节 = 80bit = 16 个 base32 字符
	raw := encode32(sum[:10])
	return codePrefix + "-" + raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
}

// Verify 核对授权码是否与本机匹配。
func Verify(secret, want string) error {
	if strings.TrimSpace(want) == "" {
		return errors.New("未配置授权码（main.go 的 licenseCode 为空）")
	}
	cpu, mac, err := Fingerprint()
	if err != nil {
		return err
	}
	got := Code(secret, cpu, mac)
	if !sameCode(got, want) {
		return &MismatchError{CPU: cpu, MAC: mac}
	}
	return nil
}

func sameCode(a, b string) bool {
	a, b = extract(a), extract(b)
	if len(a) != len(b) || a == "" {
		return false
	}
	// 常量时间比较，避免时序侧信道（象征意义，成本极低）
	diff := 0
	for i := 0; i < len(a); i++ {
		diff |= int(a[i] ^ b[i])
	}
	return diff == 0
}

// extract 只保留授权码本体（去掉前缀、横线、空格，统一大写）。
func extract(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimPrefix(s, codePrefix)
	return s
}

func normalize(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), "")
}

func encode32(b []byte) string {
	var sb strings.Builder
	sb.Grow((len(b)*8 + 4) / 5)
	var buf uint32
	var bits uint
	for _, x := range b {
		buf = buf<<8 | uint32(x)
		bits += 8
		for bits >= 5 {
			bits -= 5
			sb.WriteByte(alphabet[(buf>>bits)&0x1f])
		}
	}
	if bits > 0 {
		sb.WriteByte(alphabet[(buf<<(5-bits))&0x1f])
	}
	return sb.String()
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
