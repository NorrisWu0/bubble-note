package theme

import (
	"testing"

	"github.com/norriswu0/bubble-note/internal/config"
)

func TestResolveCatppuccinFlavor(t *testing.T) {
	palette, err := Resolve(config.ThemeConfig{Preset: "catppuccin", Flavor: "macchiato"})
	if err != nil {
		t.Fatal(err)
	}
	if palette.Background != "#24273a" {
		t.Fatalf("background = %q, want #24273a", palette.Background)
	}
}

func TestResolveThemeOverride(t *testing.T) {
	palette, err := Resolve(config.ThemeConfig{
		Preset: "catppuccin",
		Flavor: "mocha",
		Overrides: map[string]string{
			"primary": "#123456",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if palette.Primary != "#123456" {
		t.Fatalf("primary = %q, want #123456", palette.Primary)
	}
}

func TestResolveRejectsUnknownTheme(t *testing.T) {
	if _, err := Resolve(config.ThemeConfig{Preset: "dracula", Flavor: "default"}); err == nil {
		t.Fatal("expected unsupported theme error")
	}
}
