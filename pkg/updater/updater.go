package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	// Unchecked means the app is installed but no release has been fetched
	// for it yet, so whether it is current is unknown.
	Unchecked
)

var statusNames = map[Status]string{
	UpToDate:        "up_to_date",
	UpdateAvailable: "update_available",
	NotInstalled:    "not_installed",
	Unmanaged:       "unmanaged",
	Failed:          "failed",
	Unchecked:       "unchecked",
}

func (s Status) String() string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return "unknown"
}

// MarshalText lets a Status appear in JSON output by name.
func (s Status) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

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

// CompareVersions compares two normalized versions the way pacman's vercmp
// does, returning -1, 0 or 1. Versions are split into runs of digits and
// letters: digit runs compare numerically, letter runs alphabetically, and a
// digit run is newer than a letter run. When one version is a prefix of the
// other, a trailing letter run marks a pre-release (1.0rc1 < 1.0) and a
// trailing digit run a later release (1.0 < 1.0.1).
func CompareVersions(a, b string) int {
	as, bs := versionSegments(a), versionSegments(b)
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, y := as[i], bs[i]
		xNum, yNum := isNumeric(x), isNumeric(y)
		switch {
		case xNum && yNum:
			x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
			if len(x) != len(y) {
				return sign(len(x) - len(y))
			}
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		case xNum:
			return 1
		case yNum:
			return -1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(as) > len(bs):
		return trailing(as[len(bs)])
	case len(bs) > len(as):
		return -trailing(bs[len(as)])
	}
	return 0
}

// trailing is the result for the version that has the extra segment s.
func trailing(s string) int {
	if isNumeric(s) {
		return 1
	}
	return -1
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// versionSegments splits "1.10rc2" into ["1", "10", "rc", "2"].
func versionSegments(v string) []string {
	var segs []string
	start := -1
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	isAlpha := func(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
	v = strings.ToLower(v)
	for i := 0; i <= len(v); i++ {
		if start >= 0 && (i == len(v) || isDigit(v[i]) != isDigit(v[start]) || !(isDigit(v[i]) || isAlpha(v[i]))) {
			segs = append(segs, v[start:i])
			start = -1
		}
		if i < len(v) && start < 0 && (isDigit(v[i]) || isAlpha(v[i])) {
			start = i
		}
	}
	return segs
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
	if latest := NormalizeVersion(latestTag); latest != "" && CompareVersions(latest, NormalizeVersion(installed)) > 0 {
		return UpdateAvailable
	}
	return UpToDate
}

// Local evaluates every tracked app against the latest release recorded in
// cfg, without contacting GitHub.
func Local(cfg *config.Config) []Result {
	sysType := system.GetSystemPreferredType()

	results := make([]Result, 0, len(cfg.Apps))
	for i, app := range cfg.Apps {
		r := Result{Index: i, App: app}
		ver, t, found := DetectInstalled(app)
		r.Installed = ver
		r.InstalledType = t
		r.Status = Evaluate(ver, t, found, sysType, app.Latest)
		if app.Latest == "" && r.Status == UpToDate {
			r.Status = Unchecked
		}
		results = append(results, r)
	}
	return results
}

// Check fetches the latest release for the tracked apps at indexes, or for
// every tracked app when none are given, and records the results in cfg.
// The caller is responsible for saving cfg.
func Check(cfg *config.Config, indexes ...int) []Result {
	sysType := system.GetSystemPreferredType()
	now := time.Now().Format(time.RFC3339)

	if len(indexes) == 0 {
		for i := range cfg.Apps {
			indexes = append(indexes, i)
		}
	}

	results := make([]Result, 0, len(indexes))
	for _, i := range indexes {
		app := cfg.Apps[i]
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
		r.App = cfg.Apps[i]

		results = append(results, r)
	}
	return results
}

// download and installPackages are replaced in tests.
var (
	download        = installer.DownloadUpdate
	installPackages = installer.InstallPackages
)

// Install downloads and installs the release in r, then records the newly
// installed version in cfg. The caller is responsible for saving cfg.
func Install(cfg *config.Config, r Result) error {
	return InstallAll(cfg, []Result{r})[0]
}

// InstallAll downloads the release in each result, then installs them all
// with one elevated command so the password is asked for once. It returns an
// error for each result, nil when it was installed, and records the newly
// installed versions in cfg. The caller is responsible for saving cfg.
func InstallAll(cfg *config.Config, results []Result) []error {
	errs := make([]error, len(results))
	dir, err := os.MkdirTemp("", "autonomix-")
	if err != nil {
		for i := range errs {
			errs[i] = err
		}
		return errs
	}
	defer os.RemoveAll(dir)

	var paths []string
	var staged []int
	for i, r := range results {
		// One directory per app, so assets with the same name don't collide.
		appDir := filepath.Join(dir, strconv.Itoa(i))
		if err := os.Mkdir(appDir, 0o755); err != nil {
			errs[i] = err
			continue
		}
		path, err := download(r.Release, appDir)
		if err != nil {
			errs[i] = err
			continue
		}
		paths = append(paths, path)
		staged = append(staged, i)
	}
	if len(paths) == 0 {
		return errs
	}

	if err := installPackages(paths); err != nil {
		if len(paths) > 1 {
			err = fmt.Errorf("installing %d updates together failed: %w", len(paths), err)
		}
		for _, i := range staged {
			errs[i] = err
		}
		return errs
	}
	for _, i := range staged {
		if ver, _, ok := DetectInstalled(results[i].App); ok {
			cfg.Apps[results[i].Index].Version = ver
		}
	}
	return errs
}
