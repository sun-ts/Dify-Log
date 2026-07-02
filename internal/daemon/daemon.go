package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	pidFileName = "dify-log-excel.pid"
	logFileName = "dify-log-excel.out.log"
)

type Paths struct {
	PIDFile string
	LogFile string
}

type Status struct {
	Running bool
	PID     int
	Stale   bool
	Paths   Paths
}

func PathsFor(logDir string) Paths {
	return Paths{
		PIDFile: filepath.Join(logDir, pidFileName),
		LogFile: filepath.Join(logDir, logFileName),
	}
}

func WritePID(path string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func ReadPID(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	raw := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(raw)
	if err != nil || pid <= 0 {
		return 0, false, fmt.Errorf("invalid pid file %s", path)
	}
	return pid, true, nil
}

func ClearPID(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func Inspect(paths Paths) (Status, error) {
	pid, ok, err := ReadPID(paths.PIDFile)
	if err != nil {
		return Status{Paths: paths}, err
	}
	if !ok {
		return Status{Paths: paths}, nil
	}
	running := ProcessRunning(pid)
	return Status{
		Running: running,
		PID:     pid,
		Stale:   !running,
		Paths:   paths,
	}, nil
}
