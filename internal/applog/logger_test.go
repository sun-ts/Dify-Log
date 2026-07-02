package applog

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, true, "info")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Error("error message")

	text := buf.String()
	if strings.Contains(text, "debug message") {
		t.Fatalf("debug log was not filtered: %s", text)
	}
	for _, want := range []string{"level=info", "info message", "level=error", "error message"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q: %s", want, text)
		}
	}
}

func TestLoggerDisabledWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, false, "debug")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	logger.Error("error message")

	if buf.String() != "" {
		t.Fatalf("disabled logger wrote output: %s", buf.String())
	}
}

func TestLoggerRejectsInvalidLevel(t *testing.T) {
	_, err := New(&bytes.Buffer{}, true, "trace")
	if err == nil {
		t.Fatal("expected invalid level to fail")
	}
}
