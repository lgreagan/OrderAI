package services

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"OrderAI/license"
)

// LicenseService 管理本机授权：读授权文件、在线激活、离线激活码。
//
// 授权文件默认放 data/license.dat，绑定本机 CPU + 主网卡 MAC。
type LicenseService struct {
	// 授权文件路径（.env: LICENSE_FILE）
	file string
	// 授权中心地址（.env: LICENSE_SERVER），形如 http://1.2.3.4:8090
	server string

	mu     sync.RWMutex
	cur    *license.License
	loaded bool
}

func NewLicenseService(file, server string) *LicenseService {
	if strings.TrimSpace(file) == "" {
		file = "data/license.dat"
	}
	return &LicenseService{
		file:   file,
		server: strings.TrimSpace(server),
	}
}

// Server 授权中心地址，激活页上会显示，方便排查配置。
func (s *LicenseService) Server() string { return s.server }

// File 授权文件路径。
func (s *LicenseService) File() string { return s.file }

// Machine 本机码、CPU、MAC。
func (s *LicenseService) Machine() (id, cpu, mac string, err error) {
	cpu, mac, err = license.Fingerprint()
	if err != nil {
		return "", "", "", err
	}
	return license.MachineIDFrom(cpu, mac), cpu, mac, nil
}

// Current 当前授权文件（含校验结果）。未激活返回 nil + ErrNotFound。
func (s *LicenseService) Current() (*license.License, error) {
	s.mu.RLock()
	if s.loaded {
		l := s.cur
		s.mu.RUnlock()
		if l == nil {
			return nil, license.ErrNotFound
		}
		return l, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return s.cur, nil
	}
	l, err := license.Load(s.file)
	if err != nil {
		s.loaded = true
		s.cur = nil
		if errors.Is(err, license.ErrNotFound) {
			return nil, err
		}
		log.Printf("读取授权文件失败: %v", err)
		return nil, err
	}
	s.cur = l
	s.loaded = true
	return l, nil
}

// Ready 系统是否可用：授权文件存在、属于本机、签名正确、未过期。
func (s *LicenseService) Ready() error {
	machine, _, _, err := s.Machine()
	if err != nil {
		return err
	}
	l, err := s.Current()
	if err != nil {
		return err
	}
	if err := l.VerifyLocal(machine); err != nil {
		if errors.Is(err, license.ErrMachineMismatch) || errors.Is(err, license.ErrTampered) {
			log.Printf("授权文件与当前机器不匹配: %v", err)
		}
		return err
	}
	return nil
}

// Activate 在线激活：拿 PIN 去授权中心换授权文件。
func (s *LicenseService) Activate(pin string) (*license.License, string, error) {
	l, msg, err := license.ActivateOnline(s.server, pin)
	if err != nil {
		return nil, "", err
	}
	if err := license.Save(s.file, l); err != nil {
		return nil, "", err
	}
	s.set(l)
	log.Printf("在线激活成功：客户=%s 到期=%s 机器=%s", l.Customer, l.ExpiresText(), l.Machine)
	return l, msg, nil
}

// ActivateOffline 用离线授权码激活（授权中心连不上时人工念码）。
func (s *LicenseService) ActivateOffline(code string) (*license.License, error) {
	machine, _, _, err := s.Machine()
	if err != nil {
		return nil, err
	}
	expires, err := license.ParseOfflineCode(machine, code)
	if err != nil {
		return nil, err
	}
	l := &license.License{
		Machine:   machine,
		Customer:  "离线授权",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expires.Unix(),
	}
	l.Sign()
	if err := license.Save(s.file, l); err != nil {
		return nil, err
	}
	s.set(l)
	log.Printf("离线激活成功：到期=%s 机器=%s", l.ExpiresText(), l.Machine)
	return l, nil
}

// Reload 丢掉缓存重新读文件（授权文件被手工替换后调一下）。
func (s *LicenseService) Reload() {
	s.mu.Lock()
	s.loaded = false
	s.cur = nil
	s.mu.Unlock()
}

func (s *LicenseService) set(l *license.License) {
	s.mu.Lock()
	s.cur = l
	s.loaded = true
	s.mu.Unlock()
}
