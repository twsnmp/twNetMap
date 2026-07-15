//go:build windows

package scanner

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW is a Windows creation flag to start a process without a console window.
const CREATE_NO_WINDOW = 0x08000000

func setupCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = CREATE_NO_WINDOW
}
