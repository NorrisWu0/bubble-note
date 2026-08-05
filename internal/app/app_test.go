package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/domain"
)

type behaviorStore struct {
	note        domain.Note
	saveCount   int
	createCount int
	failSave    bool
}

func (s *behaviorStore) CreateNote(title, content string, tags []string) (domain.Note, error) {
	s.createCount++
	s.note = domain.Note{ID: "created", Title: title, Content: content, Tags: tags}
	return s.note, nil
}

func (s *behaviorStore) GetNote(string) (domain.Note, error) { return s.note, nil }

func (s *behaviorStore) ListNotes(domain.NoteFilter) ([]domain.Note, error) {
	if s.note.ID == "" {
		return nil, nil
	}
	return []domain.Note{s.note}, nil
}

func (s *behaviorStore) SaveNote(id, title, content string, tags []string) (domain.Note, error) {
	s.saveCount++
	if s.failSave {
		return domain.Note{}, errors.New("save failed")
	}
	s.note = domain.Note{ID: id, Title: title, Content: content, Tags: tags}
	return s.note, nil
}

func (s *behaviorStore) DeleteNote(string) error { return nil }

func (s *behaviorStore) ListRevisions(string) ([]domain.Revision, error) { return nil, nil }

func (s *behaviorStore) RestoreRevision(string, string) (domain.Note, error) { return s.note, nil }

func (s *behaviorStore) Close() error { return nil }

type noteBehavior struct {
	model Model
	store *behaviorStore
}

func openSavedNote() noteBehavior {
	store := &behaviorStore{note: domain.Note{
		ID:        "note-1",
		Title:     "quiet-cat",
		Content:   "original body",
		Tags:      []string{"work"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}
	model := New(store)
	model.beginEdit(store.note)
	return noteBehavior{model: model, store: store}
}

func (b *noteBehavior) editTitle() {
	b.press(tea.KeyMsg{Type: tea.KeyEsc})
	b.press(tea.KeyMsg{Type: tea.KeyShiftTab})
	b.press(tea.KeyMsg{Type: tea.KeyShiftTab})
	b.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" updated")})
}

func (b *noteBehavior) editTags() {
	b.press(tea.KeyMsg{Type: tea.KeyEsc})
	b.press(tea.KeyMsg{Type: tea.KeyShiftTab})
	b.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(", urgent")})
}

func (b *noteBehavior) editBody() {
	b.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" more")})
}

func (b *noteBehavior) press(key tea.KeyMsg) {
	updated, cmd := b.model.Update(key)
	b.model = updated.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = b.model.Update(msg)
			b.model = updated.(Model)
		}
	}
}

func (b *noteBehavior) save() {
	b.press(tea.KeyMsg{Type: tea.KeyCtrlS})
}

func (b *noteBehavior) expectUnsavedPrompt(t *testing.T) {
	t.Helper()
	if !b.model.confirmingExit {
		t.Fatal("expected unsaved changes prompt")
	}
}

func TestUserCanEditTitleTagsAndBody(t *testing.T) {
	tests := []struct {
		name string
		edit func(*noteBehavior)
	}{
		{name: "title", edit: (*noteBehavior).editTitle},
		{name: "tags", edit: (*noteBehavior).editTags},
		{name: "body", edit: (*noteBehavior).editBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := openSavedNote()
			tt.edit(&note)
			if !note.model.dirty {
				t.Fatalf("editing %s should mark the note unsaved", tt.name)
			}
		})
	}
}

func TestUserCannotSaveAnUnchangedNote(t *testing.T) {
	note := openSavedNote()
	note.save()
	if note.store.saveCount != 0 {
		t.Fatal("saving an unchanged note should not create a revision")
	}
}

func TestUserSavesAnEditedNoteWithoutLeavingIt(t *testing.T) {
	note := openSavedNote()
	note.editBody()
	note.save()
	if note.store.saveCount != 1 {
		t.Fatalf("save count = %d, want 1", note.store.saveCount)
	}
	if note.model.screen != editScreen {
		t.Fatal("saving should keep the note open")
	}
	if note.model.dirty {
		t.Fatal("saved note should no longer be unsaved")
	}
}

func TestUserIsWarnedBeforeLeavingAnEditedNote(t *testing.T) {
	exits := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "escape", key: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "interrupt", key: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}
	for _, exit := range exits {
		t.Run(exit.name, func(t *testing.T) {
			note := openSavedNote()
			note.editBody()
			note.press(exit.key)
			if exit.name == "escape" {
				note.press(tea.KeyMsg{Type: tea.KeyEsc})
			}
			note.expectUnsavedPrompt(t)
		})
	}
}

func TestUserCanTypeQWhileEditing(t *testing.T) {
	note := openSavedNote()
	note.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if note.model.screen != editScreen {
		t.Fatal("typing q should keep the note open")
	}
	if !note.model.dirty {
		t.Fatal("typing q should mark the note unsaved")
	}
	if note.model.confirmingExit {
		t.Fatal("typing q should not open the exit prompt")
	}
}

func TestUserCanMoveBackToThePreviousField(t *testing.T) {
	note := openSavedNote()
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyShiftTab})
	note.press(tea.KeyMsg{Type: tea.KeyShiftTab})
	note.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if note.model.title.Value() != "quiet-cat!" {
		t.Fatalf("title = %q, want quiet-cat!", note.model.title.Value())
	}
	if note.model.tags.Value() != "work" {
		t.Fatalf("tags = %q, want work", note.model.tags.Value())
	}
}

func TestUserCanIndentWhileEditingTheBody(t *testing.T) {
	note := openSavedNote()
	note.press(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.HasSuffix(note.model.editor.Value(), "  ") {
		t.Fatal("tab should insert indentation while editing the body")
	}
}

func TestUserCanDiscardEditedChanges(t *testing.T) {
	note := openSavedNote()
	note.editBody()
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if note.model.screen != listScreen {
		t.Fatal("discarding should leave the editor")
	}
	if note.store.saveCount != 0 {
		t.Fatal("discarding should not save a revision")
	}
}

func TestUserCanCancelLeavingAnEditedNote(t *testing.T) {
	note := openSavedNote()
	note.editBody()
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if note.model.screen != editScreen {
		t.Fatal("cancelling should keep the note open")
	}
	if !note.model.dirty {
		t.Fatal("cancelling should keep changes unsaved")
	}
}

func TestUserCanSaveWhileLeavingAnEditedNote(t *testing.T) {
	note := openSavedNote()
	note.editBody()
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	note.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if note.model.screen != listScreen {
		t.Fatal("saving from the prompt should leave the editor")
	}
	if note.store.saveCount != 1 {
		t.Fatalf("save count = %d, want 1", note.store.saveCount)
	}
}

func TestUserStaysInTheEditorWhenSavingFails(t *testing.T) {
	note := openSavedNote()
	note.store.failSave = true
	note.editBody()
	note.save()
	if note.model.screen != editScreen {
		t.Fatal("failed save should keep the note open")
	}
	if !note.model.dirty {
		t.Fatal("failed save should keep changes unsaved")
	}
}

func TestParseFilter(t *testing.T) {
	filter := parseFilter("jasmine tag:shopping after:2026-01-01 before:2026-02-01")
	if filter.Query != "jasmine" {
		t.Fatalf("query = %q, want jasmine", filter.Query)
	}
	if filter.Tag != "shopping" {
		t.Fatalf("tag = %q, want shopping", filter.Tag)
	}
	if filter.From == nil || filter.From.Format("2006-01-02") != "2026-01-01" {
		t.Fatalf("from = %v, want 2026-01-01", filter.From)
	}
	if filter.Through == nil || filter.Through.Format("2006-01-02") != "2026-02-01" {
		t.Fatalf("through = %v, want 2026-02-01", filter.Through)
	}
}

func TestGeneratedTitle(t *testing.T) {
	title := generatedTitle()
	if len(title) < 3 {
		t.Fatalf("generated title = %q, unexpectedly short", title)
	}
	for _, character := range title {
		if character == '-' {
			return
		}
	}
	t.Fatalf("generated title = %q, want adjective-animal format", title)
}

func TestEditorFrameShowsNoteStateAndKeyHints(t *testing.T) {
	note := openSavedNote()
	note.model.width = 80
	note.model.height = 24
	note.editBody()
	view := note.model.View()
	for _, text := range []string{
		"bubble-note / quiet-cat",
		"[UNSAVED]",
		"ctrl-s save",
		"tab indent",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("view does not contain %q:\n%s", text, view)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("line is %d columns wide, want at most 80: %q", lipgloss.Width(line), line)
		}
	}
}

func TestEditorFrameShowsHowToEnterBodyEditing(t *testing.T) {
	note := openSavedNote()
	note.press(tea.KeyMsg{Type: tea.KeyEsc})
	view := note.model.View()
	if !strings.Contains(view, "BODY  [press Enter to edit]") {
		t.Fatalf("view does not explain body editing:\n%s", view)
	}
	if !strings.Contains(view, "enter edit body") {
		t.Fatalf("view does not show body edit keybinding:\n%s", view)
	}
}

func TestEditorFrameFitsANarrowTerminal(t *testing.T) {
	note := openSavedNote()
	note.model.width = 40
	note.model.height = 16
	view := note.model.View()
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line is %d columns wide, want at most 40: %q", lipgloss.Width(line), line)
		}
	}
}
