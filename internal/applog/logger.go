package applog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	ErrorLevel Level = iota
	InfoLevel
	DebugLevel
)

func ParseLevel(value string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return ErrorLevel, nil
	case "info":
		return InfoLevel, nil
	case "debug":
		return DebugLevel, nil
	default:
		return InfoLevel, fmt.Errorf("log_level must be one of error, info, debug")
	}
}

func LevelName(level Level) string {
	switch level {
	case ErrorLevel:
		return "error"
	case InfoLevel:
		return "info"
	case DebugLevel:
		return "debug"
	default:
		return "info"
	}
}

type Logger struct {
	enabled bool
	level   Level
	out     io.Writer
}

func New(out io.Writer, enabled bool, levelName string) (*Logger, error) {
	level, err := ParseLevel(levelName)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = io.Discard
	}
	return &Logger{enabled: enabled, level: level, out: out}, nil
}

func (l *Logger) Enabled() bool {
	return l != nil && l.enabled
}

func (l *Logger) LevelName() string {
	if l == nil {
		return "info"
	}
	return LevelName(l.level)
}

func (l *Logger) Error(format string, args ...any) {
	l.log(ErrorLevel, format, args...)
}

func (l *Logger) Info(format string, args ...any) {
	l.log(InfoLevel, format, args...)
}

func (l *Logger) Debug(format string, args ...any) {
	l.log(DebugLevel, format, args...)
}

func (l *Logger) Write(p []byte) (int, error) {
	if l == nil || !l.enabled {
		return len(p), nil
	}
	l.Error("%s", strings.TrimRight(string(p), "\r\n"))
	return len(p), nil
}

func (l *Logger) log(level Level, format string, args ...any) {
	if l == nil || !l.enabled || level > l.level {
		return
	}
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.out, "ts=%s level=%s msg=%q\n", time.Now().Format(time.RFC3339Nano), LevelName(level), message)
}

func DailyLogPath(logDir string, t time.Time) string {
	return filepath.Join(logDir, "app-"+t.Format("2006-01-02")+".log")
}

type DailyFileWriter struct {
	mu     sync.Mutex
	dir    string
	now    func() time.Time
	date   string
	file   *os.File
	mirror io.Writer
}

func NewDailyFileWriter(logDir string, mirror io.Writer) *DailyFileWriter {
	return &DailyFileWriter{
		dir:    logDir,
		now:    time.Now,
		mirror: mirror,
	}
}

func (w *DailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(w.now()); err != nil {
		return 0, err
	}
	if w.mirror != nil {
		_, _ = w.mirror.Write(p)
	}
	return w.file.Write(p)
}

func (w *DailyFileWriter) Path(t time.Time) string {
	return DailyLogPath(w.dir, t)
}

func (w *DailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *DailyFileWriter) rotateLocked(now time.Time) error {
	date := now.Format("2006-01-02")
	if w.file != nil && w.date == date {
		return nil
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	file, err := os.OpenFile(DailyLogPath(w.dir, now), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.date = date
	return nil
}
