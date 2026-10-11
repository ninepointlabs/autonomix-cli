package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/manager"
	"github.com/ninepointlabs/autonomix-cli/pkg/updater"
	"github.com/ninepointlabs/autonomix-cli/tui"
)

const SelfRepoURL = "https://github.com/ninepointlabs/autonomix-cli"

var version = "dev" // Set by goreleaser

const usage = `Usage:
  autonomix-cli                 Open the TUI
  autonomix-cli add <url>       Track a GitHub repository
  autonomix-cli <url>           Same as add
  autonomix-cli check           List tracked apps with available updates
  autonomix-cli update [-y]     Install available updates (-y: don't ask)
  autonomix-cli --version       Print version
  autonomix-cli --help          Show this help
`

func main() {
	if len(os.Args) > 1 {
		args := os.Args[1:]
		switch args[0] {
		case "-h", "--help", "help":
			fmt.Print(usage)
			return
		case "-v", "--version", "version":
			fmt.Println(version)
			return
		case "add":
			if len(args) < 2 {
				fmt.Print(usage)
				os.Exit(1)
			}
			addApp(args[1])
			return
		case "check":
			os.Exit(runCheck())
		case "update":
			assumeYes := len(args) > 1 && (args[1] == "-y" || args[1] == "--yes")
			os.Exit(runUpdate(assumeYes))
		default:
			if strings.Contains(args[0], "github.com/") {
				addApp(args[0])
				return
			}
			fmt.Printf("Unknown command: %s\n\n%s", args[0], usage)
			os.Exit(1)
		}
	}

	cfg := loadConfig()

	p := tea.NewProgram(tui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

// loadConfig loads the config and ensures autonomix-cli itself is tracked.
func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Ensure self is tracked and version is up to date
	tracked := false
	for i, app := range cfg.Apps {
		if app.RepoURL == SelfRepoURL {
			cfg.Apps[i].Version = version
			tracked = true
			break
		}
	}
	if !tracked {
		cfg.Apps = append(cfg.Apps, config.App{
			Name:    "Autonomix CLI",
			RepoURL: SelfRepoURL,
			Version: version,
		})
	}
	// Always save to ensure version is updated
	config.Save(cfg)
	return cfg
}

func addApp(url string) {
	cfg := loadConfig()

	fmt.Printf("Adding repository: %s...\n", url)
	res, err := manager.AddApp(cfg, url)
	if err != nil {
		fmt.Printf("Error adding app: %v\n", err)
		os.Exit(1)
	}

	if res.Created {
		fmt.Printf("Successfully added %s (Latest: %s)\n", res.App.Name, res.App.Latest)
	} else {
		fmt.Printf("Repository %s is already tracked.\n", res.App.Name)
	}
}

// checkApps checks all tracked apps, prints a report and returns the results.
func checkApps(cfg *config.Config) []updater.Result {
	results := updater.Check(cfg)
	if err := config.Save(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
	}

	for _, r := range results {
		switch r.Status {
		case updater.UpToDate:
			fmt.Printf("  %s: up to date (%s)\n", r.App.Name, r.Installed)
		case updater.UpdateAvailable:
			fmt.Printf("  %s: update available %s -> %s\n", r.App.Name, r.Installed, r.Release.TagName)
		case updater.NotInstalled:
			fmt.Printf("  %s: not installed, skipping\n", r.App.Name)
		case updater.Unmanaged:
			fmt.Printf("  %s: installed outside the system package manager, skipping\n", r.App.Name)
		case updater.Failed:
			fmt.Printf("  %s: check failed: %v\n", r.App.Name, r.Err)
		}
	}
	return results
}

func runCheck() int {
	cfg := loadConfig()
	fmt.Println("Checking Autonomix apps for updates...")

	code := 0
	for _, r := range checkApps(cfg) {
		if r.Status == updater.Failed {
			code = 1
		}
	}
	return code
}

func runUpdate(assumeYes bool) int {
	cfg := loadConfig()
	fmt.Println("Checking Autonomix apps for updates...")

	code := 0
	updated := 0
	reader := bufio.NewReader(os.Stdin)
	for _, r := range checkApps(cfg) {
		if r.Status == updater.Failed {
			code = 1
		}
		if r.Status != updater.UpdateAvailable {
			continue
		}

		if !assumeYes {
			fmt.Printf("Update %s %s -> %s? [y/N] ", r.App.Name, r.Installed, r.Release.TagName)
			answer, _ := reader.ReadString('\n')
			if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
				continue
			}
		}

		fmt.Printf("Updating %s to %s...\n", r.App.Name, r.Release.TagName)
		if err := updater.Install(cfg, r); err != nil {
			fmt.Printf("Failed to update %s: %v\n", r.App.Name, err)
			code = 1
			continue
		}
		updated++
		if err := config.Save(cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
		}
	}

	if updated == 0 && code == 0 {
		fmt.Println("Nothing to update.")
	}
	return code
}
