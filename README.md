# bubble-note

`bubble-note` is a simple, local-first note-taking application for the terminal. Notes are written and stored as Markdown, with no login or account required.

## Features

- Create, edit, and delete notes.
- Store notes locally in SQLite.
- Use generated names such as `quiet-cat` for new notes, then rename them.
- Add tags to notes.
- Search Markdown content.
- Filter by tags and update dates.
- Keep immutable note revisions with configurable retention.
- Work entirely offline.
- Use Catppuccin color themes with configurable semantic color overrides.

Remote S3-compatible storage is planned as an optional layer. It is not required for local use.

## Install

Install the latest version with Go:

```sh
go install github.com/norriswu0/bubble-note/cmd/bubble-note@latest
```

Make sure Go's binary directory is on your `PATH`, then run:

```sh
bubble-note
```

To install from a local checkout instead:

```sh
make install
```

To build a standalone binary in `bin/`:

```sh
make build
```

## Usage

From the note list:

- `n` creates a note.
- `Enter` or `e` edits the selected note.
- `/` starts search.
- `j`/`k` or arrow keys navigate.
- `d` deletes the selected note.
- `q` quits the application.

While editing:

- Notes open in Markdown body editing mode.
- `Tab` inserts two spaces for indentation in the body.
- `Esc` leaves body editing and enters field navigation mode.
- In field navigation mode, `Tab` moves to the next field and `Shift-Tab` moves to the previous field.
- Title and tags can be edited directly when focused.
- `Enter` on the body field starts body editing mode again.
- `Ctrl-S` saves only when the title, tags, or body changed.
- From field navigation mode, `Esc` or `Ctrl-C` starts the exit flow when changes are unsaved.
- `Ctrl-C` starts the exit flow directly from body editing mode.
- Choose `s` to save, `d` to discard, or `c`/`Esc` to cancel.

There is no autosave. Saving a changed note creates a new immutable revision and keeps the note open.

## Status

The first version stores notes in SQLite. Notes use immutable revisions, with a configurable retention limit (14 by default). S3 support is intentionally reserved behind a future remote-store boundary and is not required to use the app.

## Search Filters

Use these optional terms in the search prompt:

- `tag:work` filters by tag.
- `after:2026-01-01` filters notes updated on or after a date.
- `before:2026-02-01` filters notes updated before a date.

Content terms can be combined with these filters.

## Data And Configuration

The application creates its database and configuration under the platform's user config directory. On Linux, the configuration file is `~/.config/bubble-note/config.yaml`. It is created with defaults on first launch and existing configuration is never overwritten. Edit this file directly to update bubble-note's settings, then restart the application.

Configure revision retention with:

```yaml
revision_retention: 14
```

The default is to keep the latest 14 revisions per note.

Configure body indentation width with `indent_spaces`:

```yaml
indent_spaces: 2
```

When editing the Markdown body, `Tab` inserts the configured number of spaces.

The default UI theme is Catppuccin Mocha. Available flavors are `latte`, `frappe`, `macchiato`, and `mocha`:

```yaml
theme:
  preset: catppuccin
  flavor: mocha
  overrides:
    primary: "#F5A97F"
    danger: "#ED8796"
```

Theme changes take effect when bubble-note is restarted.

## Development

```sh
go run ./cmd/bubble-note
go test ./...
go build ./cmd/bubble-note
```

Useful Make targets:

```sh
make run
make test
make check
make clean
```
