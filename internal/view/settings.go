package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/norriswu0/bubble-note/internal/theme"
)

type SettingRow struct {
	Section  string
	Label    string
	Value    string
	Selected bool
	Action   bool
}

type SettingsModel struct {
	Width       int
	Height      int
	Rows        []SettingRow
	Status      string
	Dirty       bool
	Input       string
	InputActive bool
	Hint        string
	HintOK      bool
}

func RenderSettings(model SettingsModel, palette theme.Palette) string {
	headerState := "SETTINGS"
	if model.Dirty {
		headerState = "SETTINGS / UNSAVED"
	}
	header := topBar(model.Width, "", headerState, palette)
	var body strings.Builder
	lastSection := ""
	for _, row := range model.Rows {
		if row.Section != lastSection {
			if body.Len() > 0 {
				body.WriteString("\n")
			}
			body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Secondary)).Bold(true).Render(row.Section) + "\n")
			lastSection = row.Section
		}
		marker := "  "
		if row.Selected {
			marker = ">>"
		}
		value := row.Value
		if row.Selected && model.InputActive {
			value = model.Input
		}
		line := fmt.Sprintf("%s %-22s %s", marker, row.Label, value)
		if row.Action {
			line = fmt.Sprintf("%s %-22s %s", marker, row.Label, value)
		}
		if row.Selected {
			line = selected(line, palette)
		}
		body.WriteString(line + "\n")
	}
	if model.Hint != "" {
		body.WriteString("\n")
		color := palette.Danger
		if model.HintOK {
			color = palette.Secondary
		}
		body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("  "+model.Hint) + "\n")
	}
	footer := "up/down navigate   enter edit/cycle   ctrl-s save   esc back"
	if model.Status != "" {
		footer = model.Status + "   |   " + footer
	}
	return header + "\n" + panel(body.String(), contentHeight(model.Height), false, model.Width, palette) + "\n" + footBar(footer, model.Width, palette)
}

type ConfirmModel struct {
	Title   string
	Message string
	Actions string
}

func RenderConfirmModal(content string, modal ConfirmModel, width, height int, palette theme.Palette) string {
	base := content
	if base == "" {
		base = " "
	}
	modalText := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(palette.Primary)).Render(modal.Title) + "\n\n" + modal.Message + "\n\n" + muted(modal.Actions, palette)
	modalWidth := width - 10
	if modalWidth < 32 {
		modalWidth = 32
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel(modalText, 0, true, modalWidth, palette), lipgloss.WithWhitespaceChars(" "))
}
