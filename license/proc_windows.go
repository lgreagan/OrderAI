//go:build windows

package license

import "syscall"

// hiddenWindow 避免调用 wmic / PowerShell 时弹出黑窗口。
func hiddenWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
