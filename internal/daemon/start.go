package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func StartDetached(executable string, args []string, workingDir string, logPath string) (int, error) {
	if logPath == "" {
		logPath = os.DevNull
	} else {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return 0, err
		}
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	cmd := exec.Command(executable, args...)
	cmd.Dir = workingDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachedSysProcAttr()

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}

func WaitStopped(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !ProcessRunning(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
