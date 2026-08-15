package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultRevisionRetention = 14
const DefaultIndentSpaces = 2
const MinimumRevisionRetention = 14

type Config struct {
	NotesDir          string        `yaml:"notes_dir"`
	Editor            string        `yaml:"editor"`
	GitClient         string        `yaml:"git_client"`
	RevisionRetention int           `yaml:"revision_retention"`
	IndentSpaces      int           `yaml:"indent_spaces"`
	Theme             ThemeConfig   `yaml:"theme"`
	Storage           StorageConfig `yaml:"storage"`
}

type ThemeConfig struct {
	Preset    string            `yaml:"preset"`
	Flavor    string            `yaml:"flavor"`
	Overrides map[string]string `yaml:"overrides"`
}

type StorageConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Region          string `yaml:"region"`
	Bucket          string `yaml:"bucket"`
	Prefix          string `yaml:"prefix"`
	PathStyle       bool   `yaml:"path_style"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token,omitempty"`
}

type legacySyncConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Region          string `yaml:"region"`
	Bucket          string `yaml:"bucket"`
	Prefix          string `yaml:"prefix"`
	PathStyle       bool   `yaml:"path_style"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token"`
}

func Default() Config {
	return Config{
		GitClient:         "lazygit",
		RevisionRetention: DefaultRevisionRetention,
		IndentSpaces:      DefaultIndentSpaces,
		Theme: ThemeConfig{
			Preset: "catppuccin",
			Flavor: "mocha",
		},
		Storage: StorageConfig{
			Region: "us-east-1",
		},
	}
}

// EditorCommand returns the editor command to launch, honouring the config value,
// then the EDITOR environment variable, then nvim.
func (c Config) EditorCommand() string {
	if strings.TrimSpace(c.Editor) != "" {
		return c.Editor
	}
	if editor := os.Getenv("EDITOR"); strings.TrimSpace(editor) != "" {
		return editor
	}
	return "nvim"
}

// GitClientCommand returns the git TUI client to launch, defaulting to lazygit.
func (c Config) GitClientCommand() string {
	if strings.TrimSpace(c.GitClient) != "" {
		return c.GitClient
	}
	return "lazygit"
}

// NotesDirectory resolves the notes directory, defaulting to the app directory
// under the user config dir.
func (c Config) NotesDirectory() (string, error) {
	if strings.TrimSpace(c.NotesDir) != "" {
		return c.NotesDir, nil
	}
	dir, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "notes"), nil
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
	var keys map[string]yaml.Node
	if err := yaml.Unmarshal(data, &keys); err == nil {
		if _, hasStorage := keys["storage"]; !hasStorage {
			if syncNode, hasSync := keys["sync"]; hasSync {
				var legacy legacySyncConfig
				if err := syncNode.Decode(&legacy); err == nil {
					cfg.Storage = StorageConfig{Endpoint: legacy.Endpoint, Region: legacy.Region, Bucket: legacy.Bucket, Prefix: legacy.Prefix, PathStyle: legacy.PathStyle, AccessKeyID: legacy.AccessKeyID, SecretAccessKey: legacy.SecretAccessKey, SessionToken: legacy.SessionToken}
				}
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.RevisionRetention < MinimumRevisionRetention {
		return fmt.Errorf("revision_retention must be at least %d", MinimumRevisionRetention)
	}
	if c.IndentSpaces < 1 {
		return fmt.Errorf("indent_spaces must be at least 1")
	}
	return nil
}

func (c StorageConfig) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("storage.region is required")
	}
	if c.Bucket == "" {
		return fmt.Errorf("storage.bucket is required")
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return fmt.Errorf("storage.access_key_id and storage.secret_access_key are required")
	}
	return nil
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
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".config.yaml-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary config: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
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
