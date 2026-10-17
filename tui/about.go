package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ninepointlabs/autonomix-cli/pkg/about"
)

var (
	aboutTitleStyle = list.DefaultStyles().Title
	aboutLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(12)
	aboutDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	aboutKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

var aboutKey = key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "about"))

// aboutDoneMsg reports the result of the Email or Copy action.
type aboutDoneMsg struct{ text string }

// copyToClipboard copies text with wl-copy, xclip or xsel, whichever is installed.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		tools := [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}
		for _, t := range tools {
			if _, err := exec.LookPath(t[0]); err != nil {
				continue
			}
			cmd := exec.Command(t[0], t[1:]...)
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err != nil {
				return aboutDoneMsg{fmt.Sprintf("Couldn't copy with %s: %v", t[0], err)}
			}
			return aboutDoneMsg{"Copied " + text}
		}
		return aboutDoneMsg{"No clipboard tool found (install wl-clipboard, xclip or xsel). The address is " + text}
	}
}

func (m Model) openAbout() Model {
	m.state = viewAbout
	m.aboutLicense = false
	m.aboutNote = ""
	return m
}

func (m Model) updateAbout(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "i":
		if m.aboutLicense {
			m.aboutLicense = false
			return m, nil
		}
		m.state = viewList
		return m, nil
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "l":
		m.aboutLicense = !m.aboutLicense
		m.licenseView.SetContent(lipgloss.NewStyle().Width(m.licenseView.Width).Render(m.about.LicenseText))
		m.licenseView.GotoTop()
		return m, nil
	case "e":
		if err := openBrowser(m.about.MailtoURL()); err != nil {
			m.aboutNote = fmt.Sprintf("Couldn't open your mail app: %v. Write to %s", err, about.SupportEmail)
		} else {
			m.aboutNote = "Opened a new email to " + about.SupportEmail
		}
		return m, nil
	case "c":
		return m, copyToClipboard(about.SupportEmail)
	}
	if m.aboutLicense {
		var cmd tea.Cmd
		m.licenseView, cmd = m.licenseView.Update(msg)
		return m, cmd
	}
	return m, nil
}

// sizeLicenseView fits the license viewport to the window.
func (m *Model) sizeLicenseView(width, height int) {
	h, v := docStyle.GetFrameSize()
	m.licenseView.Width = max(20, width-h)
	m.licenseView.Height = max(5, height-v-4)
	m.aboutWidth = width - h
}

func (m Model) aboutView() string {
	keys := aboutKeyStyle.Render("e") + aboutDimStyle.Render(" email · ") +
		aboutKeyStyle.Render("c") + aboutDimStyle.Render(" copy address · ") +
		aboutKeyStyle.Render("l") + aboutDimStyle.Render(" license · ") +
		aboutKeyStyle.Render("esc") + aboutDimStyle.Render(" back")

	if m.aboutLicense {
		return aboutTitleStyle.Render(about.LicenseName) + "\n\n" +
			m.licenseView.View() + "\n\n" +
			aboutDimStyle.Render("↑/↓ scroll · ") + aboutKeyStyle.Render("l") + aboutDimStyle.Render(" or ") +
			aboutKeyStyle.Render("esc") + aboutDimStyle.Render(" close license")
	}

	width := m.aboutWidth
	if width <= 0 {
		width = 80
	}
	value := lipgloss.NewStyle().Width(max(20, width-12))
	row := func(label, text string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, aboutLabelStyle.Render(label), value.Render(text))
	}

	var b strings.Builder
	b.WriteString(aboutTitleStyle.Render(about.Name) + "\n\n")
	b.WriteString(about.Tagline + "\n")
	b.WriteString(aboutDimStyle.Render("Version "+m.about.Version) + "\n\n")
	b.WriteString(row("Made by", about.Organization) + "\n")
	b.WriteString(row("Created", about.Created) + "\n")
	b.WriteString(row("License", about.LicenseName+", "+about.Copyright+aboutDimStyle.Render("  (l to read)")) + "\n")
	b.WriteString(row("Support", about.SupportEmail+aboutDimStyle.Render("  (e email, c copy)")) + "\n")
	b.WriteString(row("Privacy", about.Privacy) + "\n")
	b.WriteString(row("Built with", about.BuiltWith()) + "\n\n")
	b.WriteString(keys + "\n")
	if m.aboutNote != "" {
		b.WriteString("\n" + statusStyle.Render(m.aboutNote) + "\n")
	}
	return b.String()
}
