//go:build linux || darwin

package demo

import (
	"os/exec"
	"syscall"
)

func startProcess(cmd *exec.Cmd) (func(bool), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func(running bool) {
		if running {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}, nil
}
