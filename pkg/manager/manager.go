package manager

import (
	"fmt"
	"strings"

	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
)

// AddResult contains the info about the added app
type AddResult struct {
	App     config.App
	Created bool // true if new, false if updated/existed
}

// AddApp handles the logic of adding a new repository to the configuration
func AddApp(cfg *config.Config, repoURL string) (*AddResult, error) {
	// Clean the URL to bare repo URL
	// E.g. https://github.com/owner/repo/releases -> https://github.com/owner/repo
	if strings.Contains(repoURL, "github.com") {
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) == 2 {
			pathParts := strings.Split(strings.Trim(parts[1], "/"), "/")
			if len(pathParts) >= 2 {
				repoURL = "https://github.com/" + pathParts[0] + "/" + pathParts[1]
			}
		}
	}

	// Check if already exists
	for _, app := range cfg.Apps {
		if strings.EqualFold(app.RepoURL, repoURL) {
			return &AddResult{App: app, Created: false}, fmt.Errorf("repository already tracked")
		}
	}

	rel, err := github.GetLatestRelease(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}

	// Determine a good name for the app
	parts := strings.Split(repoURL, "/")
	repoName := ""
	if len(parts) > 0 {
		repoName = parts[len(parts)-1]
	}

	appName := rel.Name
	if appName == "" || strings.HasPrefix(appName, "v") || strings.Contains(strings.ToLower(appName), "release") {
		appName = repoName
	}

	newApp := config.App{
		Name:    appName,
		RepoURL: repoURL,
		Latest:  rel.TagName,
	}

	// Check if installed locally
	if ver, _, installed := system.CheckInstalled(newApp.Name); installed {
		newApp.Version = ver
	} else if repoName != "" && repoName != newApp.Name {
		if ver, _, installed := system.CheckInstalled(repoName); installed {
			newApp.Version = ver
		}
	}

	cfg.Apps = append(cfg.Apps, newApp)
	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return &AddResult{App: newApp, Created: true}, nil
}
