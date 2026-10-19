// 授权中心（License Server）
//
// 独立小程序，跑在你自己的机器 / VPS 上；客户那边的 OrderAI 激活页连过来换授权。
//
// 启动：
//
//	go run ./tools/licserver
//	LICS_PORT=8090 LICS_TOKEN=my-secret go run ./tools/licserver
//
// 管理页： http://127.0.0.1:8090/  （首次进入填 LICS_TOKEN）
// 客户端： .env 里 LICENSE_SERVER=http://<本机IP>:8090
package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"OrderAI/license"

	"github.com/labstack/echo/v4"
)

//go:embed admin.html
var adminHTML []byte

// Pin 一次性激活码。一个 PIN 绑一台机器；同一台机器重复激活视为重装，直接续签。
type Pin struct {
	PIN       string `json:"pin"`
	Customer  string `json:"customer"`
	Days      int    `json:"days"` // 0 = 永久
	CreatedAt int64  `json:"created_at"`
	UsedAt    int64  `json:"used_at,omitempty"`
	Machine   string `json:"machine,omitempty"`
	CPU       string `json:"cpu,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Revoked   bool   `json:"revoked,omitempty"`
}

type store struct {
	PINs []*Pin `json:"pins"`
}

type server struct {
	mu    sync.Mutex
	path  string
	data  store
	token string
}

func loadStore(path string) (*server, error) {
	s := &server{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %v", path, err)
	}
	return s, nil
}

// saveLocked 调用方需持有锁。
func (s *server) saveLocked() error {
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *server) findPin(code string) *Pin {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, p := range s.data.PINs {
		if strings.EqualFold(p.PIN, code) {
			return p
		}
	}
	return nil
}

// ========================================
// 管理接口
// ========================================

func (s *server) auth(c echo.Context) bool {
	token := c.Request().Header.Get("X-Lics-Token")
	if token == "" {
		token = c.QueryParam("token")
	}
	return token != "" && token == s.token
}

func (s *server) guard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !s.auth(c) {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"success": false, "message": "管理口令不对",
			})
		}
		return next(c)
	}
}

// List 激活码列表。
//
// GET /api/pins
func (s *server) List(c echo.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pins := make([]Pin, 0, len(s.data.PINs))
	used, total := 0, len(s.data.PINs)
	for i := len(s.data.PINs) - 1; i >= 0; i-- {
		p := *s.data.PINs[i]
		if p.UsedAt > 0 {
			used++
		}
		pins = append(pins, p)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"pins":  pins,
			"total": total,
			"used":  used,
		},
	})
}

// Create 批量生成激活码。
//
// POST /api/pins  {customer, days, count}
func (s *server) Create(c echo.Context) error {
	var req struct {
		Customer string `json:"customer"`
		Days     int    `json:"days"`
		Count    int    `json:"count"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "参数错误"})
	}
	req.Customer = strings.TrimSpace(req.Customer)
	if req.Customer == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "请填使用单位"})
	}
	if req.Days < 0 || req.Days > 3650 {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "有效天数需在 0-3650 之间（0 = 永久）"})
	}
	if req.Count <= 0 || req.Count > 50 {
		req.Count = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	created := make([]Pin, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		p := &Pin{
			PIN:       newPIN(),
			Customer:  req.Customer,
			Days:      req.Days,
			CreatedAt: now.Unix(),
		}
		s.data.PINs = append(s.data.PINs, p)
		created = append(created, *p)
	}
	if err := s.saveLocked(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
	}
	log.Printf("生成 %d 个激活码：%s / %d 天", len(created), req.Customer, req.Days)

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("已生成 %d 个激活码", len(created)),
		"data":    created,
	})
}

// Revoke 作废激活码（已激活的机器下次校验前仍可用，重装后即失效）。
//
// POST /api/pins/:pin/revoke
func (s *server) Revoke(c echo.Context) error {
	code := c.Param("pin")

	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPin(code)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"success": false, "message": "激活码不存在"})
	}
	p.Revoked = true
	if err := s.saveLocked(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "已作废"})
}

// Offline 按机器码生成离线激活码。
//
// POST /api/offline  {machine, days}
func (s *server) Offline(c echo.Context) error {
	var req struct {
		Machine string `json:"machine"`
		Days    int    `json:"days"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "参数错误"})
	}
	req.Machine = strings.ToUpper(strings.TrimSpace(req.Machine))
	if req.Machine == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "请填机器码"})
	}
	if req.Days <= 0 || req.Days > 3650 {
		req.Days = 365
	}
	expires := time.Now().AddDate(0, 0, req.Days)
	code := license.OfflineCode(req.Machine, expires)
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"code":    code,
			"expires": expires.Format("2006-01-02"),
		},
	})
}

// ========================================
// 客户端激活
// ========================================

// Activate 客户端换授权：PIN + 机器指纹 → 签名好的授权文件。
//
// POST /api/activate
func (s *server) Activate(c echo.Context) error {
	var req license.ActivateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "参数错误"})
	}
	req.Machine = strings.ToUpper(strings.TrimSpace(req.Machine))
	if req.Machine == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "缺少机器码"})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPin(req.PIN)
	if p == nil {
		log.Printf("激活失败：激活码 %s 不存在（机器 %s）", req.PIN, req.Machine)
		return c.JSON(http.StatusNotFound, map[string]any{"success": false, "message": "激活码不存在，请检查是否输错"})
	}
	if p.Revoked {
		return c.JSON(http.StatusForbidden, map[string]any{"success": false, "message": "该激活码已作废"})
	}
	if p.UsedAt > 0 && p.Machine != "" && !strings.EqualFold(p.Machine, req.Machine) {
		log.Printf("激活失败：%s 已绑定 %s，本次机器 %s", p.PIN, p.Machine, req.Machine)
		return c.JSON(http.StatusForbidden, map[string]any{
			"success": false,
			"message": "该激活码已绑定其他机器（" + p.Machine + "），请联系供应商",
		})
	}

	now := time.Now()
	p.UsedAt = now.Unix()
	p.Machine = req.Machine
	p.CPU = req.CPU
	p.MAC = req.MAC
	p.Hostname = req.Hostname
	p.ExpiresAt = 0
	if p.Days > 0 {
		p.ExpiresAt = now.AddDate(0, 0, p.Days).Unix()
	}

	lic := &license.License{
		Machine:   req.Machine,
		Customer:  p.Customer,
		IssuedAt:  now.Unix(),
		ExpiresAt: p.ExpiresAt,
	}
	lic.Sign()

	if err := s.saveLocked(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
	}

	log.Printf("激活成功：%s → %s（%s，到期 %s）", p.PIN, req.Machine, req.Hostname, lic.ExpiresText())

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("激活成功，有效期至 %s", lic.ExpiresText()),
		"data":    map[string]any{"license": lic},
	})
}

// ========================================
// 启动
// ========================================

func main() {
	port := flag.String("port", envOr("LICS_PORT", "8090"), "监听端口")
	dataPath := flag.String("data", envOr("LICS_DATA", "tools/licserver/data/pins.json"), "激活记录存放路径")
	flag.Parse()

	token := envOr("LICS_TOKEN", "")
	generated := false
	if token == "" {
		token = randomToken(8)
		generated = true
	}

	s, err := loadStore(*dataPath)
	if err != nil {
		log.Fatal(err)
	}
	s.token = token

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	// 极简访问日志，省掉一个额外的依赖
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}
			log.Printf("%s %s %d %s", c.Request().Method, c.Request().URL.Path,
				c.Response().Status, time.Since(start).Round(time.Millisecond))
			return nil
		}
	})
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic: %v", r)
					c.JSON(http.StatusInternalServerError, map[string]any{
						"success": false, "message": "服务器内部错误",
					})
				}
			}()
			return next(c)
		}
	})

	e.GET("/", func(c echo.Context) error {
		return c.HTMLBlob(http.StatusOK, adminHTML)
	})

	api := e.Group("/api", s.guard)
	api.GET("/pins", s.List)
	api.POST("/pins", s.Create)
	api.POST("/pins/:pin/revoke", s.Revoke)
	api.POST("/offline", s.Offline)

	// 客户端调用，不走管理口令
	e.POST("/api/activate", s.Activate)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	log.Println("========================================")
	log.Printf("授权中心已启动：http://0.0.0.0:%s", *port)
	log.Printf("管理口令：%s", token)
	if generated {
		log.Println("（这是本次随机生成的口令，正式使用请设环境变量 LICS_TOKEN）")
	}
	log.Printf("激活记录：%s", *dataPath)
	log.Println("客户那边配置：LICENSE_SERVER=http://<本机IP>:" + *port)
	log.Println("========================================")

	if err := e.Start(":" + *port); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

const pinAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func newPIN() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退回时间戳，保证还能出码
		ts := fmt.Sprintf("%d", time.Now().UnixNano())
		return "LICS-" + ts[len(ts)-8:]
	}
	var sb strings.Builder
	for _, x := range b {
		sb.WriteByte(pinAlphabet[int(x)%len(pinAlphabet)])
	}
	s := sb.String()
	return "LICS-" + s[0:4] + "-" + s[4:8]
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "lics-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	var sb strings.Builder
	for _, x := range b {
		sb.WriteByte(pinAlphabet[int(x)%len(pinAlphabet)])
	}
	return strings.ToLower(sb.String())
}
