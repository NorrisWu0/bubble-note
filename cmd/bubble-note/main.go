package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/app"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/database/sqlite"
	"github.com/norriswu0/bubble-note/internal/storage"
	"github.com/norriswu0/bubble-note/internal/theme"
)

func main() {
	dir, err := config.UserPath()
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fatal(fmt.Errorf("create app directory: %w", err))
	}
	cfg, err := config.LoadOrCreate(filepath.Join(dir, "config.yaml"))
	if err != nil {
		fatal(err)
	}
	palette, err := theme.Resolve(cfg.Theme)
	if err != nil {
		fatal(err)
	}
	store, err := sqlite.Open(filepath.Join(dir, "notes.db"), cfg.RevisionRetention)
	if err != nil {
		fatal(err)
	}
	defer store.Close()
	var checker storage.ConnectionChecker
	var syncer storage.NoteSyncer
	if cfg.Storage.Validate() == nil {
		client, clientErr := storage.NewS3(cfg.Storage)
		if clientErr == nil {
			checker = client
			syncer = client
		}
	}

	configPath := filepath.Join(dir, "config.yaml")
	program := tea.NewProgram(app.NewWithConfig(store, palette, cfg, configPath, checker, func(retention int) error {
		return store.SetRevisionRetention(retention)
	}, syncer), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "bubble-note:", err)
	os.Exit(1)
}
