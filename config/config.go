package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// versionToken matches a version number like "0.4.5", "v1.2" or "1.0.0-beta.1".
var versionToken = regexp.MustCompile(`(?i)\bv?\d+(\.\d+)+(-[0-9a-z.]+)?\b`)

// StripVersion removes version numbers from a display name, since release
// titles like "Repokase 0.1.0" go stale after the first update.
// e.g. "Jubako 0.4.5" -> "Jubako", "v1.2.0 - Tool" -> "Tool"
func StripVersion(name string) string {
	name = versionToken.ReplaceAllString(name, "")
	name = strings.Join(strings.Fields(name), " ")
	return strings.Trim(name, " -_:()[]")
}

type App struct {
	Name        string `json:"name"`
	RepoURL     string `json:"repo_url"`
	Version     string `json:"version"` // Installed version
	Latest      string `json:"latest"`  // Latest version detected
	LastChecked string `json:"last_checked"`
	// Package is the name the package manager knows the app by, read from
	// the installed package, when it differs from Name and the repo name.
	Package string `json:"package,omitempty"`
}

type Config struct {
	Apps []App `json:"apps"`
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".autonomix"), nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{Apps: []App{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Clean up names saved with a version in them by older releases
	for i, app := range cfg.Apps {
		if cleaned := StripVersion(app.Name); cleaned != "" {
			cfg.Apps[i].Name = cleaned
		}
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
