package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultRevisionRetention = 14
const DefaultIndentSpaces = 2

type Config struct {
	RevisionRetention int         `yaml:"revision_retention"`
	IndentSpaces      int         `yaml:"indent_spaces"`
	Theme             ThemeConfig `yaml:"theme"`
}

type ThemeConfig struct {
	Preset    string            `yaml:"preset"`
	Flavor    string            `yaml:"flavor"`
	Overrides map[string]string `yaml:"overrides"`
}

func Default() Config {
	return Config{
		RevisionRetention: DefaultRevisionRetention,
		IndentSpaces:      DefaultIndentSpaces,
		Theme: ThemeConfig{
			Preset: "catppuccin",
			Flavor: "mocha",
		},
	}
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
	if cfg.IndentSpaces < 1 {
		return cfg, fmt.Errorf("indent_spaces must be at least 1")
	}
	return cfg, nil
}

func LoadOrCreate(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := Save(path, cfg); err != nil {
			return cfg, err
		}
	} else if err != nil {
		return cfg, fmt.Errorf("check config: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "bubble-note"), nil
}
