package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/git"
	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/theme"
)

type fakeStore struct {
	notes       map[string]notes.Note
	createCount int
	saveCount   int
	deleteCount int
}

func newFakeStore() *fakeStore {
	return &fakeStore{notes: map[string]notes.Note{}}
}

func (s *fakeStore) NotesDir() string { return "" }

func (s *fakeStore) Path(id string) (string, error) {
	return filepath.Join("/tmp", id, "README.md"), nil
}

func (s *fakeStore) Refresh() error { return nil }

func (s *fakeStore) Reload(id string) (notes.Note, error) {
	note := s.notes[id]
	note.UpdatedAt = time.Now()
	s.notes[id] = note
	return note, nil
}

func (s *fakeStore) CreateNote(title, content string, tags []string) (notes.Note, error) {
	s.createCount++
	note := notes.Note{ID: "created-" + itoa(s.createCount), Title: title, Content: content, Tags: notes.NormalizeTags(tags), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.notes[note.ID] = note
	return note, nil
}

func (s *fakeStore) GetNote(id string) (notes.Note, error) { return s.notes[id], nil }

func (s *fakeStore) ListNotes(notes.Filter) ([]notes.Note, error) {
	var result []notes.Note
	for _, note := range s.notes {
		result = append(result, note)
	}
	return result, nil
}

func (s *fakeStore) SaveNote(id, title, content string, tags []string) (notes.Note, error) {
	s.saveCount++
	note := s.notes[id]
	note.Title = title
	note.Content = content
	note.Tags = notes.NormalizeTags(tags)
	note.UpdatedAt = time.Now()
	s.notes[id] = note
	return note, nil
}

func (s *fakeStore) DeleteNote(id string) error {
	s.deleteCount++
	delete(s.notes, id)
	return nil
}

func testModel(t *testing.T, store *fakeStore) Model {
	t.Helper()
	cfg := config.Default()
	return New(store, cfg, filepath.Join(t.TempDir(), "config.yaml"), theme.Default())
}

func press(t *testing.T, model *Model, key tea.KeyMsg) {
	t.Helper()
	updated, cmd := model.Update(key)
	*model = updated.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = model.Update(msg)
			*model = updated.(Model)
		}
	}
}

func TestListNavigationAndView(t *testing.T) {
	store := newFakeStore()
	store.notes["a"] = notes.Note{ID: "a", Title: "first", Content: "hello", UpdatedAt: time.Now()}
	store.notes["b"] = notes.Note{ID: "b", Title: "second", Content: "world", UpdatedAt: time.Now()}
	model := testModel(t, store)
	model.notes = []notes.Note{store.notes["a"], store.notes["b"]}

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", model.cursor)
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != viewScreen {
		t.Fatal("enter should open the rendered view")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != listScreen {
		t.Fatal("esc should return to the list")
	}
}

func TestMissingEditorPromptsInstall(t *testing.T) {
	store := newFakeStore()
	store.notes["a"] = notes.Note{ID: "a", Title: "note", Content: "body", UpdatedAt: time.Now()}
	model := testModel(t, store)
	model.editor = "bubble-note-missing-editor-xyz"
	model.notes = []notes.Note{store.notes["a"]}

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !strings.Contains(model.status, "not found") {
		t.Fatalf("status = %q, want missing-editor prompt", model.status)
	}
}

func TestMissingGitClientPromptsInstall(t *testing.T) {
	store := newFakeStore()
	model := testModel(t, store)
	model.gitClient = "bubble-note-missing-git-xyz"

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !strings.Contains(model.status, "git client not found") {
		t.Fatalf("status = %q, want missing-git prompt", model.status)
	}
}

func TestNewNoteFormCreatesNote(t *testing.T) {
	store := newFakeStore()
	model := testModel(t, store)
	model.editor = "bubble-note-missing-editor-xyz"

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if model.screen != formScreen || model.form != formNew {
		t.Fatal("n should open the new-note form")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my note")})
	press(t, &model, tea.KeyMsg{Type: tea.KeyEnter})
	if store.createCount != 1 {
		t.Fatalf("create count = %d, want 1", store.createCount)
	}
	if model.screen != listScreen {
		t.Fatal("creating should return to the list when no editor is available")
	}
}

func TestEditFormSavesTitleAndTags(t *testing.T) {
	store := newFakeStore()
	store.notes["a"] = notes.Note{ID: "a", Title: "old", Content: "body", Tags: []string{"work"}, UpdatedAt: time.Now()}
	model := testModel(t, store)
	model.notes = []notes.Note{store.notes["a"]}

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if model.screen != formScreen || model.form != formEdit {
		t.Fatal("t should open the edit form")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyCtrlU})
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("renamed")})
	press(t, &model, tea.KeyMsg{Type: tea.KeyTab})
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(", urgent")})
	press(t, &model, tea.KeyMsg{Type: tea.KeyEnter})
	if store.saveCount != 1 {
		t.Fatalf("save count = %d, want 1", store.saveCount)
	}
	if store.notes["a"].Title != "renamed" {
		t.Fatalf("title = %q, want renamed", store.notes["a"].Title)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	store := newFakeStore()
	store.notes["a"] = notes.Note{ID: "a", Title: "keep", UpdatedAt: time.Now()}
	model := testModel(t, store)
	model.notes = []notes.Note{store.notes["a"]}

	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if !model.confirmingDelete {
		t.Fatal("delete should require confirmation")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if model.confirmingDelete || store.deleteCount != 0 {
		t.Fatal("cancel should preserve the note")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if store.deleteCount != 1 {
		t.Fatal("confirmed delete should delete the note")
	}
}

func TestSettingsCyclesThemeAndSaves(t *testing.T) {
	store := newFakeStore()
	model := testModel(t, store)
	model.beginSettings()
	press(t, &model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.settings.Theme.Flavor != "latte" {
		t.Fatalf("flavor = %q, want latte", model.settings.Theme.Flavor)
	}
	if !model.settingsDirty {
		t.Fatal("changing theme should mark settings dirty")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyCtrlS})
	if model.settingsDirty {
		t.Fatal("saving should clear the dirty flag")
	}
	if model.savedSettings.Theme.Flavor != "latte" {
		t.Fatalf("saved flavor = %q, want latte", model.savedSettings.Theme.Flavor)
	}
}

func TestSettingsEditsNotesDir(t *testing.T) {
	store := newFakeStore()
	model := testModel(t, store)
	model.beginSettings()

	press(t, &model, tea.KeyMsg{Type: tea.KeyDown})
	if model.settingsFocus != settingNotesDir {
		t.Fatalf("focus = %d, want notes dir", model.settingsFocus)
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyEnter})
	if !model.settingsActive {
		t.Fatal("enter should activate the notes dir input")
	}
	press(t, &model, tea.KeyMsg{Type: tea.KeyCtrlU})
	press(t, &model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/notes")})
	press(t, &model, tea.KeyMsg{Type: tea.KeyCtrlS})

	if model.settingsDirty {
		t.Fatal("saving should clear the dirty flag")
	}
	if model.savedSettings.NotesDir != "/tmp/notes" {
		t.Fatalf("saved notes dir = %q, want /tmp/notes", model.savedSettings.NotesDir)
	}
	cfg, err := config.Load(model.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NotesDir != "/tmp/notes" {
		t.Fatalf("config notes_dir = %q, want /tmp/notes", cfg.NotesDir)
	}
}

func TestGitStatusLabel(t *testing.T) {
	if got := gitStatusLabel(git.Info{}); got != "git: not initialized" {
		t.Fatalf("non-repo label = %q", got)
	}
}
