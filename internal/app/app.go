package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/theme"
	"github.com/norriswu0/bubble-note/internal/view"
)

type screen int

const (
	listScreen screen = iota
	viewScreen
	editScreen
)

type Model struct {
	service        *notes.Service
	notes          []notes.Note
	cursor         int
	screen         screen
	search         textinput.Model
	title          textinput.Model
	tags           textinput.Model
	editor         textarea.Model
	viewer         viewport.Model
	viewNote       notes.Note
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
	savedNote      notes.Note
}

func New(repository notes.Repository, palettes ...theme.Palette) Model {
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
	return Model{service: notes.NewService(repository), search: search, title: title, tags: tags, editor: editor, viewer: viewer, palette: palette, indentSpaces: config.DefaultIndentSpaces}
}

func NewWithSettings(store notes.Repository, palette theme.Palette, indentSpaces int) Model {
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
	notes, err := m.service.List(parseFilter(m.search.Value()))
	if err != nil {
		return errMsg{err}
	}
	return notesLoadedMsg(notes)
}

type notesLoadedMsg []notes.Note

type savedMsg struct {
	note notes.Note
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
		m.beginEdit(notes.Note{Title: generatedTitle(), Content: ""})
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
			if err := m.service.Delete(m.notes[m.cursor].ID); err != nil {
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
	var note notes.Note
	var err error
	if m.editingID == "" {
		note, err = m.service.Create(m.title.Value(), m.editor.Value(), tags)
	} else {
		note, err = m.service.Save(m.editingID, m.title.Value(), m.editor.Value(), tags)
	}
	if err != nil {
		return errMsg{err}
	}
	return savedMsg{note}
}

func (m *Model) beginEdit(note notes.Note) {
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

func (m *Model) beginView(note notes.Note) {
	m.screen = viewScreen
	m.viewNote = note
	m.confirmingExit = false
	m.viewer.SetContent(view.RenderMarkdown(view.Note{Title: note.Title, Content: note.Content}, m.contentWidth()-4))
	m.resizeViewer()
	m.viewer.GotoTop()
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
		m.viewer.SetContent(view.RenderMarkdown(view.Note{Title: m.viewNote.Title, Content: m.viewNote.Content}, m.contentWidth()-4))
	}
}

func (m Model) contentWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	width -= 2
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

func (m Model) View() string {
	switch m.screen {
	case editScreen:
		return view.RenderEditor(view.EditorModel{
			Width: m.width, Height: m.height, Title: m.title.Value(), TitleView: m.title.View(), TagsView: m.tags.View(), BodyView: m.editor.View(),
			TitleActive: m.focus == 0, TagsActive: m.focus == 1, BodyActive: m.focus == 2,
			State: func() string {
				if m.dirty {
					return "UNSAVED"
				}
				return "SAVED"
			}(), Status: m.status, ConfirmingExit: m.confirmingExit,
		}, m.palette)
	case viewScreen:
		return view.RenderReader(view.ReaderModel{Width: m.width, Height: m.height, Title: m.viewNote.Title, Tags: m.viewNote.Tags, Body: m.viewer.View()}, m.palette)
	default:
		rows := make([]view.NoteRow, len(m.notes))
		for i, note := range m.notes {
			rows[i] = view.NoteRow{Title: note.Title, Updated: note.UpdatedAt.Local().Format("2006-01-02 15:04"), Excerpt: note.Content, Tags: note.Tags, Selected: i == m.cursor}
		}
		searchInput := ""
		if m.searching {
			searchInput = m.search.View()
		}
		return view.RenderList(view.ListModel{Width: m.width, Height: m.height, Count: len(m.notes), Search: m.search.Value(), SearchInput: searchInput, Rows: rows, Status: m.status}, m.palette)
	}
}

func generatedTitle() string { return notes.GeneratedTitle() }

func parseFilter(value string) notes.Filter { return notes.ParseFilter(value) }
