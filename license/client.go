package license

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 在线激活客户端：把 PIN + 本机指纹发到授权中心，换回签名好的授权文件。

// ActivateRequest 激活请求体。
type ActivateRequest struct {
	PIN      string `json:"pin"`
	Machine  string `json:"machine"`
	CPU      string `json:"cpu"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
}

type activateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		License *License `json:"license"`
	} `json:"data"`
}

// ActivateOnline 向授权中心申请激活。返回授权文件与服务器提示语。
func ActivateOnline(serverURL, pin string) (*License, string, error) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return nil, "", errors.New("未配置授权中心地址（.env 的 LICENSE_SERVER）")
	}
	pin = strings.ToUpper(strings.TrimSpace(pin))
	if pin == "" {
		return nil, "", errors.New("请输入激活码")
	}

	cpu, mac, err := Fingerprint()
	if err != nil {
		return nil, "", err
	}
	body, _ := json.Marshal(ActivateRequest{
		PIN:      pin,
		Machine:  MachineIDFrom(cpu, mac),
		CPU:      cpu,
		MAC:      mac,
		Hostname: Hostname(),
	})

	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/activate", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("授权中心地址无效: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("连不上授权中心（%s）：请检查网络或 .env 配置", serverURL)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	var out activateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("授权中心返回异常（HTTP %d）", resp.StatusCode)
	}
	if !out.Success || out.Data.License == nil {
		msg := out.Message
		if msg == "" {
			msg = fmt.Sprintf("激活失败（HTTP %d）", resp.StatusCode)
		}
		return nil, "", errors.New(msg)
	}

	l := out.Data.License
	if err := l.VerifyLocal(MachineIDFrom(cpu, mac)); err != nil {
		return nil, "", fmt.Errorf("授权文件校验不通过: %v", err)
	}
	return l, out.Message, nil
}
