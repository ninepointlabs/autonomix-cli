package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/about"
)

func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return next.(Model)
}

func TestAboutScreen(t *testing.T) {
	m := NewModel(&config.Config{}, about.Info{Version: "1.2.3", LicenseText: "MIT License\n\nThe text."})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)

	m = press(t, m, "i")
	if m.state != viewAbout {
		t.Fatalf("i should open About, state = %v", m.state)
	}
	view := m.View()
	for _, want := range []string{"Autonomix CLI", "Version 1.2.3", "Nine Point Labs", "January 2026", "help@ninepointlabs.com", "No accounts", "Bubble Tea"} {
		if !strings.Contains(view, want) {
			t.Errorf("About view is missing %q", want)
		}
	}

	m = press(t, m, "l")
	if !m.aboutLicense || !strings.Contains(m.View(), "The text.") {
		t.Errorf("l should show the license text")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.aboutLicense || m.state != viewAbout {
		t.Errorf("esc should close the license first")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.state != viewList {
		t.Errorf("esc should return to the list, state = %v", m.state)
	}
}

func TestAboutInHelp(t *testing.T) {
	m := NewModel(&config.Config{}, about.Info{})
	for _, b := range m.list.AdditionalShortHelpKeys() {
		if b.Help().Key == "i" && b.Help().Desc == "about" {
			return
		}
	}
	t.Error("i about is missing from the help")
}
