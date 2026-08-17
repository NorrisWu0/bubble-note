package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/theme"
)

func TestRenderFormKeepsWithinTerminalWidth(t *testing.T) {
	output := RenderForm(FormModel{
		Width:       40,
		Height:      16,
		Heading:     "NEW NOTE",
		TitleView:   "quiet-cat",
		TagsView:    "work",
		TitleActive: true,
	}, theme.Default())
	for _, line := range strings.Split(output, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line is %d columns wide, want at most 40: %q", lipgloss.Width(line), line)
		}
	}
	if !strings.Contains(output, "NEW NOTE") {
		t.Fatalf("form output does not contain heading:\n%s", output)
	}
}

func TestRenderReaderShowsRenderedBody(t *testing.T) {
	output := RenderReader(ReaderModel{Width: 80, Height: 24, Title: "quiet-cat", Body: "# rendered"}, theme.Default())
	if !strings.Contains(output, "rendered") {
		t.Fatalf("reader output does not contain body:\n%s", output)
	}
}

func TestRenderListShowsGitStatus(t *testing.T) {
	output := RenderList(ListModel{Width: 80, Height: 24, GitStatus: "git:main clean"}, theme.Default())
	if !strings.Contains(output, "git:main clean") {
		t.Fatalf("list output does not contain git status:\n%s", output)
	}
}
