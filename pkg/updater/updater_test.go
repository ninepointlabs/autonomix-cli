package updater

import (
	"testing"

	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
)

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v0.3.1":      "0.3.1",
		"0.3.1-1":     "0.3.1",
		" 1.2.0 ":     "1.2.0",
		"1.0.0-1.el9": "1.0.0",
		"1.0.0-beta":  "1.0.0-beta",
		"":            "",
	}
	for in, want := range cases {
		if got := NormalizeVersion(in); got != want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		typ       packages.Type
		found     bool
		latest    string
		want      Status
	}{
		{"not installed", "", packages.Unknown, false, "v1.0.0", NotInstalled},
		{"binary on path", "tool version 1.0.0", packages.Unknown, true, "v1.1.0", Unmanaged},
		{"other package manager", "1.0.0", packages.Flatpak, true, "v1.1.0", Unmanaged},
		{"up to date", "1.0.0-1", packages.Pacman, true, "v1.0.0", UpToDate},
		{"update available", "1.0.0-1", packages.Pacman, true, "v1.1.0", UpdateAvailable},
		{"no release tag", "1.0.0-1", packages.Pacman, true, "", UpToDate},
	}
	for _, tt := range tests {
		got := Evaluate(tt.installed, tt.typ, tt.found, packages.Pacman, tt.latest)
		if got != tt.want {
			t.Errorf("%s: Evaluate() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
