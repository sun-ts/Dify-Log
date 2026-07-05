package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Host                string   `toml:"host"`
	Port                int      `toml:"port"`
	LogAPIKey           string   `toml:"log_api_key"`
	DataDir             string   `toml:"data_dir"`
	ExcelDir            string   `toml:"excel_dir"`
	LogEnabled          bool     `toml:"log_enabled"`
	LogLevel            string   `toml:"log_level"`
	LogDir              string   `toml:"log_dir"`
	LogBody             bool     `toml:"log_body"`
	Timezone            string   `toml:"timezone"`
	SyncIntervalSeconds int      `toml:"sync_interval_seconds"`
	MaskFields          []string `toml:"mask_fields"`
}

func Default(baseDir string) Config {
	return Config{
		Host:                "127.0.0.1",
		Port:                8000,
		LogAPIKey:           "dev-log-api-key",
		DataDir:             filepath.Join(baseDir, "data"),
		ExcelDir:            filepath.Join(baseDir, "data", "excel"),
		LogEnabled:          true,
		LogLevel:            "info",
		LogDir:              filepath.Join(baseDir, "logs"),
		LogBody:             true,
		Timezone:            "Asia/Shanghai",
		SyncIntervalSeconds: 5,
		MaskFields:          []string{"password", "token", "api_key", "phone"},
	}
}

func Load(baseDir string) (Config, error) {
	cfg := Default(baseDir)
	path := filepath.Join(baseDir, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
		return cfg, cfg.Validate()
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.DataDir = resolvePath(baseDir, cfg.DataDir)
	cfg.ExcelDir = resolvePath(baseDir, cfg.ExcelDir)
	cfg.LogDir = resolvePath(baseDir, cfg.LogDir)
	if level, err := parseLogLevel(cfg.LogLevel); err == nil {
		cfg.LogLevel = level
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.LogAPIKey == "" {
		return errors.New("log_api_key must not be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.SyncIntervalSeconds <= 0 {
		return fmt.Errorf("sync_interval_seconds must be greater than 0")
	}
	if c.Timezone == "" {
		return errors.New("timezone must not be empty")
	}
	if _, err := parseLogLevel(c.LogLevel); err != nil {
		return err
	}
	return nil
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func resolvePath(baseDir, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(baseDir, value)
}

func parseLogLevel(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error", "info", "debug":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("log_level must be one of error, info, debug")
	}
}
