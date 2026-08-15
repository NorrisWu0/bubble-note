package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/migrate"
)

// RunMigrate migrates notes from the legacy SQLite database into the filesystem
// notes directory. It is a one-shot, non-destructive command: the legacy
// database is left untouched.
func RunMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dbPath := fs.String("db", "", "path to the legacy notes.db (default: ~/.config/bubble-note/notes.db)")
	notesDir := fs.String("notes-dir", "", "destination notes directory (default: resolved from config)")
	dryRun := fs.Bool("dry-run", false, "list what would be migrated without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir, err := config.UserPath()
	if err != nil {
		return err
	}
	if *dbPath == "" {
		*dbPath = filepath.Join(dir, "notes.db")
	}
	if *notesDir == "" {
		cfg, err := config.LoadOrCreate(filepath.Join(dir, "config.yaml"))
		if err != nil {
			return err
		}
		*notesDir, err = cfg.NotesDirectory()
		if err != nil {
			return err
		}
	}

	notes, err := migrate.ReadLegacy(*dbPath)
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		fmt.Println("No legacy notes found to migrate.")
		return nil
	}
	fmt.Printf("Found %d legacy note(s) to migrate into %s\n", len(notes), *notesDir)
	if *dryRun {
		for _, note := range notes {
			fmt.Printf("  - %s (%s)\n", note.Title, note.ID)
		}
		return nil
	}

	exported, skipped, err := migrate.Export(*notesDir, notes)
	if err != nil {
		return err
	}
	fmt.Printf("Migrated %d note(s) to %s\n", exported, *notesDir)
	if skipped > 0 {
		fmt.Printf("Skipped %d note(s) that already exist\n", skipped)
	}
	fmt.Println("The legacy database was left untouched.")
	return nil
}
