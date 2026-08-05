package view

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// Note is the display data needed by note views. It deliberately excludes
// persistence details such as revision identifiers.
type Note struct {
	Title   string
	Content string
}

func RenderMarkdown(note Note, width int) string {
	if width < 20 {
		width = 20
	}
	renderer, err := glamour.NewTermRenderer(
		// Avoid Glamour's terminal background probe in the alternate-screen TUI.
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return note.Content
	}
	rendered, err := renderer.Render(note.Content)
	if err != nil {
		return note.Content
	}
	return strings.TrimSpace(rendered)
}
