package view

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/norriswu0/bubble-note/internal/theme"
)

// Note is the display data needed by note views. It deliberately excludes
// persistence details such as revision identifiers.
type Note struct {
	Title   string
	Content string
}

func RenderMarkdown(note Note, width int, palette theme.Palette) string {
	if width < 20 {
		width = 20
	}
	renderer, err := glamour.NewTermRenderer(
		// Avoid Glamour's terminal background probe in the alternate-screen TUI.
		glamour.WithStyles(markdownStyle(palette)),
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

// markdownStyle maps the app's palette onto Glamour's style config so rendered
// Markdown matches the selected theme instead of Glamour's default dark style.
func markdownStyle(p theme.Palette) ansi.StyleConfig {
	color := func(v string) *string { return &v }
	bold := func(v bool) *bool { return &v }

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: color(p.Text)},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: color(p.Primary), Bold: bold(true)},
		},
		Strong: ansi.StylePrimitive{Bold: bold(true)},
		Emph:   ansi.StylePrimitive{Italic: bold(true)},
		Link: ansi.StylePrimitive{
			Color:     color(p.Secondary),
			Underline: bold(true),
		},
		LinkText: ansi.StylePrimitive{
			Color:     color(p.Secondary),
			Underline: bold(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: color(p.Secondary)},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: color(p.Secondary)},
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: color(p.Muted)},
		},
		HorizontalRule: ansi.StylePrimitive{Color: color(p.Border)},
	}
}
