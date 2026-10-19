package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Secret 签名密钥。客户端与授权中心（tools/licserver）必须一致。
// 换掉它 = 已发出的授权文件全部失效。
const Secret = "OrderAI!2026@根#license$7f3a9c"

// 授权状态错误。页面据此显示不同提示。
var (
	ErrNotFound        = errors.New("本机尚未激活")
	ErrExpired         = errors.New("授权已过期")
	ErrMachineMismatch = errors.New("授权不属于本机")
	ErrTampered        = errors.New("授权文件已损坏或被改动")
)

// License 授权文件内容。Token 为签名，机器码/客户/时间任一被改都会验签失败。
type License struct {
	Machine   string `json:"machine"`    // 机器码
	Customer  string `json:"customer"`   // 客户/使用单位
	IssuedAt  int64  `json:"issued_at"`  // 签发时间（unix 秒）
	ExpiresAt int64  `json:"expires_at"` // 到期时间（unix 秒），0 = 永久
	Token     string `json:"token"`      // HMAC 签名
}

// Sign 按当前内容重算签名。
func (l *License) Sign() {
	l.Token = signOf(l)
}

// Expired 是否已过期（永久授权恒为 false）。
func (l *License) Expired() bool {
	return l.ExpiresAt > 0 && time.Now().Unix() > l.ExpiresAt
}

// ExpiresText 到期日文案。
func (l *License) ExpiresText() string {
	if l.ExpiresAt <= 0 {
		return "永久"
	}
	return time.Unix(l.ExpiresAt, 0).Format("2006-01-02")
}

// DaysLeft 剩余天数（永久返回 -1）。
func (l *License) DaysLeft() int {
	if l.ExpiresAt <= 0 {
		return -1
	}
	d := int((l.ExpiresAt - time.Now().Unix()) / 86400)
	if d < 0 {
		d = 0
	}
	return d
}

// VerifyLocal 校验：机器码匹配 + 签名正确 + 未过期。
func (l *License) VerifyLocal(machine string) error {
	if l == nil {
		return ErrNotFound
	}
	if !strings.EqualFold(strings.TrimSpace(l.Machine), strings.TrimSpace(machine)) {
		return ErrMachineMismatch
	}
	if !hmac.Equal([]byte(strings.ToLower(l.Token)), []byte(strings.ToLower(signOf(l)))) {
		return ErrTampered
	}
	if l.Expired() {
		return ErrExpired
	}
	return nil
}

func signOf(l *License) string {
	payload := fmt.Sprintf("%s|%s|%d|%d",
		strings.ToUpper(strings.TrimSpace(l.Machine)),
		strings.TrimSpace(l.Customer),
		l.IssuedAt, l.ExpiresAt,
	)
	m := hmac.New(sha256.New, []byte(Secret))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// ========================================
// 授权文件读写
// ========================================

// Load 读授权文件。文件不存在返回 ErrNotFound。
func Load(path string) (*License, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var l License
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, ErrTampered
	}
	return &l, nil
}

// Save 写授权文件（0600，仅本机账号可读）。
func Save(path string, l *License) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ========================================
// 离线授权码（在线激活不了时的兜底，人工念给客户）
//   形如 ORDAI1-20271019-9K3M7Q
// ========================================

const offlinePrefix = "ORDAI1"

// OfflineCode 生成离线授权码：绑定机器码 + 到期日，不含客户名。
func OfflineCode(machine string, expires time.Time) string {
	date := expires.Format("20060102")
	return offlinePrefix + "-" + date + "-" + offlineTag(machine, date)
}

// ParseOfflineCode 校验离线授权码，返回到期时间。
func ParseOfflineCode(machine, code string) (time.Time, error) {
	raw := strings.ToUpper(strings.TrimSpace(code))
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "－", "-")
	raw = strings.TrimPrefix(raw, offlinePrefix+"-")
	raw = strings.TrimPrefix(raw, offlinePrefix)
	raw = strings.TrimPrefix(raw, "-")

	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return time.Time{}, errors.New("授权码格式不对，应形如 ORDAI1-20271019-9K3M7Q")
	}
	date, tag := parts[0], parts[1]
	expires, err := time.ParseInLocation("20060102", date, time.Local)
	if err != nil {
		return time.Time{}, errors.New("授权码里的日期无效")
	}
	if !strings.EqualFold(tag, offlineTag(machine, date)) {
		return time.Time{}, ErrMachineMismatch
	}
	if time.Now().After(expires.Add(24 * time.Hour)) {
		return time.Time{}, ErrExpired
	}
	return expires, nil
}

func offlineTag(machine, date string) string {
	m := hmac.New(sha256.New, []byte(Secret))
	m.Write([]byte(normalize(machine) + "|" + date))
	return encode32(m.Sum(nil)[:4])[:6]
}

// ========================================
// 编码工具
// ========================================

// Crockford Base32：去掉 I / L / O / U，避免和 1 / 0 看混。
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

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

func sha256sum(s string) [32]byte {
	return sha256.Sum256([]byte(s))
}
