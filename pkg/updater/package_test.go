package updater

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
)

// stubInstalled makes only the packages in installed (name -> version) look
// installed, and records the names looked up.
func stubInstalled(t *testing.T, installed map[string]string) (lookups *[]string) {
	t.Helper()
	lookups = new([]string)
	old := checkInstalled
	t.Cleanup(func() { checkInstalled = old })
	checkInstalled = func(name string) (string, packages.Type, bool) {
		*lookups = append(*lookups, name)
		if ver, ok := installed[name]; ok {
			return ver, packages.Pacman, true
		}
		return "", packages.Unknown, false
	}
	return lookups
}

// ideas is tracked as repo "ideas", but its package is named "idea".
var ideas = config.App{Name: "ideas", RepoURL: "https://github.com/ninepointlabs/ideas"}

var ideasRelease = &github.Release{
	TagName: "v0.1.1",
	Assets: []github.Asset{
		{Name: "idea-0.1.1-1-x86_64.pkg.tar.zst"},
		{Name: "idea_0.1.1_amd64.deb"},
		{Name: "checksums.txt"},
	},
}

func TestDetectInstalledByPackage(t *testing.T) {
	stubInstalled(t, map[string]string{"idea": "0.1.1-1"})

	if _, _, found := DetectInstalled(ideas); found {
		t.Fatal("found ideas without a package name")
	}
	app := ideas
	app.Package = "idea"
	ver, _, found := DetectInstalled(app)
	if !found || ver != "0.1.1-1" {
		t.Errorf("DetectInstalled with package = %q, %v; want 0.1.1-1, true", ver, found)
	}
}

func TestDetectInstalledTriesPackageFirst(t *testing.T) {
	lookups := stubInstalled(t, map[string]string{"idea": "0.1.1-1", "ideas": "9.9"})
	app := ideas
	app.Package = "idea"
	if ver, _, _ := DetectInstalled(app); ver != "0.1.1-1" {
		t.Errorf("got version %q, want the package's 0.1.1-1", ver)
	}
	if (*lookups)[0] != "idea" {
		t.Errorf("first lookup = %q, want idea", (*lookups)[0])
	}
}

func TestDetectInstalledFromReleaseBackfillsPackage(t *testing.T) {
	stubInstalled(t, map[string]string{"idea": "0.1.1-1"})

	ver, _, found, pkg := DetectInstalledFromRelease(ideas, ideasRelease)
	if !found || ver != "0.1.1-1" || pkg != "idea" {
		t.Errorf("got %q, %v, %q; want 0.1.1-1, true, idea", ver, found, pkg)
	}
	if Evaluate(ver, packages.Pacman, found, packages.Pacman, ideasRelease.TagName) != UpToDate {
		t.Error("ideas 0.1.1-1 against v0.1.1 isn't up to date")
	}
}

func TestDetectInstalledFromReleaseNotInstalled(t *testing.T) {
	lookups := stubInstalled(t, nil)

	if _, _, found, pkg := DetectInstalledFromRelease(ideas, ideasRelease); found || pkg != "" {
		t.Errorf("got found=%v pkg=%q, want not found", found, pkg)
	}
	// "idea" is parsed from both the .pkg.tar.zst and the .deb; look it up once.
	n := 0
	for _, name := range *lookups {
		if name == "idea" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("looked up idea %d times, want 1 (lookups: %v)", n, *lookups)
	}
}

func TestDetectInstalledFromReleaseKeepsRecordedPackage(t *testing.T) {
	stubInstalled(t, map[string]string{"idea": "0.1.1-1"})
	app := ideas
	app.Package = "idea"
	if _, _, found, pkg := DetectInstalledFromRelease(app, ideasRelease); !found || pkg != "" {
		t.Errorf("got found=%v pkg=%q; want found, and no new package to record", found, pkg)
	}
}

func TestInstallAllRecordsPackageName(t *testing.T) {
	stubInstall(t, nil, nil)
	stubInstalled(t, map[string]string{"idea": "0.1.1-1"})
	oldName := packageName
	t.Cleanup(func() { packageName = oldName })
	packageName = func(path string) string {
		if !strings.HasSuffix(filepath.Base(path), ".pkg.tar.zst") {
			t.Errorf("unexpected package file %s", path)
		}
		return "idea"
	}

	cfg := &config.Config{Apps: []config.App{ideas}}
	r := Result{Index: 0, App: ideas, Release: ideasRelease, Status: NotInstalled}
	if err := Install(cfg, r); err != nil {
		t.Fatal(err)
	}
	got := cfg.Apps[0]
	if got.Package != "idea" || got.Version != "0.1.1-1" {
		t.Errorf("after install: package %q version %q; want idea, 0.1.1-1", got.Package, got.Version)
	}
}
