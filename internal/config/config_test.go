package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfigUsesBaseDir(t *testing.T) {
	base := t.TempDir()

	cfg, err := Load(base)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Address() != "127.0.0.1:8000" {
		t.Fatalf("Address() = %q", cfg.Address())
	}
	if cfg.DataDir != filepath.Join(base, "data") {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.ExcelDir != filepath.Join(base, "data", "excel") {
		t.Fatalf("ExcelDir = %q", cfg.ExcelDir)
	}
	if cfg.LogDir != filepath.Join(base, "logs") {
		t.Fatalf("LogDir = %q", cfg.LogDir)
	}
	if !cfg.LogEnabled {
		t.Fatal("LogEnabled = false, want true")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if !reflect.DeepEqual(cfg.MaskFields, []string{"password", "token", "api_key", "phone"}) {
		t.Fatalf("MaskFields = %#v", cfg.MaskFields)
	}
}

func TestLoadConfigFileAndResolveRelativePaths(t *testing.T) {
	base := t.TempDir()
	content := []byte(`host = "0.0.0.0"
port = 9001
log_api_key = "secret-key"
data_dir = "state"
excel_dir = "xlsx"
timezone = "UTC"
sync_interval_seconds = 9
log_enabled = false
log_level = "debug"
log_dir = "run-logs"
log_body = false
mask_fields = ["secret", "phone"]
`)
	if err := os.WriteFile(filepath.Join(base, "config.toml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(base)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Address() != "0.0.0.0:9001" {
		t.Fatalf("Address() = %q", cfg.Address())
	}
	if cfg.DataDir != filepath.Join(base, "state") {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.ExcelDir != filepath.Join(base, "xlsx") {
		t.Fatalf("ExcelDir = %q", cfg.ExcelDir)
	}
	if cfg.LogEnabled {
		t.Fatal("LogEnabled = true, want false")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.LogDir != filepath.Join(base, "run-logs") {
		t.Fatalf("LogDir = %q", cfg.LogDir)
	}
	if cfg.LogBody {
		t.Fatal("LogBody = true, want false")
	}
}

func TestBlankAPIKeyIsRejected(t *testing.T) {
	base := t.TempDir()
	content := []byte(`log_api_key = ""`)
	if err := os.WriteFile(filepath.Join(base, "config.toml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(base)
	if err == nil {
		t.Fatal("expected blank API key to fail")
	}
}

func TestInvalidLogLevelIsRejected(t *testing.T) {
	base := t.TempDir()
	content := []byte(`log_level = "trace"`)
	if err := os.WriteFile(filepath.Join(base, "config.toml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(base)
	if err == nil {
		t.Fatal("expected invalid log level to fail")
	}
}

func TestLogLevelIgnoresCaseAndSpaces(t *testing.T) {
	base := t.TempDir()
	content := []byte(`log_level = " DEBUG "`)
	if err := os.WriteFile(filepath.Join(base, "config.toml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(base)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
}
