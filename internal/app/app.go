package app

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/domain"
	"github.com/norriswu0/bubble-note/internal/theme"
)

type screen int

const (
	listScreen screen = iota
	viewScreen
	editScreen
)

type Model struct {
	store          domain.NoteStore
	notes          []domain.Note
	cursor         int
	screen         screen
	search         textinput.Model
	title          textinput.Model
	tags           textinput.Model
	editor         textarea.Model
	viewer         viewport.Model
	viewNote       domain.Note
	searching      bool
	editingID      string
	dirty          bool
	status         string
	focus          int
	width          int
	height         int
	palette        theme.Palette
	indentSpaces   int
	confirmingExit bool
	saveThenExit   bool
	savedTitle     string
	savedTags      string
	savedContent   string
	savedNote      domain.Note
}

func New(store domain.NoteStore, palettes ...theme.Palette) Model {
	search := textinput.New()
	search.Placeholder = "search notes"
	search.Prompt = "/ "
	title := textinput.New()
	title.Prompt = "Title: "
	tags := textinput.New()
	tags.Prompt = "Tags:  "
	editor := textarea.New()
	editor.Prompt = "  "
	editor.ShowLineNumbers = true
	palette := theme.Default()
	if len(palettes) > 0 {
		palette = palettes[0]
	}
	viewer := viewport.New(80, 12)
	return Model{store: store, search: search, title: title, tags: tags, editor: editor, viewer: viewer, palette: palette, indentSpaces: config.DefaultIndentSpaces}
}

func NewWithSettings(store domain.NoteStore, palette theme.Palette, indentSpaces int) Model {
	model := New(store, palette)
	if indentSpaces > 0 {
		model.indentSpaces = indentSpaces
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	notes, err := m.store.ListNotes(parseFilter(m.search.Value()))
	if err != nil {
		return errMsg{err}
	}
	return notesLoadedMsg(notes)
}

type notesLoadedMsg []domain.Note

type savedMsg struct {
	note domain.Note
}

type errMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeInputs()
	case notesLoadedMsg:
		m.notes = msg
	case savedMsg:
		m.savedTitle = msg.note.Title
		m.savedTags = strings.Join(msg.note.Tags, ", ")
		m.savedContent = msg.note.Content
		m.savedNote = msg.note
		m.title.SetValue(m.savedTitle)
		m.tags.SetValue(m.savedTags)
		m.editor.SetValue(m.savedContent)
		m.editingID = msg.note.ID
		m.dirty = false
		m.status = "Saved " + msg.note.Title
		if m.saveThenExit {
			m.saveThenExit = false
			m.confirmingExit = false
			m.leaveEditor()
		}
		return m, m.load
	case errMsg:
		m.status = "Error: " + msg.err.Error()
		m.saveThenExit = false
		m.confirmingExit = false
	}

	if m.confirmingExit {
		return m.updateExitPrompt(msg)
	}
	if m.screen == editScreen {
		return m.updateEditor(msg)
	}
	if m.screen == viewScreen {
		return m.updateView(msg)
	}
	if m.searching {
		return m.updateSearch(msg)
	}
	return m.updateList(msg)
}

func (m Model) updateExitPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "s":
		m.saveThenExit = true
		return m, m.save
	case "d":
		m.confirmingExit = false
		m.leaveEditor()
	case "c", "esc":
		m.confirmingExit = false
	}
	return m, nil
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.notes)-1 {
			m.cursor++
		}
	case "n":
		m.beginEdit(domain.Note{Title: generatedTitle(), Content: ""})
	case "enter":
		if len(m.notes) > 0 {
			m.beginView(m.notes[m.cursor])
		}
	case "e":
		if len(m.notes) > 0 {
			m.beginEdit(m.notes[m.cursor])
		}
	case "d":
		if len(m.notes) > 0 {
			if err := m.store.DeleteNote(m.notes[m.cursor].ID); err != nil {
				m.status = "Error: " + err.Error()
			} else {
				m.status = "Deleted"
				return m, m.load
			}
		}
	case "/":
		m.searching = true
		m.search.Focus()
		return m, textinput.Blink
	case "esc":
		m.search.Reset()
		return m, m.load
	}
	return m, nil
}

func (m Model) updateView(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "e":
			m.beginEdit(m.viewNote)
			return m, nil
		case "esc":
			m.screen = listScreen
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.searching = false
			m.search.Blur()
			m.search.Reset()
			return m, m.load
		case "enter":
			m.searching = false
			m.search.Blur()
			return m, m.load
		}
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, tea.Batch(cmd, m.load)
}

func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			if m.dirty {
				m.confirmingExit = true
				m.blurEditor()
				return m, nil
			}
			if key.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.leaveEditor()
			return m, nil
		case "ctrl+s":
			if !m.dirty {
				m.status = "No changes to save"
				return m, nil
			}
			return m, m.save
		case "tab":
			if m.focus == 2 {
				m.editor.InsertString(strings.Repeat(" ", m.indentSpaces))
				m.dirty = m.isTainted()
				return m, nil
			}
			m.focusEditor((m.focus + 1) % 3)
			return m, nil
		case "shift+tab":
			m.focusEditor((m.focus + 2) % 3)
			return m, nil
		}
	}
	var cmds []tea.Cmd
	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.title, cmd = m.title.Update(msg)
	case 1:
		m.tags, cmd = m.tags.Update(msg)
	case 2:
		m.editor, cmd = m.editor.Update(msg)
	}
	cmds = append(cmds, cmd)
	m.dirty = m.isTainted()
	return m, tea.Batch(cmds...)
}

func (m Model) save() tea.Msg {
	tags := strings.Split(m.tags.Value(), ",")
	var note domain.Note
	var err error
	if m.editingID == "" {
		note, err = m.store.CreateNote(m.title.Value(), m.editor.Value(), tags)
	} else {
		note, err = m.store.SaveNote(m.editingID, m.title.Value(), m.editor.Value(), tags)
	}
	if err != nil {
		return errMsg{err}
	}
	return savedMsg{note}
}

func (m *Model) beginEdit(note domain.Note) {
	m.screen = editScreen
	m.editingID = note.ID
	m.title.SetValue(note.Title)
	m.tags.SetValue(strings.Join(note.Tags, ", "))
	m.editor.SetValue(note.Content)
	m.focus = 2
	m.title.Blur()
	m.tags.Blur()
	m.editor.Focus()
	m.savedTitle = note.Title
	m.savedTags = strings.Join(note.Tags, ", ")
	m.savedContent = note.Content
	m.savedNote = note
	m.resizeInputs()
	m.dirty = false
	m.status = ""
}

func (m *Model) beginView(note domain.Note) {
	m.screen = viewScreen
	m.viewNote = note
	m.confirmingExit = false
	m.viewer.SetContent(m.renderMarkdown(note.Content))
	m.resizeViewer()
	m.viewer.GotoTop()
}

func (m Model) renderMarkdown(content string) string {
	width := m.contentWidth() - 4
	if width < 20 {
		width = 20
	}
	renderer, err := glamour.NewTermRenderer(
		// Avoid Glamour's terminal background probe; it can block in an alternate-screen TUI.
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m Model) isTainted() bool {
	return m.title.Value() != m.savedTitle || m.tags.Value() != m.savedTags || m.editor.Value() != m.savedContent
}

func (m *Model) blurEditor() {
	m.title.Blur()
	m.tags.Blur()
	m.editor.Blur()
}

func (m *Model) leaveEditor() {
	if m.editingID != "" {
		m.beginView(m.savedNote)
		return
	}
	m.screen = listScreen
	m.blurEditor()
	m.editingID = ""
}

func (m *Model) focusEditor(field int) {
	m.title.Blur()
	m.tags.Blur()
	m.editor.Blur()
	m.focus = field
	switch field {
	case 0:
		m.title.Focus()
	case 1:
		m.tags.Focus()
	case 2:
		m.editor.Focus()
	}
}

func (m *Model) resizeInputs() {
	width := m.width - 4
	if width < 12 {
		width = 12
	}
	m.title.Width = width
	m.tags.Width = width
	m.editor.SetWidth(width)
	height := m.height - 8
	if height < 5 {
		height = 5
	}
	m.editor.SetHeight(height)
	m.resizeViewer()
}

func (m *Model) resizeViewer() {
	if m.viewer.Width < 1 {
		return
	}
	m.viewer.Width = m.contentWidth() - 2
	m.viewer.Height = m.contentHeight() - 2
	if m.screen == viewScreen {
		m.viewer.SetContent(m.renderMarkdown(m.viewNote.Content))
	}
}

func (m Model) View() string {
	if m.screen == editScreen {
		return m.editorView()
	}
	if m.screen == viewScreen {
		return m.viewView()
	}
	return m.listView()
}

func (m Model) listView() string {
	header := m.topBar("", fmt.Sprintf("%d notes", len(m.notes)))
	searchLine := ""
	if m.searching {
		searchLine = m.search.View()
	} else if m.search.Value() != "" {
		searchLine = m.muted("Search: " + m.search.Value())
	}
	var body strings.Builder
	if len(m.notes) == 0 {
		body.WriteString("No notes found. Press n to create one.\n")
	}
	for i, note := range m.notes {
		marker := "  "
		if i == m.cursor {
			marker = ">>"
		}
		line := fmt.Sprintf("%s %-28s %s", marker, truncate(note.Title, 28), note.UpdatedAt.Local().Format("2006-01-02 15:04"))
		if i == m.cursor {
			line = m.selected(line)
		}
		body.WriteString(line + "\n")
		if i == m.cursor {
			body.WriteString("   " + m.muted(truncate(strings.ReplaceAll(note.Content, "\n", " "), m.contentWidth()-8)) + "\n")
			if len(note.Tags) > 0 {
				body.WriteString("   " + m.tag("#"+strings.Join(note.Tags, " #")) + "\n")
			}
		}
	}
	content := body.String()
	if searchLine != "" {
		content = searchLine + "\n\n" + content
	}
	main := m.panel(content, m.contentHeight())
	footerText := "n new   enter view   e edit   / search   d delete   q quit"
	if m.status != "" {
		footerText = m.status + "   |   " + footerText
	}
	return header + "\n" + main + "\n" + m.footBar(footerText)
}

func (m Model) editorView() string {
	state := "SAVED"
	if m.dirty {
		state = "UNSAVED"
	}
	header := m.topBar(m.title.Value(), state)
	metadata := m.field("TITLE", m.title.View(), m.focus == 0) + "\n" + m.field("TAGS", m.tags.View(), m.focus == 1)
	body := m.panel(m.field("BODY", m.editor.View(), m.focus == 2), m.editorHeight(), m.focus == 2)
	content := metadata + "\n\n" + body
	footerText := "ctrl-s save   tab indent   shift-tab previous   esc view"
	if m.status != "" {
		footerText = m.status + "   |   " + footerText
	}
	if m.confirmingExit {
		footerText = "UNSAVED CHANGES   [s] Save   [d] Discard   [c] Cancel"
	}
	return header + "\n" + content + "\n" + m.footBar(footerText)
}

func (m Model) viewView() string {
	header := m.topBar(m.viewNote.Title, "SAVED")
	metadata := m.field("TAGS", m.tagList(m.viewNote.Tags), false)
	body := m.panel(m.viewer.View(), m.contentHeight(), false)
	footer := m.footBar("e edit   up/down scroll   esc back   q quit")
	return header + "\n" + metadata + "\n" + body + "\n" + footer
}

func (m Model) tagList(tags []string) string {
	if len(tags) == 0 {
		return m.muted("no tags")
	}
	return m.tag("#" + strings.Join(tags, " #"))
}

func (m Model) topBar(noteTitle, state string) string {
	width := m.viewWidth()
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
	style := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color(m.palette.Text)).Background(lipgloss.Color(m.palette.Surface)).Bold(true)
	if state == "UNSAVED" {
		style = style.Foreground(lipgloss.Color(m.palette.Primary))
	}
	return style.Render(line)
}

func (m Model) footBar(text string) string {
	text = truncate(text, m.viewWidth()-1)
	style := lipgloss.NewStyle().Width(m.viewWidth()).Foreground(lipgloss.Color(m.palette.Muted)).BorderTop(true).BorderForeground(lipgloss.Color(m.palette.Border))
	if strings.HasPrefix(text, "UNSAVED") {
		style = style.Foreground(lipgloss.Color(m.palette.Primary)).Bold(true)
	}
	return style.Render(" " + text)
}

func (m Model) field(label, value string, active bool) string {
	color := m.palette.Muted
	if active {
		color = m.palette.Primary
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(active).Render(label) + "\n" + value
}

func (m Model) panel(content string, height int, active ...bool) string {
	border := m.palette.Border
	if len(active) > 0 && active[0] {
		border = m.palette.Primary
	}
	style := lipgloss.NewStyle().Width(m.contentWidth()).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(border)).Padding(0, 1)
	if height > 0 {
		style = style.Height(height)
	}
	return style.Render(content)
}

func (m Model) selected(value string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Selected)).Bold(true).Render(value)
}

func (m Model) muted(value string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Render(value)
}

func (m Model) tag(value string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Secondary)).Render(value)
}

func (m Model) viewWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m Model) contentWidth() int {
	width := m.viewWidth() - 2
	if width < 16 {
		return 16
	}
	return width
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 12
	}
	if m.height-5 < 3 {
		return 3
	}
	return m.height - 5
}

func (m Model) editorHeight() int {
	if m.height <= 0 {
		return 5
	}
	height := m.height - 10
	if height < 5 {
		return 5
	}
	return height
}

func generatedTitle() string {
	adjectives := []string{"amber", "brisk", "calm", "clever", "cosmic", "gentle", "quiet", "tiny"}
	animals := []string{"badger", "cat", "fox", "otter", "panda", "rabbit", "shiba", "tiger"}
	return adjectives[rand.IntN(len(adjectives))] + "-" + animals[rand.IntN(len(animals))]
}

func parseFilter(value string) domain.NoteFilter {
	filter := domain.NoteFilter{}
	var content []string
	for _, token := range strings.Fields(value) {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 {
			content = append(content, token)
			continue
		}
		switch parts[0] {
		case "tag":
			filter.Tag = parts[1]
		case "after":
			if date, err := time.Parse("2006-01-02", parts[1]); err == nil {
				filter.From = &date
			}
		case "before":
			if date, err := time.Parse("2006-01-02", parts[1]); err == nil {
				filter.Through = &date
			}
		default:
			content = append(content, token)
		}
	}
	filter.Query = strings.Join(content, " ")
	return filter
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
