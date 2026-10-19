//go:build !windows

package license

import "syscall"

func hiddenWindow() *syscall.SysProcAttr {
	return nil
}
