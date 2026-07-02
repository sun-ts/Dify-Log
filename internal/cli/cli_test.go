package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dify-log-excel/internal/config"
)

func TestStatusCommandInitializesStoreAndPrintsSummary(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"status"}, base, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "node_logs=0") {
		t.Fatalf("stdout = %s", out.String())
	}
	if !strings.Contains(out.String(), filepath.Join(base, "logs")) {
		t.Fatalf("stdout missing logs dir: %s", out.String())
	}
	if !strings.Contains(out.String(), "excel_dir="+filepath.Join(base, "data", "excel")) {
		t.Fatalf("stdout missing excel dir: %s", out.String())
	}
	if !strings.Contains(out.String(), "log_dir="+filepath.Join(base, "logs")) {
		t.Fatalf("stdout missing log dir: %s", out.String())
	}
	if !strings.Contains(out.String(), "daemon_running=false") {
		t.Fatalf("stdout missing daemon status: %s", out.String())
	}
	if !strings.Contains(out.String(), filepath.Join(base, "logs", "dify-log-excel.pid")) {
		t.Fatalf("stdout missing pid file: %s", out.String())
	}
	if !strings.Contains(out.String(), "log_enabled=true") {
		t.Fatalf("stdout missing log config: %s", out.String())
	}
	if !strings.Contains(out.String(), "log_level=info") || !strings.Contains(out.String(), "log_body=true") {
		t.Fatalf("stdout missing log level/body config: %s", out.String())
	}
}

func TestUnknownCommandReturnsUsage(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"unknown"}, base, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errOut.String(), "usage: dify-log-excel") {
		t.Fatalf("stderr = %s", errOut.String())
	}
}

func TestStartReportsAlreadyRunningWhenPIDIsAlive(t *testing.T) {
	base := t.TempDir()
	pidPath := filepath.Join(base, "logs", "dify-log-excel.pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"start"}, base, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "already_running=true") {
		t.Fatalf("stdout = %s", out.String())
	}
}

func TestStopClearsStalePID(t *testing.T) {
	base := t.TempDir()
	pidPath := filepath.Join(base, "logs", "dify-log-excel.pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("99999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"stop"}, base, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "stale_pid_cleared=true") {
		t.Fatalf("stdout = %s", out.String())
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists or unexpected error: %v", err)
	}
}

func TestStatusReportsCurrentLogFileWithoutCreatingIt(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"status"}, base, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	logPath := filepath.Join(base, "logs")
	if _, err := os.Stat(logPath); err == nil {
		t.Fatalf("status should report log path without creating log directory")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat log directory returned unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), filepath.Join(base, "logs", "app-")) {
		t.Fatalf("stdout missing current log file: %s", out.String())
	}
}

func TestOpenAppLoggerMirrorsToTerminalAndDailyFile(t *testing.T) {
	base := t.TempDir()
	var terminal bytes.Buffer
	cfg := config.Default(base)

	logger, closer, logPath, err := openAppLogger(cfg, &terminal)
	if err != nil {
		t.Fatalf("openAppLogger returned error: %v", err)
	}
	logger.Error("request failed")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if !strings.Contains(terminal.String(), "request failed") {
		t.Fatalf("terminal output = %s", terminal.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "request failed") {
		t.Fatalf("log file = %s", string(data))
	}
}

func TestUsageIncludesDaemonCommands(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run(nil, base, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	usage := errOut.String()
	for _, command := range []string{"start", "stop", "restart", "serve", "sync", "status", "version"} {
		if !strings.Contains(usage, command) {
			t.Fatalf("usage missing %q: %s", command, usage)
		}
	}
}
