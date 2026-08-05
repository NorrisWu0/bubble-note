package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/theme"
)

func TestRenderEditorKeepsWithinTerminalWidth(t *testing.T) {
	output := RenderEditor(EditorModel{
		Width:      40,
		Height:     16,
		Title:      "quiet-cat",
		TitleView:  "quiet-cat",
		TagsView:   "work",
		BodyView:   "body",
		BodyActive: true,
		State:      "UNSAVED",
	}, theme.Default())
	for _, line := range strings.Split(output, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line is %d columns wide, want at most 40: %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderReaderShowsRenderedBody(t *testing.T) {
	output := RenderReader(ReaderModel{Width: 80, Height: 24, Title: "quiet-cat", Body: "# rendered"}, theme.Default())
	if !strings.Contains(output, "rendered") {
		t.Fatalf("reader output does not contain body:\n%s", output)
	}
}
