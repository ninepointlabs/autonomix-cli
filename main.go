package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/about"
	"github.com/ninepointlabs/autonomix-cli/pkg/installer"
	"github.com/ninepointlabs/autonomix-cli/pkg/manager"
	"github.com/ninepointlabs/autonomix-cli/pkg/updater"
	"github.com/ninepointlabs/autonomix-cli/tui"
)

const SelfRepoURL = "https://github.com/ninepointlabs/autonomix-cli"

var version = "dev" // Set by goreleaser

//go:embed LICENSE
var licenseText string

func aboutInfo() about.Info {
	return about.Info{Version: version, LicenseText: licenseText}
}

const usage = `Usage:
  autonomix-cli                     Open the TUI
  autonomix-cli add <url>           Track a GitHub repository
  autonomix-cli <url>               Same as add
  autonomix-cli list                List tracked apps (no network)
  autonomix-cli check [app...]      Check tracked apps for updates
  autonomix-cli update [-y] [app...]
                                    Install available updates (-y: don't ask)
  autonomix-cli install <app>       Install a tracked app that isn't installed yet
  autonomix-cli remove <app>        Stop tracking an app (does not uninstall it)
  autonomix-cli about               Show version, license, support and privacy
  autonomix-cli --version           Print version
  autonomix-cli --help              Show this help

  <app> is an app name, repo name, owner/repo or repository URL.

Options:
  --json    Print machine-readable JSON on stdout (progress goes to stderr).
            Implies -y for update.
  -y        Don't ask before installing updates.
`

// opts holds the flags shared by every command.
type opts struct {
	json      bool
	assumeYes bool
}

func main() {
	if len(os.Args) < 2 {
		cfg, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		p := tea.NewProgram(tui.NewModel(cfg, aboutInfo()), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
		return
	}

	var o opts
	var args []string
	for _, a := range os.Args[1:] {
		switch a {
		case "--json":
			o.json = true
		case "-y", "--yes":
			o.assumeYes = true
		default:
			args = append(args, a)
		}
	}
	if o.json {
		o.assumeYes = true
		installer.Log = os.Stderr
	}
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-v", "--version", "version":
		fmt.Println(version)
	case "about":
		fmt.Print(aboutInfo().Text())
	case "add":
		if len(args) != 2 {
			os.Exit(argError(o, "add", "one repository URL"))
		}
		os.Exit(runAdd(o, args[1]))
	case "list":
		os.Exit(runList(o))
	case "check":
		os.Exit(runCheck(o, args[1:]))
	case "update":
		os.Exit(runUpdate(o, args[1:]))
	case "install":
		if len(args) != 2 {
			os.Exit(argError(o, "install", "one app"))
		}
		os.Exit(runInstall(o, args[1]))
	case "remove":
		if len(args) != 2 {
			os.Exit(argError(o, "remove", "one app"))
		}
		os.Exit(runRemove(o, args[1]))
	default:
		if strings.Contains(args[0], "github.com/") {
			os.Exit(runAdd(o, args[0]))
		}
		fmt.Printf("Unknown command: %s\n\n%s", args[0], usage)
		os.Exit(1)
	}
}

// argError reports a command given the wrong number of arguments.
func argError(o opts, cmd, want string) int {
	return fail(o, "Error: %s needs %s. Run 'autonomix-cli --help' for usage.", cmd, want)
}

// loadConfig loads the config and ensures autonomix-cli itself is tracked.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
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
	return cfg, config.Save(cfg)
}

// mustLoadConfig loads the config or reports the error and exits.
func mustLoadConfig(o opts) *config.Config {
	cfg, err := loadConfig()
	if err != nil {
		os.Exit(fail(o, "Error loading config: %v", err))
	}
	return cfg
}

// findApps resolves app queries to config indexes, or reports the first
// that doesn't match and exits.
func findApps(o opts, cfg *config.Config, queries []string) []int {
	var indexes []int
	for _, q := range queries {
		i, err := manager.FindApp(cfg, q)
		if err != nil {
			os.Exit(fail(o, "%v", err))
		}
		indexes = append(indexes, i)
	}
	return indexes
}

func saveConfig(o opts, cfg *config.Config) {
	if err := config.Save(cfg); err != nil {
		logf(o, "Error saving config: %v\n", err)
	}
}

func runAdd(o opts, url string) int {
	cfg := mustLoadConfig(o)

	logf(o, "Adding repository: %s...\n", url)
	res, err := manager.AddApp(cfg, url)
	if err != nil {
		return fail(o, "Error adding app: %v", err)
	}

	if o.json {
		results := updater.Local(cfg)
		return printJSON(appView(results[len(results)-1]))
	}
	fmt.Printf("Successfully added %s (Latest: %s)\n", res.App.Name, res.App.Latest)
	return 0
}

func runList(o opts) int {
	cfg := mustLoadConfig(o)
	results := updater.Local(cfg)

	if o.json {
		return printJSON(appViews(results))
	}
	for _, r := range results {
		printResult(r)
	}
	return 0
}

// checkApps checks the tracked apps at indexes (all when empty), prints a
// report unless printing JSON, and returns the results.
func checkApps(o opts, cfg *config.Config, indexes []int) []updater.Result {
	logf(o, "Checking Autonomix apps for updates...\n")
	results := updater.Check(cfg, indexes...)
	saveConfig(o, cfg)

	if !o.json {
		for _, r := range results {
			printResult(r)
		}
	}
	return results
}

func printResult(r updater.Result) {
	switch r.Status {
	case updater.UpToDate:
		fmt.Printf("  %s: up to date (%s)\n", r.App.Name, r.Installed)
	case updater.UpdateAvailable:
		fmt.Printf("  %s: update available %s -> %s\n", r.App.Name, r.Installed, latestTag(r))
	case updater.NotInstalled:
		fmt.Printf("  %s: not installed\n", r.App.Name)
	case updater.Unmanaged:
		fmt.Printf("  %s: installed outside the system package manager\n", r.App.Name)
	case updater.Unchecked:
		fmt.Printf("  %s: installed (%s), not checked yet\n", r.App.Name, r.Installed)
	case updater.Failed:
		fmt.Printf("  %s: check failed: %v\n", r.App.Name, r.Err)
	}
}

func runCheck(o opts, queries []string) int {
	cfg := mustLoadConfig(o)
	results := checkApps(o, cfg, findApps(o, cfg, queries))

	code := 0
	for _, r := range results {
		if r.Status == updater.Failed {
			code = 1
		}
	}
	if o.json {
		if c := printJSON(appViews(results)); c != 0 {
			return c
		}
	}
	return code
}

func runUpdate(o opts, queries []string) int {
	cfg := mustLoadConfig(o)
	results := checkApps(o, cfg, findApps(o, cfg, queries))

	code := 0
	updated := 0
	reader := bufio.NewReader(os.Stdin)
	views := make([]appJSON, 0, len(results))
	var pending []updater.Result
	var slots []int // index in views of each pending update
	for _, r := range results {
		if r.Status == updater.Failed {
			code = 1
		}
		if r.Status != updater.UpdateAvailable {
			views = append(views, appView(r))
			continue
		}

		if !o.assumeYes {
			fmt.Printf("Update %s %s -> %s? [y/N] ", r.App.Name, r.Installed, r.Release.TagName)
			answer, _ := reader.ReadString('\n')
			if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
				views = append(views, appView(r))
				continue
			}
		}

		// Filled in below, once every accepted update is installed together.
		slots = append(slots, len(views))
		views = append(views, appJSON{})
		pending = append(pending, r)
	}

	for i, v := range installAll(o, cfg, pending) {
		views[slots[i]] = v
		if !v.Updated {
			code = 1
			continue
		}
		updated++
	}

	if o.json {
		if c := printJSON(views); c != 0 {
			return c
		}
	} else if updated == 0 && code == 0 {
		fmt.Println("Nothing to update.")
	}
	return code
}

func runInstall(o opts, query string) int {
	cfg := mustLoadConfig(o)
	r := checkApps(o, cfg, findApps(o, cfg, []string{query}))[0]

	switch r.Status {
	case updater.Failed:
		return fail(o, "Failed to check %s: %v", r.App.Name, r.Err)
	case updater.Unmanaged:
		return fail(o, "%s is already installed outside the system package manager", r.App.Name)
	case updater.UpToDate, updater.UpdateAvailable:
		return fail(o, "%s is already installed, use update", r.App.Name)
	}

	v := installAll(o, cfg, []updater.Result{r})[0]
	if o.json {
		if c := printJSON(v); c != 0 {
			return c
		}
	}
	if !v.Updated {
		return 1
	}
	return 0
}

// installAll installs the releases in results with one elevated command, so
// the password is asked for once, and returns each app's state afterwards.
func installAll(o opts, cfg *config.Config, results []updater.Result) []appJSON {
	if len(results) == 0 {
		return nil
	}
	for _, r := range results {
		logf(o, "Installing %s %s...\n", r.App.Name, r.Release.TagName)
	}
	errs := updater.InstallAll(cfg, results)
	saveConfig(o, cfg)

	local := updater.Local(cfg)
	views := make([]appJSON, 0, len(results))
	for i, r := range results {
		if errs[i] != nil {
			logf(o, "Failed to install %s: %v\n", r.App.Name, errs[i])
		}
		views = append(views, installedView(r, local[r.Index], errs[i]))
	}
	return views
}

// installedView is the app's state after installing the release in r, where
// after is the app as found afterwards and err the install error.
func installedView(r, after updater.Result, err error) appJSON {
	if err != nil {
		v := appView(r)
		v.Error = err.Error()
		return v
	}
	after.Release = r.Release
	v := appView(after)
	v.Updated = true
	return v
}

func runRemove(o opts, query string) int {
	cfg := mustLoadConfig(o)
	i := findApps(o, cfg, []string{query})[0]
	if cfg.Apps[i].RepoURL == SelfRepoURL {
		return fail(o, "Autonomix CLI always tracks itself and can't be removed")
	}

	app, err := manager.RemoveApp(cfg, i)
	if err != nil {
		return fail(o, "Error removing %s: %v", app.Name, err)
	}
	if o.json {
		return printJSON(map[string]string{"removed": app.Name, "repo_url": app.RepoURL})
	}
	fmt.Printf("Stopped tracking %s\n", app.Name)
	return 0
}
