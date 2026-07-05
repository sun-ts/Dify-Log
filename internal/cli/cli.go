package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dify-log-excel/internal/applog"
	"dify-log-excel/internal/config"
	"dify-log-excel/internal/daemon"
	"dify-log-excel/internal/excelsync"
	"dify-log-excel/internal/server"
	"dify-log-excel/internal/store"
	"dify-log-excel/internal/version"
)

func Run(args []string, baseDir string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 {
		printUsage(errOut)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(out, version.Version)
		return 0
	case "start":
		return runStart(baseDir, out, errOut)
	case "stop":
		return runStop(baseDir, out, errOut)
	case "restart":
		if code := runStop(baseDir, out, errOut); code != 0 {
			return code
		}
		return runStart(baseDir, out, errOut)
	case "status":
		return runStatus(baseDir, out, errOut)
	case "sync":
		return runSync(baseDir, out, errOut)
	case "serve":
		background := false
		if len(args) > 2 || (len(args) == 2 && args[1] != "--background") {
			printUsage(errOut)
			return 2
		}
		if len(args) == 2 {
			background = true
		}
		return runServe(baseDir, out, errOut, background)
	default:
		printUsage(errOut)
		return 2
	}
}

func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	return filepath.Dir(resolved)
}

func openConfiguredStore(baseDir string) (config.Config, *store.Store, *time.Location, error) {
	cfg, err := config.Load(baseDir)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return config.Config{}, nil, nil, err
	}
	if err := os.MkdirAll(cfg.ExcelDir, 0o755); err != nil {
		return config.Config{}, nil, nil, err
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	st, err := store.Open(filepath.Join(cfg.DataDir, "dify_logs.db"))
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	if err := st.Init(context.Background()); err != nil {
		_ = st.Close()
		return config.Config{}, nil, nil, err
	}
	return cfg, st, loc, nil
}

func configuredDaemon(baseDir string) (config.Config, daemon.Paths, error) {
	cfg, err := config.Load(baseDir)
	if err != nil {
		return config.Config{}, daemon.Paths{}, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return config.Config{}, daemon.Paths{}, err
	}
	if err := os.MkdirAll(cfg.ExcelDir, 0o755); err != nil {
		return config.Config{}, daemon.Paths{}, err
	}
	return cfg, daemon.PathsFor(cfg.LogDir), nil
}

func runStart(baseDir string, out io.Writer, errOut io.Writer) int {
	cfg, paths, err := configuredDaemon(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	status, err := daemon.Inspect(paths)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if status.Running {
		fmt.Fprintf(out, "started=false\nalready_running=true\npid=%d\naddress=http://%s/api/v1/logs\npid_file=%s\ncurrent_log_file=%s\n", status.PID, cfg.Address(), paths.PIDFile, currentLogPath(cfg))
		return 0
	}
	if status.Stale {
		_ = daemon.ClearPID(paths.PIDFile)
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	pid, err := daemon.StartDetached(executable, []string{"serve", "--background"}, baseDir, currentLogPath(cfg))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if err := daemon.WritePID(paths.PIDFile, pid); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	readyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	readyErr := daemon.WaitHTTPReady(readyCtx, daemon.LocalURL(cfg.Host, cfg.Port, "/api/v1/logs"), pid)
	cancel()
	if readyErr != nil {
		_ = daemon.Terminate(pid)
		_ = daemon.WaitStopped(pid, 2*time.Second)
		_ = daemon.ClearPID(paths.PIDFile)
		fmt.Fprintf(errOut, "%v; see current_log_file=%s\n", readyErr, currentLogPath(cfg))
		return 1
	}
	fmt.Fprintf(out, "started=true\npid=%d\naddress=http://%s/api/v1/logs\npid_file=%s\ncurrent_log_file=%s\n", pid, cfg.Address(), paths.PIDFile, currentLogPath(cfg))
	return 0
}

func runStop(baseDir string, out io.Writer, errOut io.Writer) int {
	cfg, paths, err := configuredDaemon(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	status, err := daemon.Inspect(paths)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if !status.Running {
		if status.Stale {
			_ = daemon.ClearPID(paths.PIDFile)
			fmt.Fprintln(out, "daemon_running=false\nstale_pid_cleared=true")
			return 0
		}
		fmt.Fprintln(out, "daemon_running=false")
		return 0
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	stopErr := daemon.RequestStop(stopCtx, daemon.StopURL(cfg.Host, cfg.Port), cfg.LogAPIKey)
	cancel()
	if stopErr != nil {
		fmt.Fprintf(errOut, "graceful stop request failed, falling back to process signal: %v\n", stopErr)
		if err := daemon.Terminate(status.PID); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
	}
	if !daemon.WaitStopped(status.PID, 5*time.Second) {
		if err := daemon.Kill(status.PID); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if !daemon.WaitStopped(status.PID, 2*time.Second) {
			fmt.Fprintf(errOut, "process %d did not stop\n", status.PID)
			return 1
		}
	}
	_ = daemon.ClearPID(paths.PIDFile)
	fmt.Fprintf(out, "stopped=true\npid=%d\n", status.PID)
	return 0
}

func runStatus(baseDir string, out io.Writer, errOut io.Writer) int {
	cfg, st, _, err := openConfiguredStore(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer st.Close()
	summary, err := st.Summary(context.Background())
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintf(out, "executions=%d node_logs=%d pending=%d\n", summary.ExecutionCount, summary.NodeLogCount, summary.PendingCount)
	fmt.Fprintf(out, "data_dir=%s\nexcel_dir=%s\nlog_dir=%s\naddress=%s\n", cfg.DataDir, cfg.ExcelDir, cfg.LogDir, cfg.Address())
	daemonStatus, err := daemon.Inspect(daemon.PathsFor(cfg.LogDir))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintf(out, "daemon_running=%t\npid_file=%s\nlog_enabled=%t\nlog_level=%s\nlog_body=%t\ncurrent_log_file=%s\n", daemonStatus.Running, daemonStatus.Paths.PIDFile, cfg.LogEnabled, cfg.LogLevel, cfg.LogBody, currentLogPath(cfg))
	if daemonStatus.PID != 0 {
		fmt.Fprintf(out, "daemon_pid=%d\n", daemonStatus.PID)
	}
	if daemonStatus.Stale {
		fmt.Fprintln(out, "daemon_stale_pid=true")
	}
	if summary.LastSyncError != "" {
		fmt.Fprintf(out, "last_error=%s\n", summary.LastSyncError)
	}
	if summary.LastSyncedAt.Valid {
		fmt.Fprintf(out, "last_synced_at=%s\n", summary.LastSyncedAt.Time.Format(time.RFC3339Nano))
	}
	return 0
}

func runSync(baseDir string, out io.Writer, errOut io.Writer) int {
	cfg, st, _, err := openConfiguredStore(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer st.Close()
	appLog, closer, _, err := openAppLogger(cfg, nil)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer closer.Close()
	result, err := excelsync.SyncPending(context.Background(), st, 1000)
	if err != nil {
		appLog.Error("sync failed error=%q", err.Error())
		fmt.Fprintln(errOut, err)
		return 1
	}
	appLog.Info("sync completed synced=%d failed=%d", result.Synced, result.Failed)
	fmt.Fprintf(out, "synced=%d failed=%d\n", result.Synced, result.Failed)
	return 0
}

func runServe(baseDir string, out io.Writer, errOut io.Writer, background bool) int {
	cfg, st, loc, err := openConfiguredStore(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer st.Close()
	paths := daemon.PathsFor(cfg.LogDir)
	var mirror io.Writer
	if !background {
		mirror = errOut
	}
	appLog, closer, logPath, err := openAppLogger(cfg, mirror)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer closer.Close()
	if background {
		if err := daemon.WritePID(paths.PIDFile, os.Getpid()); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		defer daemon.ClearPID(paths.PIDFile)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.SyncIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := excelsync.SyncPending(context.Background(), st, 1000)
				if err != nil {
					appLog.Error("scheduled sync failed error=%q", err.Error())
					continue
				}
				if result.Synced > 0 || result.Failed > 0 {
					appLog.Info("scheduled sync completed synced=%d failed=%d", result.Synced, result.Failed)
				} else {
					appLog.Debug("scheduled sync completed synced=0 failed=0")
				}
			}
		}
	}()

	httpServer := &http.Server{
		Addr:     cfg.Address(),
		Handler:  server.NewWithLogger(st, cfg, loc, stop, appLog),
		ErrorLog: log.New(appLog, "", 0),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	appLog.Info("server starting address=%s log_file=%s background=%t", cfg.Address(), logPath, background)
	fmt.Fprintf(out, "listening=http://%s/api/v1/logs\ncurrent_log_file=%s\n", cfg.Address(), logPath)
	err = httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		appLog.Error("server failed error=%q", err.Error())
		fmt.Fprintln(errOut, err)
		return 1
	}
	result, err := excelsync.SyncPending(context.Background(), st, 1000)
	if err != nil {
		appLog.Error("final sync failed error=%q", err.Error())
	} else {
		appLog.Info("server stopped final_synced=%d final_failed=%d", result.Synced, result.Failed)
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: dify-log-excel <start|stop|restart|serve|sync|status|version>")
}

func currentLogPath(cfg config.Config) string {
	if !cfg.LogEnabled {
		return ""
	}
	return applog.DailyLogPath(cfg.LogDir, time.Now())
}

func openAppLogger(cfg config.Config, mirror io.Writer) (*applog.Logger, io.Closer, string, error) {
	if !cfg.LogEnabled {
		logger, err := applog.New(io.Discard, false, cfg.LogLevel)
		return logger, io.NopCloser(strings.NewReader("")), "", err
	}
	writer := applog.NewDailyFileWriter(cfg.LogDir, mirror)
	logger, err := applog.New(writer, true, cfg.LogLevel)
	if err != nil {
		_ = writer.Close()
		return nil, nil, "", err
	}
	return logger, writer, applog.DailyLogPath(cfg.LogDir, time.Now()), nil
}
