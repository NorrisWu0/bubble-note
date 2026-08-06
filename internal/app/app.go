package app

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/storage"
	"github.com/norriswu0/bubble-note/internal/theme"
	"github.com/norriswu0/bubble-note/internal/view"
)

type screen int

const (
	listScreen screen = iota
	viewScreen
	editScreen
	settingsScreen
)

type Model struct {
	service                *notes.Service
	notes                  []notes.Note
	cursor                 int
	screen                 screen
	search                 textinput.Model
	title                  textinput.Model
	tags                   textinput.Model
	editor                 textarea.Model
	viewer                 viewport.Model
	viewNote               notes.Note
	searching              bool
	editingID              string
	dirty                  bool
	status                 string
	focus                  int
	width                  int
	height                 int
	palette                theme.Palette
	indentSpaces           int
	confirmingExit         bool
	confirmingClear        bool
	confirmingSettingsExit bool
	confirmingDelete       bool
	deletingNoteID         string
	saveThenExit           bool
	savedTitle             string
	savedTags              string
	savedContent           string
	savedNote              notes.Note
	settings               config.Config
	savedSettings          config.Config
	configPath             string
	settingsInput          textinput.Model
	settingsFocus          int
	settingsAdvanced       bool
	settingsInputActive    bool
	settingsStatus         string
	settingsChecker        storage.ConnectionChecker
	noteSyncer             storage.NoteSyncer
	syncStatusWriter       interface {
		SetSyncStatus(string, notes.SyncStatus, string) error
		SyncETag(string) (string, error)
	}
	syncRepository interface {
		ApplyRemoteSnapshot(string, notes.SyncSnapshot) (notes.Note, error)
		CopyRemoteSnapshot(notes.SyncSnapshot) (notes.Note, error)
	}
	atomicDeleter interface {
		DeleteNoteAtomic(string, func() error) error
	}
	conflictLoading      bool
	conflictActive       bool
	conflictNoteID       string
	conflictSnapshot     notes.SyncSnapshot
	conflictETag         string
	storageState         storage.State
	storageSpinner       spinner.Model
	s3CheckID            uint64
	settingsFocused      bool
	setRevisionRetention func(int) error
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
	return Model{service: notes.NewService(repository), search: search, title: title, tags: tags, editor: editor, viewer: viewer, palette: palette, indentSpaces: config.DefaultIndentSpaces, storageSpinner: spinner.New()}
}

func NewWithSettings(store notes.Repository, palette theme.Palette, indentSpaces int) Model {
	model := New(store, palette)
	if indentSpaces > 0 {
		model.indentSpaces = indentSpaces
	}
	return model
}

func NewWithConfig(store notes.Repository, palette theme.Palette, cfg config.Config, configPath string, checker storage.ConnectionChecker, setRevisionRetention func(int) error, syncers ...storage.NoteSyncer) Model {
	model := NewWithSettings(store, palette, cfg.IndentSpaces)
	model.settings = cfg
	model.savedSettings = cfg
	model.configPath = configPath
	model.settingsChecker = checker
	if len(syncers) > 0 {
		model.noteSyncer = syncers[0]
	}
	if writer, ok := store.(interface {
		SetSyncStatus(string, notes.SyncStatus, string) error
		SyncETag(string) (string, error)
	}); ok {
		model.syncStatusWriter = writer
	}
	if repository, ok := store.(interface {
		ApplyRemoteSnapshot(string, notes.SyncSnapshot) (notes.Note, error)
		CopyRemoteSnapshot(notes.SyncSnapshot) (notes.Note, error)
	}); ok {
		model.syncRepository = repository
	}
	if deleter, ok := store.(interface {
		DeleteNoteAtomic(string, func() error) error
	}); ok {
		model.atomicDeleter = deleter
	}
	if checker != nil {
		model.storageState = storage.Checking
		model.s3CheckID = 1
	}
	model.setRevisionRetention = setRevisionRetention
	return model
}

func (m Model) Init() tea.Cmd {
	if m.settingsChecker != nil {
		return tea.Batch(m.load, m.checkS3, m.storageSpinner.Tick)
	}
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
	if _, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		m.storageSpinner, cmd = m.storageSpinner.Update(msg)
		return m, cmd
	}
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
		cmds := []tea.Cmd{m.load}
		if m.noteSyncer != nil && m.storageState == storage.Connected {
			cmds = append(cmds, m.syncNote(msg.note))
		}
		return m, tea.Batch(cmds...)
	case noteSyncResultMsg:
		status := notes.SyncLocalOnly
		if msg.err == nil {
			status = notes.SyncSynced
		} else if errors.Is(msg.err, storage.ErrConflict) {
			status = notes.SyncConflicted
		}
		if m.syncStatusWriter != nil {
			if err := m.syncStatusWriter.SetSyncStatus(msg.noteID, status, msg.etag); err != nil {
				m.status = "Sync status error: " + err.Error()
			}
		}
		m.savedNote.SyncStatus = status
		if m.viewNote.ID == msg.noteID {
			m.viewNote.SyncStatus = status
		}
		if msg.err != nil {
			m.status = "Saved locally; sync failed: " + msg.err.Error()
			return m, m.load
		}
		m.status = "Saved and synced"
		return m, m.load
	case conflictLoadedMsg:
		m.conflictLoading = false
		if msg.err != nil {
			m.status = "Conflict unavailable: " + msg.err.Error()
			return m, nil
		}
		m.conflictSnapshot = msg.snapshot
		m.conflictETag = msg.etag
		m.conflictActive = true
	case conflictResolvedMsg:
		if msg.err != nil {
			m.status = "Conflict resolution failed: " + msg.err.Error()
			return m, nil
		}
		m.conflictActive = false
		m.conflictLoading = false
		m.viewNote = msg.note
		m.savedNote.SyncStatus = msg.status
		if m.syncStatusWriter != nil {
			_ = m.syncStatusWriter.SetSyncStatus(msg.note.ID, msg.status, msg.etag)
		}
		m.status = "Conflict resolved"
		return m, m.load
	case deleteResultMsg:
		m.deletingNoteID = ""
		if msg.err != nil {
			m.status = "Delete failed; nothing changed: " + msg.err.Error()
			return m, nil
		}
		m.status = "Deleted locally and from S3"
		return m, m.load
	case errMsg:
		m.status = "Error: " + msg.err.Error()
		m.saveThenExit = false
		m.confirmingExit = false
	}

	if m.conflictLoading || m.conflictActive {
		return m.updateConflict(msg)
	}
	if m.confirmingExit || m.confirmingClear || m.confirmingSettingsExit || m.confirmingDelete {
		return m.updateConfirmation(msg)
	}
	if m.screen == editScreen {
		return m.updateEditor(msg)
	}
	if m.screen == viewScreen {
		return m.updateView(msg)
	}
	if m.screen == settingsScreen {
		return m.updateSettings(msg)
	}
	if m.searching {
		return m.updateSearch(msg)
	}
	return m.updateList(msg)
}

type noteSyncResultMsg struct {
	noteID string
	etag   string
	err    error
}

type conflictLoadedMsg struct {
	noteID   string
	snapshot notes.SyncSnapshot
	etag     string
	err      error
}

type conflictResolvedMsg struct {
	note   notes.Note
	status notes.SyncStatus
	etag   string
	err    error
}

func (m Model) syncNote(note notes.Note) tea.Cmd {
	return func() tea.Msg {
		revisions, err := m.service.Revisions(note.ID)
		if err != nil {
			return noteSyncResultMsg{noteID: note.ID, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		expectedETag := ""
		if m.syncStatusWriter != nil {
			expectedETag, err = m.syncStatusWriter.SyncETag(note.ID)
			if err != nil {
				return noteSyncResultMsg{noteID: note.ID, err: err}
			}
		}
		etag, err := m.noteSyncer.SyncNote(ctx, note, revisions, expectedETag)
		return noteSyncResultMsg{noteID: note.ID, etag: etag, err: err}
	}
}

type configSavedMsg struct {
	cfg     config.Config
	palette theme.Palette
}

type s3CheckedMsg struct {
	id  uint64
	err error
}

func (m Model) updateConfirmation(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.confirmingClear {
		switch key.String() {
		case "c", "enter":
			m.clearS3()
			m.confirmingClear = false
		case "esc":
			m.confirmingClear = false
		}
		return m, nil
	}
	if m.confirmingSettingsExit {
		switch key.String() {
		case "s":
			m.confirmingSettingsExit = false
			m.commitSettingsInput()
			return m, m.saveSettings
		case "d":
			m.settings = m.savedSettings
			m.restoreSavedTheme()
			m.confirmingSettingsExit = false
			m.screen = listScreen
		case "c", "esc":
			m.confirmingSettingsExit = false
		}
		return m, nil
	}
	if m.confirmingDelete {
		switch key.String() {
		case "d", "enter":
			m.confirmingDelete = false
			return m, m.deleteNote(m.deletingNoteID)
		case "c", "esc":
			m.confirmingDelete = false
			m.deletingNoteID = ""
		}
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

type deleteResultMsg struct{ err error }

func (m Model) deleteNote(noteID string) tea.Cmd {
	return func() tea.Msg {
		deleteRemote := func() error {
			if m.noteSyncer == nil {
				return nil
			}
			if m.storageState != storage.Connected {
				return errors.New("storage is not connected; note was not deleted")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return m.noteSyncer.DeleteNote(ctx, noteID)
		}
		if m.atomicDeleter != nil {
			return deleteResultMsg{err: m.atomicDeleter.DeleteNoteAtomic(noteID, deleteRemote)}
		}
		if m.noteSyncer != nil {
			return deleteResultMsg{err: errors.New("local store does not support atomic deletion")}
		}
		return deleteResultMsg{err: m.service.Delete(noteID)}
	}
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
		if m.settingsFocused {
			return m, nil
		}
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.settingsFocused {
			return m, nil
		}
		if m.cursor < len(m.notes)-1 {
			m.cursor++
		}
	case "n":
		if m.settingsFocused {
			return m, nil
		}
		m.beginEdit(notes.Note{Title: generatedTitle(), Content: ""})
	case "enter":
		if m.settingsFocused {
			m.beginSettings()
			return m, nil
		}
		if len(m.notes) > 0 {
			return m, m.openNote(m.notes[m.cursor])
		}
	case "e":
		if m.settingsFocused {
			return m, nil
		}
		if len(m.notes) > 0 {
			return m, m.openNoteForEdit(m.notes[m.cursor])
		}
	case "d":
		if m.settingsFocused {
			return m, nil
		}
		if len(m.notes) > 0 {
			m.confirmingDelete = true
			m.deletingNoteID = m.notes[m.cursor].ID
		}
	case "/":
		if m.settingsFocused {
			return m, nil
		}
		m.searching = true
		m.search.Focus()
		return m, textinput.Blink
	case "esc":
		m.settingsFocused = false
		m.search.Reset()
		return m, m.load
	case "tab":
		m.settingsFocused = !m.settingsFocused
	}
	return m, nil
}

func (m *Model) openNote(note notes.Note) tea.Cmd {
	m.beginView(note)
	if note.SyncStatus == notes.SyncConflicted && m.noteSyncer != nil {
		m.conflictNoteID = note.ID
		m.conflictLoading = true
		return m.loadConflict(note.ID)
	}
	return nil
}

func (m *Model) openNoteForEdit(note notes.Note) tea.Cmd {
	if note.SyncStatus == notes.SyncConflicted && m.noteSyncer != nil {
		m.beginView(note)
		m.conflictNoteID = note.ID
		m.conflictLoading = true
		return m.loadConflict(note.ID)
	}
	m.beginEdit(note)
	return nil
}

func (m Model) loadConflict(noteID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		snapshot, etag, err := m.noteSyncer.PullNote(ctx, noteID)
		return conflictLoadedMsg{noteID: noteID, snapshot: snapshot, etag: etag, err: err}
	}
}

func (m Model) updateConflict(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.conflictLoading {
		return m, nil
	}
	switch key.String() {
	case "l":
		return m, m.resolveConflictLocal()
	case "r":
		return m, m.resolveConflictRemote()
	case "c":
		return m, m.resolveConflictCopy()
	}
	return m, nil
}

func (m Model) resolveConflictLocal() tea.Cmd {
	return func() tea.Msg {
		revisions, err := m.service.Revisions(m.conflictNoteID)
		if err != nil {
			return conflictResolvedMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		etag, err := m.noteSyncer.SyncNote(ctx, m.viewNote, revisions, "")
		if err != nil {
			return conflictResolvedMsg{err: err}
		}
		return conflictResolvedMsg{note: m.viewNote, status: notes.SyncSynced, etag: etag}
	}
}

func (m Model) resolveConflictRemote() tea.Cmd {
	return func() tea.Msg {
		if m.syncRepository == nil {
			return conflictResolvedMsg{err: errors.New("local store cannot apply remote notes")}
		}
		note, err := m.syncRepository.ApplyRemoteSnapshot(m.conflictNoteID, m.conflictSnapshot)
		return conflictResolvedMsg{note: note, status: notes.SyncSynced, etag: m.conflictETag, err: err}
	}
}

func (m Model) resolveConflictCopy() tea.Cmd {
	return func() tea.Msg {
		if m.syncRepository == nil {
			return conflictResolvedMsg{err: errors.New("local store cannot copy remote notes")}
		}
		if _, err := m.syncRepository.CopyRemoteSnapshot(m.conflictSnapshot); err != nil {
			return conflictResolvedMsg{err: err}
		}
		revisions, err := m.service.Revisions(m.conflictNoteID)
		if err != nil {
			return conflictResolvedMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		etag, err := m.noteSyncer.SyncNote(ctx, m.viewNote, revisions, "")
		if err != nil {
			return conflictResolvedMsg{err: err}
		}
		return conflictResolvedMsg{note: m.viewNote, status: notes.SyncSynced, etag: etag}
	}
}

const (
	settingTheme = iota
	settingRevision
	settingRegion
	settingBucket
	settingAccessKey
	settingSecretKey
	settingStatus
	settingRefresh
	settingAdvanced
	settingEndpoint
	settingPrefix
	settingPathStyle
	settingSessionToken
	settingClear
)

var catppuccinFlavors = []string{"latte", "frappe", "macchiato", "mocha"}

type settingDescriptor struct {
	key       int
	section   string
	label     string
	advanced  bool
	focusable bool
	action    bool
}

var settingDescriptors = []settingDescriptor{
	{key: settingTheme, section: "GENERAL", label: "Theme", focusable: true},
	{key: settingRevision, section: "NOTES", label: "Revision limit", focusable: true},
	{key: settingRegion, section: "STORAGE", label: "Region", focusable: true},
	{key: settingBucket, section: "STORAGE", label: "Bucket", focusable: true},
	{key: settingAccessKey, section: "STORAGE", label: "Access key ID", focusable: true},
	{key: settingSecretKey, section: "STORAGE", label: "Secret access key", focusable: true},
	{key: settingStatus, section: "STORAGE", label: "Status"},
	{key: settingRefresh, section: "STORAGE", label: "Refresh connection", focusable: true, action: true},
	{key: settingAdvanced, section: "STORAGE", label: "Advanced storage options", focusable: true, action: true},
	{key: settingEndpoint, section: "STORAGE", label: "Endpoint", advanced: true, focusable: true},
	{key: settingPrefix, section: "STORAGE", label: "Prefix", advanced: true, focusable: true},
	{key: settingPathStyle, section: "STORAGE", label: "Path style", advanced: true, focusable: true},
	{key: settingSessionToken, section: "STORAGE", label: "Session token", advanced: true, focusable: true},
	{key: settingClear, section: "STORAGE", label: "Clear S3 configuration", focusable: true, action: true},
}

func (m *Model) beginSettings() {
	m.screen = settingsScreen
	m.settingsFocused = false
	m.settingsFocus = settingTheme
	m.settingsInputActive = false
	m.settingsStatus = ""
	m.loadSettingsInput()
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if result, ok := msg.(configSavedMsg); ok {
		if m.setRevisionRetention != nil {
			if err := m.setRevisionRetention(result.cfg.RevisionRetention); err != nil {
				m.settingsStatus = "Error: " + err.Error()
				return m, nil
			}
		}
		m.settings = result.cfg
		m.savedSettings = result.cfg
		m.palette = result.palette
		m.indentSpaces = result.cfg.IndentSpaces
		m.settingsStatus = "Saved settings"
		m.settingsInputActive = false
		if checker, err := storage.NewS3(result.cfg.Storage); err == nil {
			m.settingsChecker = checker
			m.storageState = storage.Checking
			m.s3CheckID++
			m.settingsStatus = "Checking storage..."
			return m, tea.Batch(m.checkS3, m.storageSpinner.Tick)
		}
		m.settingsChecker = nil
		m.storageState = storage.NotConfigured
		return m, nil
	}
	if result, ok := msg.(s3CheckedMsg); ok {
		if result.id != m.s3CheckID {
			return m, nil
		}
		if result.err == nil {
			m.storageState = storage.Connected
			m.settingsStatus = "S3 connected"
		} else {
			m.storageState = storage.Unavailable
			if isUnauthorized(result.err) {
				m.storageState = storage.Unauthorized
			}
			m.settingsStatus = "S3 error: " + result.err.Error()
		}
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		if m.settingsDirty() {
			m.confirmingSettingsExit = true
			m.settingsInputActive = false
			return m, nil
		}
		m.screen = listScreen
		return m, nil
	case "ctrl+s":
		m.commitSettingsInput()
		if err := m.settings.Validate(); err != nil {
			m.settingsStatus = "Error: " + err.Error()
			return m, nil
		}
		return m, m.saveSettings
	case "up", "shift+tab":
		m.commitSettingsInput()
		items := m.settingsItems()
		m.settingsFocus = items[previousSettingIndex(items, m.settingsFocus)]
		m.loadSettingsInput()
	case "down", "tab":
		m.commitSettingsInput()
		items := m.settingsItems()
		m.settingsFocus = items[nextSettingIndex(items, m.settingsFocus)]
		m.loadSettingsInput()
	case "left", "right":
		if m.settingsFocus == settingTheme {
			index := 0
			for i, flavor := range catppuccinFlavors {
				if m.settings.Theme.Flavor == flavor {
					index = i
				}
			}
			if key.String() == "right" {
				index = (index + 1) % len(catppuccinFlavors)
			} else {
				index = (index + len(catppuccinFlavors) - 1) % len(catppuccinFlavors)
			}
			m.settings.Theme.Flavor = catppuccinFlavors[index]
			m.applySelectedTheme()
			m.settingsStatus = "Theme changed; press ctrl-s to save"
		}
	case "enter":
		m.commitSettingsInput()
		switch m.settingsFocus {
		case settingPathStyle:
			m.settings.Storage.PathStyle = !m.settings.Storage.PathStyle
		case settingAdvanced:
			m.settingsAdvanced = !m.settingsAdvanced
			m.loadSettingsInput()
		case settingTheme:
			m.settings.Theme.Flavor = nextCatppuccinFlavor(m.settings.Theme.Flavor)
			m.applySelectedTheme()
			m.settingsStatus = "Theme changed; press ctrl-s to save"
		case settingRefresh:
			if err := m.settings.Storage.Validate(); err != nil {
				m.storageState = storage.NotConfigured
				m.settingsStatus = "Storage unavailable: " + err.Error()
				return m, nil
			}
			checker, err := storage.NewS3(m.settings.Storage)
			if err != nil {
				m.settingsStatus = "S3 error: " + err.Error()
				return m, nil
			}
			m.settingsChecker = checker
			m.storageState = storage.Checking
			m.settingsStatus = "Checking S3..."
			m.s3CheckID++
			return m, tea.Batch(m.checkS3, m.storageSpinner.Tick)
		case settingClear:
			m.confirmingClear = true
			m.settingsStatus = ""
		}
	case "backspace":
		if m.settingsInputActive {
			m.settingsInput, _ = m.settingsInput.Update(msg)
			m.commitSettingsInput()
		}
	default:
		if m.settingsInputActive {
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			m.commitSettingsInput()
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) loadSettingsInput() {
	m.settingsInput = textinput.New()
	m.settingsInput.Width = m.contentWidth() - 30
	if m.settingsInput.Width < 12 {
		m.settingsInput.Width = 12
	}
	value := m.settingsValue()
	m.settingsInput.SetValue(value)
	m.settingsInputActive = m.settingsFocus == settingRevision || m.settingsFocus == settingRegion || m.settingsFocus == settingBucket || m.settingsFocus == settingAccessKey || m.settingsFocus == settingSecretKey || m.settingsFocus == settingEndpoint || m.settingsFocus == settingPrefix || m.settingsFocus == settingSessionToken
	if m.settingsInputActive {
		if m.settingsFocus == settingSecretKey || m.settingsFocus == settingSessionToken {
			m.settingsInput.EchoMode = textinput.EchoPassword
		}
		m.settingsInput.Focus()
	}
}

func (m Model) settingsValue() string {
	switch m.settingsFocus {
	case settingTheme:
		return m.settings.Theme.Flavor
	case settingRevision:
		return strconv.Itoa(m.settings.RevisionRetention)
	case settingEndpoint:
		return m.settings.Storage.Endpoint
	case settingRegion:
		return m.settings.Storage.Region
	case settingBucket:
		return m.settings.Storage.Bucket
	case settingPrefix:
		return m.settings.Storage.Prefix
	case settingPathStyle:
		return strconv.FormatBool(m.settings.Storage.PathStyle)
	case settingAccessKey:
		return m.settings.Storage.AccessKeyID
	case settingSecretKey:
		return m.settings.Storage.SecretAccessKey
	case settingSessionToken:
		return m.settings.Storage.SessionToken
	}
	return ""
}

func (m *Model) commitSettingsInput() {
	if !m.settingsInputActive {
		return
	}
	value := m.settingsInput.Value()
	switch m.settingsFocus {
	case settingRevision:
		if revision, err := strconv.Atoi(value); err == nil {
			m.settings.RevisionRetention = revision
		}
	case settingEndpoint:
		m.settings.Storage.Endpoint = value
	case settingRegion:
		m.settings.Storage.Region = value
	case settingBucket:
		m.settings.Storage.Bucket = value
	case settingPrefix:
		m.settings.Storage.Prefix = value
	case settingAccessKey:
		m.settings.Storage.AccessKeyID = value
	case settingSecretKey:
		m.settings.Storage.SecretAccessKey = value
	case settingSessionToken:
		m.settings.Storage.SessionToken = value
	}
}

func (m Model) settingsDirty() bool { return !reflect.DeepEqual(m.settings, m.savedSettings) }

func (m Model) settingsItems() []int {
	items := make([]int, 0, len(settingDescriptors))
	for _, descriptor := range settingDescriptors {
		if descriptor.focusable && (!descriptor.advanced || m.settingsAdvanced) {
			items = append(items, descriptor.key)
		}
	}
	return items
}

func nextSettingIndex(items []int, focus int) int {
	for i, item := range items {
		if item == focus {
			return (i + 1) % len(items)
		}
	}
	return 0
}

func previousSettingIndex(items []int, focus int) int {
	for i, item := range items {
		if item == focus {
			return (i + len(items) - 1) % len(items)
		}
	}
	return 0
}

func (m *Model) clearS3() {
	m.settings.Storage = config.StorageConfig{Region: "us-east-1"}
	m.settingsChecker = nil
	m.storageState = storage.NotConfigured
	m.settingsStatus = "S3 configuration cleared; press ctrl-s to save"
}

func (m *Model) applySelectedTheme() {
	palette, err := theme.Resolve(m.settings.Theme)
	if err != nil {
		m.settingsStatus = "Error: " + err.Error()
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

func (m Model) saveSettings() tea.Msg {
	palette, err := theme.Resolve(m.settings.Theme)
	if err != nil {
		return errMsg{err}
	}
	if err := config.Save(m.configPath, m.settings); err != nil {
		return errMsg{err}
	}
	return configSavedMsg{cfg: m.settings, palette: palette}
}

func (m Model) checkS3() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s3CheckedMsg{id: m.s3CheckID, err: m.settingsChecker.CheckConnection(ctx)}
}

func isUnauthorized(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "403") || strings.Contains(message, "accessdenied") || strings.Contains(message, "unauthorized")
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
	var content string
	switch m.screen {
	case editScreen:
		content = view.RenderEditor(view.EditorModel{
			Width: m.width, Height: m.height, Title: m.title.Value(), TitleView: m.title.View(), TagsView: m.tags.View(), BodyView: m.editor.View(),
			TitleActive: m.focus == 0, TagsActive: m.focus == 1, BodyActive: m.focus == 2,
			State: func() string {
				if m.dirty {
					return "UNSAVED"
				}
				return "SAVED"
			}(), SyncStatus: string(m.savedNote.SyncStatus), Status: m.status, ConfirmingExit: m.confirmingExit,
		}, m.palette)
	case viewScreen:
		content = view.RenderReader(view.ReaderModel{Width: m.width, Height: m.height, Title: m.viewNote.Title, Tags: m.viewNote.Tags, Body: m.viewer.View(), SyncStatus: string(m.viewNote.SyncStatus)}, m.palette)
	case settingsScreen:
		content = view.RenderSettings(view.SettingsModel{Width: m.width, Height: m.height, Rows: m.settingRows(), Status: m.settingsStatus, Dirty: m.settingsDirty(), Input: m.settingsInput.View(), InputActive: m.settingsInputActive}, m.palette)
	default:
		rows := make([]view.NoteRow, len(m.notes))
		for i, note := range m.notes {
			rows[i] = view.NoteRow{Title: note.Title, Updated: note.UpdatedAt.Local().Format("2006-01-02 15:04"), Excerpt: note.Content, Tags: note.Tags, SyncStatus: string(note.SyncStatus), Selected: !m.settingsFocused && i == m.cursor}
		}
		searchInput := ""
		if m.searching {
			searchInput = m.search.View()
		}
		content = view.RenderList(view.ListModel{Width: m.width, Height: m.height, Count: len(m.notes), Search: m.search.Value(), SearchInput: searchInput, Rows: rows, Status: m.status, SettingsFocused: m.settingsFocused}, m.palette)
	}
	if m.confirmingExit {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Discard unsaved note changes?", Message: "Save or discard the note before leaving.", Actions: "[s] Save   [d] Discard   [c] Cancel"}, m.width, m.height, m.palette)
	}
	if m.conflictLoading {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Loading conflict...", Message: "Fetching the remote note version.", Actions: "[Ctrl-C] Exit"}, m.width, m.height, m.palette)
	}
	if m.conflictActive {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Note conflict", Message: "Local and remote versions both changed.", Actions: "[l] Keep local   [r] Accept remote   [c] Copy remote   [Ctrl-C] Exit"}, m.width, m.height, m.palette)
	}
	if m.confirmingSettingsExit {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Leave settings?", Message: "Your settings have not been saved.", Actions: "[s] Save   [d] Discard   [c] Cancel"}, m.width, m.height, m.palette)
	}
	if m.confirmingClear {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Clear S3 configuration?", Message: "This removes all stored S3 settings from the pending configuration.", Actions: "[c] Clear   [Esc] Cancel"}, m.width, m.height, m.palette)
	}
	if m.confirmingDelete {
		return view.RenderConfirmModal(content, view.ConfirmModel{Title: "Delete note?", Message: "The note will be deleted locally and from S3 when storage is connected.", Actions: "[d] Delete   [c] Cancel"}, m.width, m.height, m.palette)
	}
	return content
}

func (m Model) settingRows() []view.SettingRow {
	rows := make([]view.SettingRow, 0, len(settingDescriptors))
	for _, descriptor := range settingDescriptors {
		if descriptor.advanced && !m.settingsAdvanced {
			continue
		}
		rows = append(rows, view.SettingRow{
			Section:  descriptor.section,
			Label:    descriptor.label,
			Value:    m.settingDisplayValue(descriptor.key),
			Selected: descriptor.focusable && descriptor.key == m.settingsFocus,
			Action:   descriptor.action,
		})
	}
	return rows
}

func (m Model) settingDisplayValue(key int) string {
	secret := func(value string) string {
		if value == "" {
			return "(empty)"
		}
		return strings.Repeat("*", minInt(len(value), 16))
	}
	switch key {
	case settingTheme:
		return "< " + m.settings.Theme.Flavor + " >"
	case settingRevision:
		return strconv.Itoa(m.settings.RevisionRetention)
	case settingRegion:
		return m.settings.Storage.Region
	case settingBucket:
		return m.settings.Storage.Bucket
	case settingAccessKey:
		return secret(m.settings.Storage.AccessKeyID)
	case settingSecretKey:
		return secret(m.settings.Storage.SecretAccessKey)
	case settingStatus:
		return m.storageStatusLabel()
	case settingEndpoint:
		return m.settings.Storage.Endpoint
	case settingPrefix:
		return m.settings.Storage.Prefix
	case settingPathStyle:
		return strconv.FormatBool(m.settings.Storage.PathStyle)
	case settingSessionToken:
		return secret(m.settings.Storage.SessionToken)
	default:
		return ""
	}
}

func (m Model) storageStatusLabel() string {
	switch m.storageState {
	case storage.Checking:
		return m.storageSpinner.View() + " checking"
	case storage.Connected:
		return "✓ connected"
	case storage.Unauthorized:
		return "✗ access denied"
	case storage.Unavailable:
		return "✗ unavailable"
	default:
		return "! not configured"
	}
}

func nextCatppuccinFlavor(current string) string {
	for i, flavor := range catppuccinFlavors {
		if current == flavor {
			return catppuccinFlavors[(i+1)%len(catppuccinFlavors)]
		}
	}
	return catppuccinFlavors[0]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func generatedTitle() string { return notes.GeneratedTitle() }

func parseFilter(value string) notes.Filter { return notes.ParseFilter(value) }
