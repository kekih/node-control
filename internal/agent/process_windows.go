//go:build windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
	_ = cmd.Process.Kill()
}

func isProcessAlive(p *os.Process) bool {
	if p == nil {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", p.Pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0 && !strings.Contains(string(out), "No tasks are running")
}

func setProcessGroup(cmd *exec.Cmd) {
	// No-op on Windows
}
