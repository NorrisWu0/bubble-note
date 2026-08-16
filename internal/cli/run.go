package cli

import (
	"errors"
	"flag"
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
func Run(args []string) error {
	fs := flag.NewFlagSet("bubble-note", flag.ContinueOnError)
	notesDirFlag := fs.String("notes-dir", "", "open notes from this directory (overrides the configured notes_dir)")
	fs.Usage = func() { PrintHelp(fs.Output()) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

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
	if *notesDirFlag != "" {
		notesDir = *notesDirFlag
		cfg.NotesDir = notesDir
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
