package updater

import (
	"strings"
	"time"

	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/installer"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
)

type Status int

const (
	UpToDate Status = iota
	UpdateAvailable
	NotInstalled
	// Unmanaged means the app was found, but not through the system package
	// manager (e.g. a binary on PATH installed by mise), so updating it with a
	// release package could install a second copy.
	Unmanaged
	Failed
)

// Result is the outcome of checking a single tracked app.
type Result struct {
	Index         int
	App           config.App
	Installed     string
	InstalledType packages.Type
	Release       *github.Release
	Status        Status
	Err           error
}

// NormalizeVersion strips the "v" prefix and package revision suffixes
// (e.g. "v0.1.1" -> "0.1.1", "0.1.1-1" -> "0.1.1") so versions can be compared.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.LastIndex(v, "-"); idx > 0 {
		// Only strip if what follows the dash looks like a package revision
		// (numeric Debian revision or RPM "el" dist tag)
		suffix := v[idx+1:]
		if len(suffix) > 0 && (isNumeric(suffix) || strings.Contains(suffix, "el")) {
			v = v[:idx]
		}
	}
	return v
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// DetectInstalled looks the app up by its name, then by its repo name.
func DetectInstalled(app config.App) (string, packages.Type, bool) {
	if ver, t, ok := system.CheckInstalled(app.Name); ok {
		return ver, t, true
	}
	parts := strings.Split(strings.TrimSuffix(app.RepoURL, "/"), "/")
	if repoName := parts[len(parts)-1]; repoName != "" && repoName != app.Name {
		return system.CheckInstalled(repoName)
	}
	return "", packages.Unknown, false
}

// Evaluate decides the status of an app from what is installed and the latest release tag.
func Evaluate(installed string, installedType packages.Type, found bool, sysType packages.Type, latestTag string) Status {
	if !found {
		return NotInstalled
	}
	if installedType != sysType {
		return Unmanaged
	}
	if latest := NormalizeVersion(latestTag); latest != "" && latest != NormalizeVersion(installed) {
		return UpdateAvailable
	}
	return UpToDate
}

// Check fetches the latest release for every tracked app and records the
// results in cfg. The caller is responsible for saving cfg.
func Check(cfg *config.Config) []Result {
	sysType := system.GetSystemPreferredType()
	now := time.Now().Format(time.RFC3339)

	results := make([]Result, 0, len(cfg.Apps))
	for i, app := range cfg.Apps {
		r := Result{Index: i, App: app}

		rel, err := github.GetLatestRelease(app.RepoURL)
		if err != nil {
			r.Status = Failed
			r.Err = err
			results = append(results, r)
			continue
		}
		r.Release = rel

		ver, t, found := DetectInstalled(app)
		r.Installed = ver
		r.InstalledType = t
		r.Status = Evaluate(ver, t, found, sysType, rel.TagName)

		cfg.Apps[i].Latest = rel.TagName
		cfg.Apps[i].LastChecked = now
		if r.Status != NotInstalled && r.Status != Unmanaged {
			cfg.Apps[i].Version = ver
		}

		results = append(results, r)
	}
	return results
}

// Install downloads and installs the release in r, then records the newly
// installed version in cfg. The caller is responsible for saving cfg.
func Install(cfg *config.Config, r Result) error {
	if err := installer.InstallUpdate(r.Release); err != nil {
		return err
	}
	if ver, _, ok := DetectInstalled(r.App); ok {
		cfg.Apps[r.Index].Version = ver
	}
	return nil
}
