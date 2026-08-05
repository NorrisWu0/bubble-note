package app

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/domain"
)

type screen int

const (
	listScreen screen = iota
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
	searching      bool
	editingID      string
	dirty          bool
	status         string
	focus          int
	width          int
	height         int
	confirmingExit bool
	saveThenExit   bool
	savedTitle     string
	savedTags      string
	savedContent   string
}

func New(store domain.NoteStore) Model {
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
	return Model{store: store, search: search, title: title, tags: tags, editor: editor}
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
	case "enter", "e":
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
	m.title.Focus()
	m.focus = 0
	m.savedTitle = note.Title
	m.savedTags = strings.Join(note.Tags, ", ")
	m.savedContent = note.Content
	m.resizeInputs()
	m.dirty = false
	m.status = ""
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
	if width < 20 {
		width = 20
	}
	m.title.Width = width
	m.tags.Width = width
	m.editor.SetWidth(width)
	height := m.height - 8
	if height < 5 {
		height = 5
	}
	m.editor.SetHeight(height)
}

func (m Model) View() string {
	if m.screen == editScreen {
		return m.editorView()
	}
	return m.listView()
}

func (m Model) listView() string {
	header := titleStyle.Render("bubble-note") + "  " + mutedStyle.Render("local Markdown notes")
	if m.searching {
		header += "\n" + m.search.View()
	} else if m.search.Value() != "" {
		header += "\n" + mutedStyle.Render("Search: "+m.search.Value())
	}
	var body strings.Builder
	if len(m.notes) == 0 {
		body.WriteString("\n  No notes found. Press n to create one.\n")
	}
	for i, note := range m.notes {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		line := fmt.Sprintf("%s%-28s %s", marker, note.Title, note.UpdatedAt.Local().Format("2006-01-02 15:04"))
		if i == m.cursor {
			line = selectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
		if i == m.cursor {
			body.WriteString("    " + truncate(strings.ReplaceAll(note.Content, "\n", " "), 80) + "\n")
			if len(note.Tags) > 0 {
				body.WriteString("    " + mutedStyle.Render("#"+strings.Join(note.Tags, " #")) + "\n")
			}
		}
	}
	footer := mutedStyle.Render("n new  e/enter edit  / search  d delete  q quit")
	if m.status != "" {
		footer += "\n" + m.status
	}
	return header + "\n\n" + body.String() + "\n" + footer + "\n"
}

func (m Model) editorView() string {
	mode := "new note"
	if m.editingID != "" {
		mode = "editing " + m.editingID[:8]
	}
	state := "saved"
	if m.dirty {
		state = "unsaved"
	}
	footer := mutedStyle.Render("ctrl+s save  tab next field  esc back")
	if m.status != "" {
		footer += "\n" + m.status
	}
	if m.confirmingExit {
		footer += "\n" + titleStyle.Render("Unsaved changes: [s] Save  [d] Discard  [c] Cancel")
	}
	return titleStyle.Render(mode) + "  " + mutedStyle.Render(state) + "\n\n" + m.title.View() + "\n" + m.tags.View() + "\n\n" + m.editor.View() + "\n" + footer + "\n"
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
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-3]) + "..."
}

var titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5A65B"))
var selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5A65B"))
var mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
