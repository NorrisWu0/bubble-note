package cli

import (
	"fmt"
	"io"
)

// PrintHelp writes the top-level usage for the bubble-note command.
func PrintHelp(w io.Writer) {
	fmt.Fprint(w, `bubble-note — local-first terminal notes stored as files

Usage:
  bubble-note [flags]             Start the app
  bubble-note migrate [flags]     Migrate legacy SQLite notes into files

Commands:
  migrate     Migrate notes from a legacy SQLite database (one-shot)

Flags:
  --notes-dir string   Open notes from this directory (overrides configured notes_dir)
  -h, --help           Show this help

Migrate flags:
  --db string          Path to the legacy notes.db (default: ~/.config/bubble-note/notes.db)
  --notes-dir string   Destination notes directory (default: resolved from config)
  --dry-run            List what would be migrated without writing
`)
}
