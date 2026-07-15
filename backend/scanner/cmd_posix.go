//go:build !windows

package scanner

import (
	"os/exec"
)

func setupCmd(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}
