package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/theme"
)

type NoteRow struct {
	Title    string
	Path     string
	Updated  string
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
	GitStatus   string
}

type ReaderModel struct {
	Width  int
	Height int
	Title  string
	Tags   []string
	Body   string
}

type FormModel struct {
	Width       int
	Height      int
	Heading     string
	TitleView   string
	TagsView    string
	TitleActive bool
	TagsActive  bool
	Status      string
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
		body.WriteString("No notes found. Press a to create one.\n")
	}
	for _, row := range model.Rows {
		marker := "  "
		if row.Selected {
			marker = ">>"
		}
		pathLabel := row.Path
		if pathLabel == "" {
			pathLabel = "(root)"
		}
		line := fmt.Sprintf("%s %-24s %-24s %s", marker, truncate(row.Title, 24), truncate(pathLabel, 24), row.Updated)
		if row.Selected {
			line = selected(line, palette)
		}
		body.WriteString(line + "\n")
		if row.Selected && len(row.Tags) > 0 {
			body.WriteString("   " + tag("#"+strings.Join(row.Tags, " #"), palette) + "\n")
		}
	}
	if body.Len() > 0 {
		body.WriteString("\n")
	}
	noteContent := body.String()
	if searchLine != "" {
		noteContent = searchLine + "\n\n" + noteContent
	}
	panelHeight := contentHeight(model.Height)
	notePanel := panel(noteContent, panelHeight-1, true, model.Width, palette)
	footer := "a new   enter view   e edit   t tags   m move   / search   d delete   r refresh   g git   s settings   q quit"
	if model.GitStatus != "" {
		footer = model.GitStatus + "   |   " + footer
	}
	if model.Status != "" {
		footer = model.Status + "   |   " + footer
	}
	return header + "\n" + notePanel + "\n" + footBar(footer, model.Width, palette)
}

func RenderForm(model FormModel, palette theme.Palette) string {
	header := topBar(model.Width, "", model.Heading, palette)
	metadata := field("TITLE", model.TitleView, model.TitleActive, palette) + "\n" + field("TAGS", model.TagsView, model.TagsActive, palette)
	body := panel(metadata, formHeight(model.Height), model.TitleActive || model.TagsActive, model.Width, palette)
	footer := "enter save   tab next   shift-tab previous   esc cancel"
	if model.Status != "" {
		footer = model.Status + "   |   " + footer
	}
	return header + "\n" + body + "\n" + footBar(footer, model.Width, palette)
}

func RenderReader(model ReaderModel, palette theme.Palette) string {
	header := topBar(model.Width, model.Title, "VIEW", palette)
	metadata := field("TAGS", tagList(model.Tags, palette), false, palette)
	body := panel(model.Body, contentHeight(model.Height), false, model.Width, palette)
	return header + "\n" + metadata + "\n" + body + "\n" + footBar("e edit   t tags   d delete   up/down scroll   esc back   q quit", model.Width, palette)
}

type CreateNoteModel struct {
	Width       int
	Height      int
	PathView    string
	TitleView   string
	TagsView    string
	PathActive  bool
	TitleActive bool
	TagsActive  bool
	Error       string
}

func RenderCreateNoteModal(model CreateNoteModel, palette theme.Palette) string {
	body := field("PATH", model.PathView, model.PathActive, palette) + "\n\n" +
		field("TITLE", model.TitleView, model.TitleActive, palette) + "\n\n" +
		field("TAGS", model.TagsView, model.TagsActive, palette)
	if model.Error != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Danger)).Render(model.Error)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(palette.Primary)).Render("NEW NOTE")
	modal := title + "\n\n" + body + "\n\n" + muted("enter create   esc cancel", palette)
	modalWidth := model.Width - 10
	if modalWidth < 40 {
		modalWidth = 40
	}
	width := model.Width
	if width <= 0 {
		width = 80
	}
	height := model.Height
	if height <= 0 {
		height = 24
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel(modal, 0, true, modalWidth, palette), lipgloss.WithWhitespaceChars(" "))
}

type MoveNoteModel struct {
	Width      int
	Height     int
	PathView   string
	PathActive bool
	Error      string
	Hint       string
	HintOK     bool
}

func RenderMoveNoteModal(model MoveNoteModel, palette theme.Palette) string {
	body := field("PATH", model.PathView, model.PathActive, palette)
	if model.Error != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Danger)).Render(model.Error)
	} else if model.Hint != "" {
		color := palette.Secondary
		if !model.HintOK {
			color = palette.Danger
		}
		body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(model.Hint)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(palette.Primary)).Render("MOVE NOTE")
	modal := title + "\n\n" + body + "\n\n" + muted("enter move   esc cancel", palette)
	modalWidth := model.Width - 10
	if modalWidth < 40 {
		modalWidth = 40
	}
	width := model.Width
	if width <= 0 {
		width = 80
	}
	height := model.Height
	if height <= 0 {
		height = 24
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel(modal, 0, true, modalWidth, palette), lipgloss.WithWhitespaceChars(" "))
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
	return style.Render(line)
}

func footBar(text string, width int, palette theme.Palette) string {
	width = viewWidth(width)
	text = truncate(text, width-1)
	style := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color(palette.Muted)).BorderTop(true).BorderForeground(lipgloss.Color(palette.Border))
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

func formHeight(height int) int {
	if height <= 0 {
		return 5
	}
	value := height - 6
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
