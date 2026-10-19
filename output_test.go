package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/updater"
)

// jsonKeys returns the sorted keys v has as a JSON object.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The omarchy bar plugin reads this output, so its fields must not change.
func TestAppJSONFields(t *testing.T) {
	v := appJSON{Updated: true, Error: "x"}
	want := []string{"error", "installed", "installed_type", "last_checked", "latest", "name",
		"release_url", "repo", "repo_url", "status", "updated"}
	if got := jsonKeys(t, v); !reflect.DeepEqual(got, want) {
		t.Errorf("appJSON keys = %q, want %q", got, want)
	}
}

func pendingResult() updater.Result {
	return updater.Result{
		App:           config.App{Name: "app", RepoURL: "https://github.com/example/app", Latest: "v1.1.0"},
		Installed:     "1.0.0-1",
		InstalledType: packages.Pacman,
		Release:       &github.Release{TagName: "v1.1.0"},
		Status:        updater.UpdateAvailable,
	}
}

func TestInstalledViewFailure(t *testing.T) {
	r := pendingResult()
	v := installedView(r, r, errors.New("installing 2 updates together failed: authentication cancelled"))

	if v.Updated {
		t.Error("failed install reported as updated")
	}
	if v.Error != "installing 2 updates together failed: authentication cancelled" {
		t.Errorf("Error = %q", v.Error)
	}
	if v.Status != updater.UpdateAvailable || v.Installed != "1.0.0-1" || v.Latest != "v1.1.0" {
		t.Errorf("failed install view = %+v, want the app as it was before", v)
	}
}

func TestInstalledViewSuccess(t *testing.T) {
	r := pendingResult()
	after := r
	after.Release = nil
	after.Installed = "1.1.0-1"
	after.Status = updater.UpToDate

	v := installedView(r, after, nil)
	if !v.Updated || v.Error != "" {
		t.Errorf("view = %+v, want updated without error", v)
	}
	if v.Status != updater.UpToDate || v.Installed != "1.1.0-1" {
		t.Errorf("view = %+v, want the app as found after installing", v)
	}
	if v.ReleaseURL != "https://github.com/example/app/releases/tag/v1.1.0" {
		t.Errorf("ReleaseURL = %q", v.ReleaseURL)
	}
}
