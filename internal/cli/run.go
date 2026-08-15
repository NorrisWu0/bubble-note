package cli

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/app"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/store"
	"github.com/norriswu0/bubble-note/internal/theme"
)

// Run starts the bubble-note application.
func Run() error {
	dir, err := config.UserPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create app directory: %w", err)
	}
	cfg, err := config.LoadOrCreate(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return err
	}
	palette, err := theme.Resolve(cfg.Theme)
	if err != nil {
		return err
	}
	notesDir, err := cfg.NotesDirectory()
	if err != nil {
		return err
	}
	noteStore, err := store.New(notesDir, filepath.Join(dir, "index.db"))
	if err != nil {
		return err
	}
	defer noteStore.Close()

	configPath := filepath.Join(dir, "config.yaml")
	program := tea.NewProgram(app.New(noteStore, cfg, configPath, palette), tea.WithAltScreen())
	_, err = program.Run()
	return err
}
