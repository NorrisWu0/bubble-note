package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/theme"
)

type NoteRow struct {
	Title    string
	Updated  string
	Excerpt  string
	Tags     []string
	Selected bool
}

type ListModel struct {
	Width       int
	Height      int
	Count       int
	Search      string
	SearchInput string
	Rows        []NoteRow
	Status      string
}

type EditorModel struct {
	Width          int
	Height         int
	Title          string
	TitleView      string
	TagsView       string
	BodyView       string
	TitleActive    bool
	TagsActive     bool
	BodyActive     bool
	State          string
	Status         string
	ConfirmingExit bool
}

type ReaderModel struct {
	Width  int
	Height int
	Title  string
	Tags   []string
	Body   string
}

func RenderList(model ListModel, palette theme.Palette) string {
	header := topBar(model.Width, "", fmt.Sprintf("%d notes", model.Count), palette)
	searchLine := ""
	if model.SearchInput != "" {
		searchLine = model.SearchInput
	} else if model.Search != "" {
		searchLine = muted(model.Search, palette)
	}
	var body strings.Builder
	if len(model.Rows) == 0 {
		body.WriteString("No notes found. Press n to create one.\n")
	}
	for _, row := range model.Rows {
		marker := "  "
		if row.Selected {
			marker = ">>"
		}
		line := fmt.Sprintf("%s %-28s %s", marker, truncate(row.Title, 28), row.Updated)
		if row.Selected {
			line = selected(line, palette)
		}
		body.WriteString(line + "\n")
		if row.Selected {
			body.WriteString("   " + muted(truncate(strings.ReplaceAll(row.Excerpt, "\n", " "), contentWidth(model.Width)-8), palette) + "\n")
			if len(row.Tags) > 0 {
				body.WriteString("   " + tag("#"+strings.Join(row.Tags, " #"), palette) + "\n")
			}
		}
	}
	content := body.String()
	if searchLine != "" {
		content = searchLine + "\n\n" + content
	}
	footer := "n new   enter view   e edit   / search   d delete   q quit"
	if model.Status != "" {
		footer = model.Status + "   |   " + footer
	}
	return header + "\n" + panel(content, contentHeight(model.Height), false, model.Width, palette) + "\n" + footBar(footer, model.Width, palette)
}

func RenderEditor(model EditorModel, palette theme.Palette) string {
	header := topBar(model.Width, model.Title, model.State, palette)
	metadata := field("TITLE", model.TitleView, model.TitleActive, palette) + "\n" + field("TAGS", model.TagsView, model.TagsActive, palette)
	body := panel(field("BODY", model.BodyView, model.BodyActive, palette), editorHeight(model.Height), model.BodyActive, model.Width, palette)
	footer := "ctrl-s save   tab indent   shift-tab previous   esc view"
	if model.Status != "" {
		footer = model.Status + "   |   " + footer
	}
	if model.ConfirmingExit {
		footer = "UNSAVED CHANGES   [s] Save   [d] Discard   [c] Cancel"
	}
	return header + "\n" + metadata + "\n\n" + body + "\n" + footBar(footer, model.Width, palette)
}

func RenderReader(model ReaderModel, palette theme.Palette) string {
	header := topBar(model.Width, model.Title, "SAVED", palette)
	metadata := field("TAGS", tagList(model.Tags, palette), false, palette)
	body := panel(model.Body, contentHeight(model.Height), false, model.Width, palette)
	return header + "\n" + metadata + "\n" + body + "\n" + footBar("e edit   up/down scroll   esc back   q quit", model.Width, palette)
}

func topBar(width int, noteTitle, state string, palette theme.Palette) string {
	width = viewWidth(width)
	left := " bubble-note"
	if noteTitle != "" {
		left += " / " + truncate(noteTitle, width/2)
	}
	right := state
	if right != "" {
		right = "[" + right + "]"
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	style := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color(palette.Text)).Background(lipgloss.Color(palette.Surface)).Bold(true)
	if state == "UNSAVED" {
		style = style.Foreground(lipgloss.Color(palette.Primary))
	}
	return style.Render(line)
}

func footBar(text string, width int, palette theme.Palette) string {
	width = viewWidth(width)
	text = truncate(text, width-1)
	style := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color(palette.Muted)).BorderTop(true).BorderForeground(lipgloss.Color(palette.Border))
	if strings.HasPrefix(text, "UNSAVED") {
		style = style.Foreground(lipgloss.Color(palette.Primary)).Bold(true)
	}
	return style.Render(" " + text)
}

func field(label, value string, active bool, palette theme.Palette) string {
	color := palette.Muted
	if active {
		color = palette.Primary
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(active).Render(label) + "\n" + value
}

func panel(content string, height int, active bool, width int, palette theme.Palette) string {
	border := palette.Border
	if active {
		border = palette.Primary
	}
	style := lipgloss.NewStyle().Width(contentWidth(width)).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(border)).Padding(0, 1)
	if height > 0 {
		style = style.Height(height)
	}
	return style.Render(content)
}

func selected(value string, palette theme.Palette) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Selected)).Bold(true).Render(value)
}

func muted(value string, palette theme.Palette) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Muted)).Render(value)
}

func tag(value string, palette theme.Palette) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Secondary)).Render(value)
}

func tagList(tags []string, palette theme.Palette) string {
	if len(tags) == 0 {
		return muted("no tags", palette)
	}
	return tag("#"+strings.Join(tags, " #"), palette)
}

func viewWidth(width int) int {
	if width <= 0 {
		return 80
	}
	return width
}

func contentWidth(width int) int {
	value := viewWidth(width) - 2
	if value < 16 {
		return 16
	}
	return value
}

func contentHeight(height int) int {
	if height <= 0 {
		return 12
	}
	if height-5 < 3 {
		return 3
	}
	return height - 5
}

func editorHeight(height int) int {
	if height <= 0 {
		return 5
	}
	value := height - 10
	if value < 5 {
		return 5
	}
	return value
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if width <= 0 {
		return ""
	}
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
