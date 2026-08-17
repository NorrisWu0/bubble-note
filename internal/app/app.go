package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/git"
	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/theme"
	"github.com/norriswu0/bubble-note/internal/view"
)

type screen int

const (
	listScreen screen = iota
	viewScreen
	formScreen
	createScreen
	moveScreen
	settingsScreen
)

// Store is the concrete note store surface the app needs beyond the notes
// repository: filesystem paths, refresh, and external-editor reload.
type Store interface {
	notes.Repository
	NotesDir() string
	Path(id string) (string, error)
	Refresh() error
	Reload(id string) (notes.Note, error)
	MoveNote(id, parent string) (notes.Note, error)
}

type Model struct {
	service   *notes.Service
	store     Store
	cfg       config.Config
	notes     []notes.Note
	cursor    int
	screen    screen
	search    textinput.Model
	title     textinput.Model
	tags      textinput.Model
	path      textinput.Model
	viewer    viewport.Model
	viewNote  notes.Note
	searching bool
	formID    string
	formNote  notes.Note
	status    string
	width     int
	height    int
	palette   theme.Palette
	editor    string
	gitClient string

	createFocus      int
	createError      string
	createPathHint   string
	createPathHintOK bool
	pendingView      string

	moveID     string
	moveError  string
	moveHint   string
	moveHintOK bool

	confirmingDelete bool
	deletingNoteID   string

	gitInfo git.Info

	settings       config.Config
	savedSettings  config.Config
	configPath     string
	settingsDirty  bool
	settingsInput  textinput.Model
	settingsFocus  int
	settingsActive bool
	settingsHint   string
	settingsHintOK bool
}

const (
	settingTheme = iota
	settingNotesDir
)

func New(store Store, cfg config.Config, configPath string, palettes ...theme.Palette) Model {
	search := textinput.New()
	search.Placeholder = "search notes"
	search.Prompt = "/ "
	title := textinput.New()
	title.Prompt = "Title: "
	tags := textinput.New()
	tags.Prompt = "Tags:  "
	path := textinput.New()
	path.Prompt = "Path:  "
	path.Placeholder = "e.g. journal/bubble-note"
	palette := theme.Default()
	if len(palettes) > 0 {
		palette = palettes[0]
	}
	viewer := viewport.New(80, 12)
	return Model{
		service:       notes.NewService(store),
		store:         store,
		cfg:           cfg,
		search:        search,
		title:         title,
		tags:          tags,
		path:          path,
		viewer:        viewer,
		palette:       palette,
		editor:        cfg.EditorCommand(),
		gitClient:     cfg.GitClientCommand(),
		settings:      cfg,
		savedSettings: cfg,
		configPath:    configPath,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load, m.loadGit)
}

type notesLoadedMsg []notes.Note

type gitInfoMsg struct {
	info git.Info
	err  error
}

type editorDoneMsg struct {
	noteID string
	err    error
}

type errMsg struct{ err error }

func (m Model) load() tea.Msg {
	notes, err := m.service.List(parseFilter(m.search.Value()))
	if err != nil {
		return errMsg{err}
	}
	return notesLoadedMsg(notes)
}

func (m Model) loadGit() tea.Msg {
	info, err := git.Status(m.store.NotesDir())
	return gitInfoMsg{info: info, err: err}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeInputs()
		return m, nil
	case notesLoadedMsg:
		m.notes = msg
		return m, nil
	case gitInfoMsg:
		if msg.err != nil {
			m.status = "git: " + msg.err.Error()
			return m, nil
		}
		m.gitInfo = msg.info
		return m, nil
	case editorDoneMsg:
		return m.handleEditorDone(msg)
	case errMsg:
		m.status = "Error: " + msg.err.Error()
		return m, nil
	}

	if m.confirmingDelete {
		return m.updateConfirmation(msg)
	}
	switch m.screen {
	case formScreen:
		return m.updateForm(msg)
	case createScreen:
		return m.updateCreate(msg)
	case moveScreen:
		return m.updateMove(msg)
	case viewScreen:
		return m.updateView(msg)
	case settingsScreen:
		return m.updateSettings(msg)
	}
	if m.searching {
		return m.updateSearch(msg)
	}
	return m.updateList(msg)
}

func (m Model) handleEditorDone(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.status = "Editor error: " + msg.err.Error()
		m.pendingView = ""
		return m, tea.Batch(m.load, m.loadGit)
	}
	note, err := m.store.Reload(msg.noteID)
	if err != nil {
		m.status = "Reload error: " + err.Error()
		m.pendingView = ""
		return m, tea.Batch(m.load, m.loadGit)
	}
	m.status = "Edited " + note.Title
	if msg.noteID == m.pendingView {
		m.pendingView = ""
		m.beginView(note)
	} else if m.screen == viewScreen && m.viewNote.ID == note.ID {
		m.viewNote = note
		m.viewer.SetContent(view.RenderMarkdown(view.Note{Title: note.Title, Content: note.Content}, m.contentWidth()-4, m.palette))
	}
	return m, tea.Batch(m.load, m.loadGit)
}

func (m Model) updateConfirmation(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "d", "enter":
		m.confirmingDelete = false
		return m, m.deleteNote()
	case "c", "esc":
		m.confirmingDelete = false
		m.deletingNoteID = ""
	}
	return m, nil
}

func (m *Model) deleteNote() tea.Cmd {
	id := m.deletingNoteID
	m.deletingNoteID = ""
	if err := m.service.Delete(id); err != nil {
		m.status = "Delete failed: " + err.Error()
		return nil
	}
	m.status = "Deleted note"
	return tea.Batch(m.load, m.loadGit)
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
	case "a":
		m.beginCreate()
	case "enter":
		if len(m.notes) > 0 {
			m.beginView(m.notes[m.cursor])
		}
	case "e":
		if len(m.notes) > 0 {
			return m, m.openEditor(m.notes[m.cursor])
		}
	case "t":
		if len(m.notes) > 0 {
			m.beginForm(m.notes[m.cursor])
		}
	case "m":
		if len(m.notes) > 0 {
			m.beginMove(m.notes[m.cursor])
		}
	case "/":
		m.searching = true
		m.search.Focus()
		return m, textinput.Blink
	case "esc":
		m.search.Reset()
		return m, m.load
	case "d":
		if len(m.notes) > 0 {
			m.confirmingDelete = true
			m.deletingNoteID = m.notes[m.cursor].ID
		}
	case "r":
		return m, m.refresh()
	case "g":
		return m, m.openGit()
	case "s":
		m.beginSettings()
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
			return m, m.openEditor(m.viewNote)
		case "t":
			m.beginForm(m.viewNote)
			return m, nil
		case "m":
			m.beginMove(m.viewNote)
			return m, nil
		case "d":
			m.confirmingDelete = true
			m.deletingNoteID = m.viewNote.ID
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

func (m *Model) beginForm(note notes.Note) {
	m.screen = formScreen
	m.formNote = note
	m.formID = note.ID
	m.title.Prompt = "Title: "
	m.title.Placeholder = ""
	m.title.SetValue(note.Title)
	m.tags.SetValue(strings.Join(note.Tags, ", "))
	m.focusField(0)
	m.status = ""
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.screen = listScreen
			return m, nil
		case "enter":
			return m, m.submitForm()
		case "tab":
			if m.title.Focused() {
				m.focusField(1)
			} else {
				m.focusField(0)
			}
			return m, nil
		case "shift+tab":
			if m.tags.Focused() {
				m.focusField(0)
			} else {
				m.focusField(1)
			}
			return m, nil
		}
	}
	var cmds []tea.Cmd
	var cmd tea.Cmd
	if m.title.Focused() {
		m.title, cmd = m.title.Update(msg)
	} else {
		m.tags, cmd = m.tags.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) submitForm() tea.Cmd {
	tags := strings.Split(m.tags.Value(), ",")
	note, err := m.service.Save(m.formID, m.title.Value(), m.formNote.Content, tags)
	if err != nil {
		m.status = "Save failed: " + err.Error()
		return nil
	}
	m.status = "Saved " + note.Title
	m.screen = listScreen
	return tea.Batch(m.load, m.loadGit)
}

func (m *Model) beginCreate() {
	m.screen = createScreen
	m.createFocus = 0
	m.createError = ""
	m.createPathHint = ""
	m.createPathHintOK = true
	m.path.SetValue("")
	m.title.SetValue("")
	m.tags.SetValue("")
	m.focusCreate(0)
}

func (m *Model) focusCreate(field int) {
	m.path.Blur()
	m.title.Blur()
	m.tags.Blur()
	m.createFocus = field
	switch field {
	case 0:
		m.path.Focus()
	case 1:
		m.title.Focus()
	case 2:
		m.tags.Focus()
	}
}

func (m Model) updateCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.screen = listScreen
			m.createError = ""
			return m, nil
		case "enter":
			return m, m.submitCreate()
		case "tab":
			m.focusCreate((m.createFocus + 1) % 3)
			return m, nil
		case "shift+tab":
			m.focusCreate((m.createFocus + 2) % 3)
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch m.createFocus {
	case 0:
		m.path, cmd = m.path.Update(msg)
	case 1:
		m.title, cmd = m.title.Update(msg)
	case 2:
		m.tags, cmd = m.tags.Update(msg)
	}
	m.refreshCreateValidation()
	return m, cmd
}

func (m *Model) refreshCreateValidation() {
	m.createError = m.createValidationError()
	parent := notes.NormalizePath(m.path.Value())
	if parent == "" {
		m.createPathHint = ""
		m.createPathHintOK = true
		return
	}
	m.createPathHint, m.createPathHintOK = notesDirState(filepath.Join(m.store.NotesDir(), parent))
}

func (m Model) createValidationError() string {
	if title := strings.TrimSpace(m.title.Value()); title != "" && !notes.ValidTitle(title) {
		return "name may only contain letters, numbers, spaces and hyphens"
	}
	for _, segment := range strings.Split(m.path.Value(), "/") {
		if segment = strings.TrimSpace(segment); segment != "" && !notes.ValidSegment(segment) {
			return "path may only contain letters, numbers and hyphens"
		}
	}
	for _, tag := range strings.Split(m.tags.Value(), ",") {
		if tag = strings.TrimSpace(tag); tag != "" && !notes.ValidSegment(tag) {
			return "tags may only contain letters, numbers and hyphens"
		}
	}
	return ""
}

func (m *Model) submitCreate() tea.Cmd {
	title := strings.TrimSpace(m.title.Value())
	if title == "" {
		m.createError = "title is required"
		return nil
	}
	if err := m.createValidationError(); err != "" {
		m.createError = err
		return nil
	}
	parent := notes.NormalizePath(m.path.Value())
	tags := strings.Split(m.tags.Value(), ",")
	note, err := m.service.Create(parent, notes.NormalizeTitle(title), "", tags)
	if err != nil {
		m.createError = "Create failed: " + err.Error()
		return nil
	}
	m.createError = ""
	if !commandExists(m.editor) {
		m.status = "Created " + note.Title + "; editor not found: " + m.editor
		m.beginView(note)
		return tea.Batch(m.load, m.loadGit)
	}
	m.pendingView = note.ID
	m.screen = listScreen
	m.status = "Created " + note.Title
	return tea.Batch(m.load, m.loadGit, m.runEditor(note.ID))
}

func (m *Model) focusField(field int) {
	m.title.Blur()
	m.tags.Blur()
	if field == 0 {
		m.title.Focus()
	} else {
		m.tags.Focus()
	}
}

func (m *Model) beginMove(note notes.Note) {
	m.screen = moveScreen
	m.moveID = note.ID
	m.moveError = ""
	m.moveHint = ""
	m.moveHintOK = true
	m.path.SetValue(note.Parent)
	m.path.Prompt = "Path:  "
	m.path.Placeholder = "e.g. journal/bubble-note (empty = root)"
	m.path.Focus()
	m.refreshMoveValidation()
}

func (m Model) updateMove(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.screen = listScreen
			m.moveError = ""
			return m, nil
		case "enter":
			return m, m.submitMove()
		}
	}
	var cmd tea.Cmd
	m.path, cmd = m.path.Update(msg)
	m.refreshMoveValidation()
	return m, cmd
}

func (m *Model) refreshMoveValidation() {
	path := m.path.Value()
	for _, segment := range strings.Split(path, "/") {
		if segment = strings.TrimSpace(segment); segment != "" && !notes.ValidSegment(segment) {
			m.moveError = "path may only contain letters, numbers and hyphens"
			m.moveHint = ""
			m.moveHintOK = true
			return
		}
	}
	m.moveError = ""
	parent := notes.NormalizePath(path)
	if parent == "" {
		m.moveHint = ""
		m.moveHintOK = true
		return
	}
	m.moveHint, m.moveHintOK = notesDirState(filepath.Join(m.store.NotesDir(), parent))
}

func (m *Model) submitMove() tea.Cmd {
	if m.moveError != "" {
		return nil
	}
	parent := notes.NormalizePath(m.path.Value())
	note, err := m.store.MoveNote(m.moveID, parent)
	if err != nil {
		m.moveError = "Move failed: " + err.Error()
		return nil
	}
	m.moveID = ""
	m.screen = listScreen
	m.status = "Moved " + note.Title
	return tea.Batch(m.load, m.loadGit)
}

func (m *Model) openEditor(note notes.Note) tea.Cmd {
	return m.runEditor(note.ID)
}

func (m *Model) runEditor(noteID string) tea.Cmd {
	if !commandExists(m.editor) {
		m.status = "Editor not found: " + m.editor + " (install it or set the 'editor' config)"
		return nil
	}
	path, err := m.store.Path(noteID)
	if err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	args := strings.Fields(m.editor)
	cmd := exec.Command(args[0], append(args[1:], path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{noteID: noteID, err: err}
	})
}

func (m *Model) openGit() tea.Cmd {
	if !commandExists(m.gitClient) {
		m.status = "git client not found: " + m.gitClient + " (install it or set the 'git_client' config)"
		return nil
	}
	if !m.gitInfo.IsRepo {
		if err := git.Init(m.store.NotesDir()); err != nil {
			m.status = "git init failed: " + err.Error()
			return nil
		}
		m.status = "Initialized git repository"
	}
	args := strings.Fields(m.gitClient)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = m.store.NotesDir()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err}
		}
		if err := m.store.Refresh(); err != nil {
			return errMsg{err}
		}
		return gitInfoMsg{info: mustGitStatus(m.store.NotesDir())}
	})
}

func mustGitStatus(dir string) git.Info {
	info, _ := git.Status(dir)
	return info
}

func (m *Model) refresh() tea.Cmd {
	if err := m.store.Refresh(); err != nil {
		m.status = "Refresh failed: " + err.Error()
		return nil
	}
	m.status = "Refreshed from files"
	return tea.Batch(m.load, m.loadGit)
}

func (m *Model) beginView(note notes.Note) {
	m.screen = viewScreen
	m.viewNote = note
	m.viewer.SetContent(view.RenderMarkdown(view.Note{Title: note.Title, Content: note.Content}, m.contentWidth()-4, m.palette))
	m.resizeViewer()
	m.viewer.GotoTop()
}

func (m *Model) beginSettings() {
	m.screen = settingsScreen
	m.settings = m.savedSettings
	m.settingsDirty = false
	m.settingsFocus = settingTheme
	m.settingsActive = false
	m.settingsInput = textinput.New()
	m.settingsInput.Blur()
	m.settingsHint = ""
	m.settingsHintOK = true
}

var catppuccinFlavors = []string{"latte", "frappe", "macchiato", "mocha"}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		if m.settingsDirty {
			m.settings = m.savedSettings
			m.restoreSavedTheme()
			m.settingsDirty = false
		}
		m.screen = listScreen
		return m, nil
	case "ctrl+s":
		m.commitSettingsInput()
		if !m.settingsDirty {
			return m, nil
		}
		m.saveSettings()
		return m, nil
	case "up", "k":
		m.commitSettingsInput()
		m.settingsFocus = previousSettingFocus(m.settingsFocus)
		m.settingsActive = false
		return m, nil
	case "down", "j":
		m.commitSettingsInput()
		m.settingsFocus = nextSettingFocus(m.settingsFocus)
		m.settingsActive = false
		return m, nil
	case "enter":
		if m.settingsFocus == settingTheme {
			m.settings.Theme.Flavor = nextCatppuccinFlavor(m.settings.Theme.Flavor)
			m.applySelectedTheme()
			m.settingsDirty = true
			return m, nil
		}
		m.beginSettingsInput()
		return m, nil
	case "left", "right":
		if m.settingsFocus == settingTheme {
			if key.String() == "right" {
				m.settings.Theme.Flavor = nextCatppuccinFlavor(m.settings.Theme.Flavor)
			} else {
				m.settings.Theme.Flavor = previousCatppuccinFlavor(m.settings.Theme.Flavor)
			}
			m.applySelectedTheme()
			m.settingsDirty = true
			return m, nil
		}
		if m.settingsActive {
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			m.settingsDirty = m.settingsDirty || m.settingsInput.Value() != m.savedSettings.NotesDir
			m.refreshNotesDirHint()
			return m, cmd
		}
		return m, nil
	}
	if m.settingsActive {
		var cmd tea.Cmd
		m.settingsInput, cmd = m.settingsInput.Update(msg)
		m.settingsDirty = m.settingsDirty || m.settingsInput.Value() != m.savedSettings.NotesDir
		m.refreshNotesDirHint()
		return m, cmd
	}
	return m, nil
}

func (m *Model) beginSettingsInput() {
	m.settingsInput = textinput.New()
	m.settingsInput.SetValue(m.effectiveNotesDir())
	m.settingsInput.Width = m.contentWidth() - 30
	if m.settingsInput.Width < 12 {
		m.settingsInput.Width = 12
	}
	m.settingsInput.Focus()
	m.settingsActive = true
	m.refreshNotesDirHint()
}

func (m *Model) refreshNotesDirHint() {
	m.settingsHint, m.settingsHintOK = notesDirState(m.settingsInput.Value())
}

func notesDirState(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "empty — uses the default directory", true
	}
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return "not a directory", false
		}
		return "valid directory", true
	}
	if errors.Is(err, os.ErrNotExist) {
		return "does not exist — will be created", true
	}
	return "cannot access: " + err.Error(), false
}

func (m *Model) commitSettingsInput() {
	if !m.settingsActive {
		return
	}
	value := strings.TrimSpace(m.settingsInput.Value())
	if value != m.settings.NotesDir {
		m.settings.NotesDir = value
		m.settingsDirty = true
	}
	m.settingsActive = false
	m.settingsInput.Blur()
}

func (m *Model) saveSettings() {
	palette, err := theme.Resolve(m.settings.Theme)
	if err != nil {
		m.status = "Theme error: " + err.Error()
		return
	}
	if err := config.Save(m.configPath, m.settings); err != nil {
		m.status = "Save settings failed: " + err.Error()
		return
	}
	m.savedSettings = m.settings
	m.palette = palette
	m.settingsDirty = false
	m.status = "Saved settings; notes_dir applies on next launch"
}

func (m Model) effectiveNotesDir() string {
	dir, err := m.settings.NotesDirectory()
	if err != nil {
		return m.settings.NotesDir
	}
	return dir
}

func nextSettingFocus(focus int) int {
	if focus == settingTheme {
		return settingNotesDir
	}
	return settingTheme
}

func previousSettingFocus(focus int) int { return nextSettingFocus(focus) }

func (m *Model) applySelectedTheme() {
	palette, err := theme.Resolve(m.settings.Theme)
	if err != nil {
		m.status = "Error: " + err.Error()
		return
	}
	m.palette = palette
}

func (m *Model) restoreSavedTheme() {
	palette, err := theme.Resolve(m.savedSettings.Theme)
	if err == nil {
		m.palette = palette
	}
}

func (m Model) View() string {
	var content string
	switch m.screen {
	case formScreen:
		content = view.RenderForm(view.FormModel{
			Width:       m.width,
			Height:      m.height,
			Heading:     "EDIT NOTE",
			TitleView:   m.title.View(),
			TagsView:    m.tags.View(),
			TitleActive: m.title.Focused(),
			TagsActive:  m.tags.Focused(),
			Status:      m.status,
		}, m.palette)
	case viewScreen:
		content = view.RenderReader(view.ReaderModel{Width: m.width, Height: m.height, Title: m.viewNote.Title, Tags: m.viewNote.Tags, Body: m.viewer.View()}, m.palette)
	case settingsScreen:
		content = view.RenderSettings(view.SettingsModel{Width: m.width, Height: m.height, Rows: m.settingRows(), Status: m.status, Dirty: m.settingsDirty, Input: m.settingsInput.View(), InputActive: m.settingsActive, Hint: m.settingsHint, HintOK: m.settingsHintOK}, m.palette)
	default:
		rows := make([]view.NoteRow, len(m.notes))
		for i, note := range m.notes {
			rows[i] = view.NoteRow{Title: note.Title, Path: note.Parent, Updated: note.UpdatedAt.Local().Format("2006-01-02 15:04"), Tags: note.Tags, Selected: i == m.cursor}
		}
		searchInput := ""
		if m.searching {
			searchInput = m.search.View()
		}
		content = view.RenderList(view.ListModel{
			Width:       m.width,
			Height:      m.height,
			Count:       len(m.notes),
			Search:      m.search.Value(),
			SearchInput: searchInput,
			Rows:        rows,
			Status:      m.status,
			GitStatus:   gitStatusLabel(m.gitInfo),
		}, m.palette)
	}
	if m.screen == createScreen {
		return view.RenderCreateNoteModal(view.CreateNoteModel{
			Width:       m.width,
			Height:      m.height,
			PathView:    m.path.View(),
			TitleView:   m.title.View(),
			TagsView:    m.tags.View(),
			PathActive:  m.createFocus == 0,
			TitleActive: m.createFocus == 1,
			TagsActive:  m.createFocus == 2,
			Error:       m.createError,
		}, m.palette)
	}
	if m.screen == moveScreen {
		return view.RenderMoveNoteModal(view.MoveNoteModel{
			Width:      m.width,
			Height:     m.height,
			PathView:   m.path.View(),
			PathActive: true,
			Error:      m.moveError,
			Hint:       m.moveHint,
			HintOK:     m.moveHintOK,
		}, m.palette)
	}
	if m.confirmingDelete {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Delete note?", Message: "The note's files will be removed from the notes directory.", Actions: "[d] Delete   [c] Cancel"}, m.width, m.height, m.palette)
	}
	return content
}

func gitStatusLabel(info git.Info) string {
	if !info.IsRepo {
		return "git: not initialized"
	}
	if info.Dirty {
		return "git: dirty"
	}
	return "git: up-to-date"
}

func (m Model) settingRows() []view.SettingRow {
	return []view.SettingRow{
		{Section: "GENERAL", Label: "Theme", Value: "< " + m.settings.Theme.Flavor + " >", Selected: m.settingsFocus == settingTheme},
		{Section: "GENERAL", Label: "Notes dir", Value: m.effectiveNotesDir(), Selected: m.settingsFocus == settingNotesDir},
	}
}

func (m *Model) resizeInputs() {
	width := m.width - 4
	if width < 12 {
		width = 12
	}
	m.title.Width = width
	m.tags.Width = width
	m.resizeViewer()
}

func (m *Model) resizeViewer() {
	if m.viewer.Width < 1 {
		return
	}
	m.viewer.Width = m.contentWidth() - 2
	m.viewer.Height = m.contentHeight() - 2
	if m.screen == viewScreen {
		m.viewer.SetContent(view.RenderMarkdown(view.Note{Title: m.viewNote.Title, Content: m.viewNote.Content}, m.contentWidth()-4, m.palette))
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

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func nextCatppuccinFlavor(current string) string {
	for i, flavor := range catppuccinFlavors {
		if current == flavor {
			return catppuccinFlavors[(i+1)%len(catppuccinFlavors)]
		}
	}
	return catppuccinFlavors[0]
}

func previousCatppuccinFlavor(current string) string {
	for i, flavor := range catppuccinFlavors {
		if current == flavor {
			return catppuccinFlavors[(i+len(catppuccinFlavors)-1)%len(catppuccinFlavors)]
		}
	}
	return catppuccinFlavors[0]
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func generatedTitle() string { return notes.GeneratedTitle() }

func parseFilter(value string) notes.Filter { return notes.ParseFilter(value) }
