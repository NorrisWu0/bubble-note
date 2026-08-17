package view

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/norriswu0/bubble-note/internal/theme"
)

func rgbSequence(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func TestRenderMarkdownStylesContent(t *testing.T) {
	palette := theme.Default()
	out := RenderMarkdown(Note{Title: "t", Content: "# Heading\n\n**bold** text"}, 60, palette)
	if strings.Contains(out, "**") {
		t.Fatalf("markdown should be rendered, got raw emphasis:\n%s", out)
	}
	if !strings.Contains(out, "Heading") {
		t.Fatalf("heading text missing:\n%s", out)
	}
	if !strings.Contains(out, rgbSequence(palette.Primary)) {
		t.Fatalf("heading should use theme primary color %q:\n%s", palette.Primary, out)
	}
}

func TestRenderMarkdownFallsBackToRawContent(t *testing.T) {
	out := RenderMarkdown(Note{Content: "plain"}, 60, theme.Default())
	if !strings.Contains(out, "plain") {
		t.Fatalf("output = %q, want plain content", out)
	}
}
