package updater

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
)

// stubInstall replaces downloading and installing for a test. Downloads of
// the tags in failDownload fail, the others write a file into the staging
// directory. installErr is what the batch install returns.
func stubInstall(t *testing.T, failDownload map[string]bool, installErr error) (calls *[][]string) {
	t.Helper()
	calls = new([][]string)
	oldDownload, oldInstall := download, installPackages
	t.Cleanup(func() { download, installPackages = oldDownload, oldInstall })

	download = func(rel *github.Release, dir string) (string, error) {
		if failDownload[rel.TagName] {
			return "", errors.New("failed to download: bad status: 404 Not Found")
		}
		path := filepath.Join(dir, rel.TagName+".pkg.tar.zst")
		return path, os.WriteFile(path, nil, 0o644)
	}
	installPackages = func(paths []string) error {
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("package %s isn't staged: %v", p, err)
			}
		}
		*calls = append(*calls, paths)
		return installErr
	}
	return calls
}

func pendingUpdates(n int) (*config.Config, []Result) {
	cfg := &config.Config{}
	var results []Result
	for i := 0; i < n; i++ {
		app := config.App{Name: "autonomix-test-app-" + string(rune('a'+i)), RepoURL: "https://github.com/example/app"}
		cfg.Apps = append(cfg.Apps, app)
		results = append(results, Result{
			Index:   i,
			App:     app,
			Release: &github.Release{TagName: "v" + string(rune('a'+i))},
			Status:  UpdateAvailable,
		})
	}
	return cfg, results
}

func TestInstallAllInstallsOnce(t *testing.T) {
	calls := stubInstall(t, nil, nil)
	cfg, results := pendingUpdates(3)

	errs := InstallAll(cfg, results)
	for i, err := range errs {
		if err != nil {
			t.Errorf("result %d: unexpected error %v", i, err)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("installed %d times, want once", len(*calls))
	}
	if got := len((*calls)[0]); got != 3 {
		t.Errorf("installed %d packages, want 3", got)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Dir((*calls)[0][0]))); !os.IsNotExist(err) {
		t.Errorf("staging directory wasn't removed: %v", err)
	}
}

func TestInstallAllSkipsFailedDownloads(t *testing.T) {
	calls := stubInstall(t, map[string]bool{"vb": true}, nil)
	cfg, results := pendingUpdates(3)

	errs := InstallAll(cfg, results)
	if errs[0] != nil || errs[2] != nil {
		t.Errorf("errs = %v, want only the second to fail", errs)
	}
	if errs[1] == nil || !strings.Contains(errs[1].Error(), "download") {
		t.Errorf("errs[1] = %v, want the download error", errs[1])
	}
	if len(*calls) != 1 || len((*calls)[0]) != 2 {
		t.Errorf("install calls = %q, want one call with 2 packages", *calls)
	}
}

func TestInstallAllBatchFailure(t *testing.T) {
	calls := stubInstall(t, map[string]bool{"va": true}, errors.New("authentication cancelled"))
	cfg, results := pendingUpdates(3)

	errs := InstallAll(cfg, results)
	if len(*calls) != 1 {
		t.Fatalf("installed %d times, want once (no per-package retries)", len(*calls))
	}
	if errs[0] == nil || !strings.Contains(errs[0].Error(), "download") {
		t.Errorf("errs[0] = %v, want the download error", errs[0])
	}
	for _, i := range []int{1, 2} {
		want := "installing 2 updates together failed: authentication cancelled"
		if errs[i] == nil || errs[i].Error() != want {
			t.Errorf("errs[%d] = %v, want %q", i, errs[i], want)
		}
	}
}

func TestInstallSingle(t *testing.T) {
	calls := stubInstall(t, nil, errors.New("exit status 1: error: target not found"))
	cfg, results := pendingUpdates(1)

	err := Install(cfg, results[0])
	if err == nil || err.Error() != "exit status 1: error: target not found" {
		t.Errorf("Install() = %v, want the installer's error unchanged", err)
	}
	if len(*calls) != 1 || len((*calls)[0]) != 1 {
		t.Errorf("install calls = %q, want one call with 1 package", *calls)
	}
}
