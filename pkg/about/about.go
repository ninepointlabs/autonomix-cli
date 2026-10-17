// Package about holds the facts shown by the TUI's About screen and the
// about command: who makes Autonomix CLI, its license, support and privacy.
package about

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	Name         = "Autonomix CLI"
	Tagline      = "Install and update Linux apps from GitHub releases."
	Organization = "Nine Point Labs"
	SupportEmail = "help@ninepointlabs.com"
	Created      = "January 2026" // first commit: 2026-01-13
	LicenseName  = "MIT License"
	Copyright    = "© 2026 Nine Point Labs" // as written in LICENSE

	Privacy = "No accounts and no telemetry. Autonomix CLI only talks to GitHub: " +
		"api.github.com to look up the latest release of each tracked repository, " +
		"Autonomix CLI included (when the TUI opens, and when you add, check, update or " +
		"install an app), and GitHub's release downloads when you install a package. " +
		"If GITHUB_TOKEN or GH_TOKEN is set, it is sent only to api.github.com and never saved. " +
		"Your tracked apps are kept in ~/.autonomix/config.json. Downloaded packages go to " +
		"the system temp directory and are deleted after installing. Installed versions are " +
		"read from your local package managers. Release pages and the Email action open " +
		"in your own browser or mail app."
)

// Info is the About screen for one build of the app.
type Info struct {
	Version     string // from main.version, set by goreleaser/PKGBUILD ldflags
	LicenseText string // the repo's LICENSE, embedded at build time
}

// depVersion returns the version of a module this binary was built with.
func depVersion(path string) string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == path {
				if d.Replace != nil {
					return d.Replace.Version
				}
				return d.Version
			}
		}
	}
	return "unknown"
}

// BuiltWith lists the toolkit versions, e.g. "Go 1.25.5, Bubble Tea v1.3.10, ...".
func BuiltWith() string {
	return fmt.Sprintf("Go %s, Bubble Tea %s, Bubbles %s, Lip Gloss %s",
		strings.TrimPrefix(runtime.Version(), "go"),
		depVersion("github.com/charmbracelet/bubbletea"),
		depVersion("github.com/charmbracelet/bubbles"),
		depVersion("github.com/charmbracelet/lipgloss"))
}

// osName returns PRETTY_NAME from /etc/os-release, or "".
func osName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// Details are the version and system details to paste into a support email.
func (i Info) Details() string {
	system := runtime.GOOS + "/" + runtime.GOARCH
	if n := osName(); n != "" {
		system = n + ", " + system
	}
	if d := os.Getenv("XDG_CURRENT_DESKTOP"); d != "" {
		system += ", " + d
	}
	return fmt.Sprintf("%s %s\n%s\n%s", Name, i.Version, BuiltWith(), system)
}

// mailEscape escapes s for a mailto: query, with %20 for spaces since mail
// clients show "+" literally.
func mailEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// MailtoURL is a support email with the version and system details filled in.
func (i Info) MailtoURL() string {
	return "mailto:" + SupportEmail +
		"?subject=" + mailEscape(Name+" "+i.Version) +
		"&body=" + mailEscape("\n\n--\n"+i.Details())
}

// Text is the About screen as plain text, for the about command.
func (i Info) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n%s\n\n", Name, i.Version, Tagline)
	rows := [][2]string{
		{"Made by", Organization},
		{"Created", Created},
		{"License", LicenseName + ", " + Copyright},
		{"Support", SupportEmail},
		{"Privacy", Privacy},
		{"Built with", BuiltWith()},
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "%-11s %s\n", r[0]+":", r[1])
	}
	return b.String()
}
