package theme

import (
	"fmt"
	"strings"

	catppuccin "github.com/catppuccin/go"
	"github.com/norriswu0/bubble-note/internal/config"
)

type Palette struct {
	Background string
	Surface    string
	Text       string
	Muted      string
	Primary    string
	Secondary  string
	Selected   string
	Border     string
	Danger     string
}

func Default() Palette {
	palette, err := Resolve(config.Default().Theme)
	if err != nil {
		panic(err)
	}
	return palette
}

func Resolve(cfg config.ThemeConfig) (Palette, error) {
	preset := strings.ToLower(strings.TrimSpace(cfg.Preset))
	if preset == "" {
		preset = "catppuccin"
	}
	if preset != "catppuccin" {
		return Palette{}, fmt.Errorf("unsupported theme preset %q", cfg.Preset)
	}

	flavor := strings.ToLower(strings.TrimSpace(cfg.Flavor))
	if flavor == "" {
		flavor = "mocha"
	}
	colors := catppuccin.Variant(flavor)
	if colors == nil {
		return Palette{}, fmt.Errorf("unsupported Catppuccin flavor %q", cfg.Flavor)
	}

	palette := Palette{
		Background: colors.Base().Hex,
		Surface:    colors.Surface0().Hex,
		Text:       colors.Text().Hex,
		Muted:      colors.Subtext0().Hex,
		Primary:    colors.Mauve().Hex,
		Secondary:  colors.Teal().Hex,
		Selected:   colors.Lavender().Hex,
		Border:     colors.Surface2().Hex,
		Danger:     colors.Red().Hex,
	}
	if err := applyOverrides(&palette, cfg.Overrides); err != nil {
		return Palette{}, err
	}
	return palette, nil
}

func applyOverrides(palette *Palette, overrides map[string]string) error {
	for name, value := range overrides {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("theme override %q cannot be empty", name)
		}
		switch strings.ToLower(name) {
		case "background":
			palette.Background = value
		case "surface":
			palette.Surface = value
		case "text":
			palette.Text = value
		case "muted":
			palette.Muted = value
		case "primary":
			palette.Primary = value
		case "secondary":
			palette.Secondary = value
		case "selected":
			palette.Selected = value
		case "border":
			palette.Border = value
		case "danger":
			palette.Danger = value
		default:
			return fmt.Errorf("unsupported theme override %q", name)
		}
	}
	return nil
}
