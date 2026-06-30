package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dify-log-excel/internal/config"
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
	case "status":
		return runStatus(baseDir, out, errOut)
	case "sync":
		return runSync(baseDir, out, errOut)
	case "serve":
		return runServe(baseDir, out, errOut)
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
	fmt.Fprintf(out, "data_dir=%s\nexcel_dir=%s\naddress=%s\n", cfg.DataDir, cfg.ExcelDir, cfg.Address())
	if summary.LastSyncError != "" {
		fmt.Fprintf(out, "last_error=%s\n", summary.LastSyncError)
	}
	if summary.LastSyncedAt.Valid {
		fmt.Fprintf(out, "last_synced_at=%s\n", summary.LastSyncedAt.Time.Format(time.RFC3339Nano))
	}
	return 0
}

func runSync(baseDir string, out io.Writer, errOut io.Writer) int {
	_, st, _, err := openConfiguredStore(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer st.Close()
	result, err := excelsync.SyncPending(context.Background(), st, 1000)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintf(out, "synced=%d failed=%d\n", result.Synced, result.Failed)
	return 0
}

func runServe(baseDir string, out io.Writer, errOut io.Writer) int {
	cfg, st, loc, err := openConfiguredStore(baseDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer st.Close()

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
				_, _ = excelsync.SyncPending(context.Background(), st, 1000)
			}
		}
	}()

	httpServer := &http.Server{
		Addr:    cfg.Address(),
		Handler: server.New(st, cfg, loc),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(out, "listening=http://%s/api/v1/logs\n", cfg.Address())
	err = httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(errOut, err)
		return 1
	}
	_, _ = excelsync.SyncPending(context.Background(), st, 1000)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: dify-log-excel <serve|sync|status|version>")
}
