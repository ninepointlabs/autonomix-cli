package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/updater"
)

// appJSON is how a tracked app appears in --json output.
type appJSON struct {
	Name          string         `json:"name"`
	Repo          string         `json:"repo"` // owner/repo
	RepoURL       string         `json:"repo_url"`
	Status        updater.Status `json:"status"`
	Installed     string         `json:"installed"`      // installed version, empty if not installed
	InstalledType string         `json:"installed_type"` // pacman, deb, rpm, flatpak, snap, binary, or empty
	Latest        string         `json:"latest"`         // latest release tag, empty if never checked
	ReleaseURL    string         `json:"release_url"`
	LastChecked   string         `json:"last_checked"`
	Updated       bool           `json:"updated,omitempty"` // installed by this run
	Error         string         `json:"error,omitempty"`
}

func latestTag(r updater.Result) string {
	if r.Release != nil {
		return r.Release.TagName
	}
	return r.App.Latest
}

func appView(r updater.Result) appJSON {
	repoURL := strings.TrimSuffix(r.App.RepoURL, "/")
	v := appJSON{
		Name:        r.App.Name,
		Repo:        strings.TrimPrefix(repoURL, "https://github.com/"),
		RepoURL:     repoURL,
		Status:      r.Status,
		Latest:      latestTag(r),
		LastChecked: r.App.LastChecked,
	}

	if r.Status != updater.NotInstalled && r.Status != updater.Failed {
		v.Installed = r.Installed
		v.InstalledType = string(r.InstalledType)
		if r.InstalledType == packages.Unknown {
			v.InstalledType = "binary" // found on PATH, not through a package manager
		}
	}

	if r.Release != nil && r.Release.HTMLURL != "" {
		v.ReleaseURL = r.Release.HTMLURL
	} else if v.Latest != "" {
		v.ReleaseURL = repoURL + "/releases/tag/" + v.Latest
	}

	if r.Err != nil {
		v.Error = r.Err.Error()
	}
	return v
}

func appViews(results []updater.Result) []appJSON {
	views := make([]appJSON, 0, len(results))
	for _, r := range results {
		views = append(views, appView(r))
	}
	return views
}

// printJSON writes v to stdout and returns the exit code.
func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
		return 1
	}
	return 0
}

// fail reports an error, as {"error": ...} on stdout in JSON mode, and
// returns the exit code.
func fail(o opts, format string, a ...any) int {
	msg := fmt.Sprintf(format, a...)
	if o.json {
		printJSON(map[string]string{"error": msg})
	} else {
		fmt.Println(msg)
	}
	return 1
}

// logf prints progress, to stderr in JSON mode so stdout stays parseable.
func logf(o opts, format string, a ...any) {
	if o.json {
		fmt.Fprintf(os.Stderr, format, a...)
	} else {
		fmt.Printf(format, a...)
	}
}
