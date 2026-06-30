package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Host                string   `toml:"host"`
	Port                int      `toml:"port"`
	LogAPIKey           string   `toml:"log_api_key"`
	DataDir             string   `toml:"data_dir"`
	ExcelDir            string   `toml:"excel_dir"`
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
		ExcelDir:            filepath.Join(baseDir, "logs"),
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
