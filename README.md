# bubble-note

`bubble-note` is a local-first, terminal note-taking app. Notes live as plain
Markdown files in a git repository, so you can edit them with nvim and version
control them with git.

## Features

- Notes are plain files: `<notes_dir>/<title>/README.md` (body) and `manifest.json` (metadata).
- Edit note content in your editor (nvim), launched from bubble-note.
- Edit titles and tags in a small built-in form.
- Full-text search (SQLite FTS5 index, rebuilt from files).
- Filter by tag and update dates.
- Catppuccin color themes.
- Optional git integration: see status in the footer, launch lazygit for full git operations.

## Install

```sh
go install github.com/norriswu0/bubble-note/cmd/bubble-note@latest
```

Run with `bubble-note`.

## Usage

From the note list:

- `n` creates a note (title/tags form, then opens your editor).
- `Enter` opens the selected note in rendered view.
- `e` opens the selected note's body in your editor.
- `t` edits the title and tags.
- `/` starts search.
- `j`/`k` or arrow keys navigate.
- `d` deletes the selected note.
- `r` re-scans the notes directory and rebuilds the search index.
- `g` opens lazygit in the notes directory (initializes the repo first if needed).
- `s` opens settings (theme and notes directory).
- `q` quits.

While viewing a note: `e` edit body, `t` edit tags, `d` delete, `Esc` back, `Up`/`Down` scroll.

## Search Filters

- `tag:work` filters by tag.
- `after:2026-01-01` filters notes updated on or after a date.
- `before:2026-02-01` filters notes updated before a date.

## Configuration

The config file lives at `~/.config/bubble-note/config.yaml`:

```yaml
notes_dir: ""            # default: ~/.config/bubble-note/notes
editor: ""               # default: $EDITOR, falling back to nvim
git_client: "lazygit"    # git TUI launched with 'g'

theme:
  preset: catppuccin
  flavor: mocha
```

Notes are stored as directories under `notes_dir`. Each directory contains a
`README.md` (the Markdown body) and a `manifest.json` (title, tags, timestamps).
Point `notes_dir` at a git repository to version control your notes; `git` is the
history and sync mechanism. Changing `notes_dir` takes effect on the next launch.

## Migrating from the legacy SQLite database

Older versions stored notes in `~/.config/bubble-note/notes.db`. Migrate them to
files with a one-shot command:

```sh
bubble-note migrate --dry-run   # preview what would be migrated
bubble-note migrate             # write notes into notes_dir
```

This migrates the latest content of each note, preserving IDs and timestamps,
and leaves the legacy `notes.db` untouched.

## Requirements

`bubble-note` requires an editor and a git TUI client. It will prompt you to
install them if missing:

- Editor: `nvim` (or anything, via the `editor` config or `$EDITOR`).
- Git TUI: `lazygit`.

## Development

```sh
go run ./cmd/bubble-note
go test ./...
go build ./cmd/bubble-note
```

Useful Make targets: `make run`, `make test`, `make check`, `make clean`.

## Architecture

- `internal/notes` contains note schemas, business rules, and the note service.
- `internal/notes/files` is the filesystem source of truth.
- `internal/database/sqlite` is the rebuildable FTS search index.
- `internal/store` combines the two behind the notes repository.
- `internal/git` wraps the git CLI for status detection and init.
- `internal/app` coordinates Bubble Tea events and view rendering.
- `internal/view` contains display models and terminal presentation.
