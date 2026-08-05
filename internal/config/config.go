package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultRevisionRetention = 14

type Config struct {
	RevisionRetention int `yaml:"revision_retention"`
}

func Default() Config {
	return Config{RevisionRetention: DefaultRevisionRetention}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.RevisionRetention < 1 {
		return cfg, fmt.Errorf("revision_retention must be at least 1")
	}
	return cfg, nil
}

func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "bubble-note"), nil
}
