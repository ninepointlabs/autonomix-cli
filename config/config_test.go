package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripVersion(t *testing.T) {
	cases := map[string]string{
		"Repokase 0.1.0":      "Repokase",
		"Jubako 0.4.5":        "Jubako",
		"Tool v1.2.0":         "Tool",
		"v1.2.0 - Tool":       "Tool",
		"Tool (1.0.0-beta.1)": "Tool",
		"Autonomix CLI":       "Autonomix CLI",
		"omaforge":            "omaforge",
		"v0.3.2":              "",
		"":                    "",
	}
	for in, want := range cases {
		if got := StripVersion(in); got != want {
			t.Errorf("StripVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPackageFieldIsOptional(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := GetConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// A config written before the package field existed.
	old := `{"apps":[{"name":"ideas","repo_url":"https://github.com/ninepointlabs/ideas","version":"","latest":"v0.1.1","last_checked":""}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Apps[0].Package; got != "" {
		t.Errorf("Package = %q, want empty", got)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"package"`) {
		t.Errorf("saved an empty package field:\n%s", data)
	}

	cfg.Apps[0].Package = "idea"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil || cfg.Apps[0].Package != "idea" {
		t.Errorf("Package after round trip = %q (err %v), want idea", cfg.Apps[0].Package, err)
	}
}

func TestLoadCorruptConfigNamesFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := GetConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"apps": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Load()
	if err == nil {
		t.Fatal("Load() of a corrupt config returned no error")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Errorf("error %q should name %s and the parse problem", err, path)
	}
}
