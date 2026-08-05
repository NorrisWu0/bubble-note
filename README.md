# bubble-note

`bubble-note` is a local-first Markdown note-taking TUI built with Go and Bubble Tea.

## Status

The first version stores notes in SQLite. Notes use immutable revisions, with a configurable retention limit (14 by default). S3 support is intentionally reserved behind a future remote-store boundary and is not required to use the app.

## Development

```sh
go run ./cmd/bubble-note
go test ./...
go build ./cmd/bubble-note
```

The application creates its database and configuration under the platform's user config directory. A `config.yaml` file can configure revision retention.

While searching, use these optional terms in the search prompt:

- `tag:work` filters by tag.
- `after:2026-01-01` filters notes updated on or after a date.
- `before:2026-02-01` filters notes updated before a date.

Content terms can be combined with these filters. Use `Ctrl-S` to create a revision; notes are not autosaved.

When editing, changes to the title, tags, or body mark the note as unsaved. `Ctrl-S` saves only when changes exist and keeps the note open. Leaving a changed note with `Esc`, `q`, or `Ctrl-C` asks whether to save, discard, or cancel.
