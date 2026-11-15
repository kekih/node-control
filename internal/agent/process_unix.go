//go:build !windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	// Kill entire process group
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	_ = exec.Command("pkill", "-9", "-P", fmt.Sprintf("%d", pid)).Run()
	_ = cmd.Process.Kill()
}

func isProcessAlive(p *os.Process) bool {
	if p == nil {
		return false
	}
	err := p.Signal(syscall.Signal(0))
	return err == nil
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
