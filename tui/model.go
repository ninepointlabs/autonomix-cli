package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ninepointlabs/autonomix-cli/config"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/installer"
	"github.com/ninepointlabs/autonomix-cli/pkg/manager"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
)

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	// Remove "v" prefix
	v = strings.TrimPrefix(v, "v")
	// Remove Debian/RPM package revision suffix (e.g., "0.1.1-1" -> "0.1.1")
	if idx := strings.LastIndex(v, "-"); idx > 0 {
		// Only strip if what follows the dash looks like a package revision (number)
		suffix := v[idx+1:]
		// Check if suffix is purely numeric (Debian revision) or contains "el" (RPM dist tag)
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

var (
	docStyle         = lipgloss.NewStyle().Margin(1, 2)
	statusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	installedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green
	updateStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
	notInstalledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")) // Grey
)

type state int

const (
	viewList state = iota
	viewAdd
	viewSelectAsset
)

// Define self repo URL matching main.go to identify it
const SelfRepoURL = "https://github.com/ninepointlabs/autonomix-cli"

type item struct {
	app config.App
}

func (i item) Title() string       { return i.app.Name }
func (i item) Description() string {
	status := "Not Installed"
	style := notInstalledStyle
	
	if i.app.Version != "" {
		status = "Installed: " + i.app.Version
		style = installedStyle
		
		vInstalled := normalizeVersion(i.app.Version)
		vLatest := normalizeVersion(i.app.Latest)
		
		if vLatest != "" && vLatest != vInstalled {
			status = fmt.Sprintf("Update Available: %s -> %s", i.app.Version, i.app.Latest)
			style = updateStyle
		}
	}
	
	return fmt.Sprintf("%s (%s)", i.app.RepoURL, style.Render(status))
}
func (i item) FilterValue() string { return i.app.Name }

type Model struct {
	list      list.Model
	input     textinput.Model
	state     state
	config    *config.Config
	quitting  bool
	err       error
	status    string
	
	// Selection for install
	assetList list.Model
	selectedApp *config.App
}

// openBrowser opens the specified URL in the default browser of the user.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func NewModel(cfg *config.Config) Model {
	items := []list.Item{}
	for _, app := range cfg.Apps {
		items = append(items, item{app: app})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Autonomix Apps"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "check updates")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "install/open")),
		}
	}

	
	assetsL := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	assetsL.Title = "Select Package to Install"
	assetsL.SetShowHelp(false)

	ti := textinput.New()
	ti.Placeholder = "https://github.com/owner/repo"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return Model{
		list:      l,
		input:     ti,
		state:     viewList,
		config:    cfg,
		assetList: assetsL,
	}
}

func (m Model) Init() tea.Cmd {
	// Check for updates for all tracked apps on startup
	var cmds []tea.Cmd
	for i, app := range m.config.Apps {
		cmds = append(cmds, checkUpdateCmd(app, i))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == viewSelectAsset {
			switch msg.String() {
			case "enter":
				// Selected asset
				if index := m.assetList.Index(); index >= 0 && index < len(m.assetList.Items()) {
					selectedAsset := m.assetList.Items()[index].(assetItem).asset
					m.status = fmt.Sprintf("Downloading %s...", selectedAsset.Name)
					m.state = viewList // go back to main view while installing
					return m, downloadAssetCmd(&selectedAsset)
				}
			case "esc", "q":
				m.state = viewList
				m.selectedApp = nil
				return m, nil
			}
			var cmd tea.Cmd
			m.assetList, cmd = m.assetList.Update(msg)
			return m, cmd
		}

		if m.state == viewAdd {
			switch msg.Type {
			case tea.KeyEnter:
				url := m.input.Value()
				if url != "" {
					// Optimistically clear input
					m.input.Reset()
					return m, checkRepoArgCmd(url)
				}
				m.state = viewList
				m.input.Reset()
				return m, nil
			case tea.KeyEsc:
				m.state = viewList
				m.input.Reset()
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		if m.state == viewList {
			// Clear error if any key pressed
			if m.err != nil {
				m.err = nil
				return m, nil
			}

			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				// Open release page OR Install Update
				if index := m.list.Index(); index >= 0 && index < len(m.list.Items()) {
					selectedItem := m.list.Items()[index].(item)
					
					// Check if update available or not installed
					vInstalled := normalizeVersion(selectedItem.app.Version)
					vLatest := normalizeVersion(selectedItem.app.Latest)
					
					// Install if not installed OR update available
					// Note: vLatest check ensures we actually found a release on GitHub
					if vLatest != "" && (vInstalled == "" || vLatest != vInstalled) {
						// Trigger install/update
						action := "update"
						if vInstalled == "" {
							action = "install"
						}
						m.status = fmt.Sprintf("Fetching assets for %s...", action)
						return m, fetchAssetsCmd(selectedItem.app)
					}
					
					// Fallback to opening browser
					url := selectedItem.app.RepoURL
					// Try to be smart, if we have a tag, go to that release tag
					if selectedItem.app.Latest != "" {
						url = fmt.Sprintf("%s/releases/tag/%s", strings.TrimSuffix(url, ".git"), selectedItem.app.Latest)
					}
					openBrowser(url)
					return m, nil
				}
			case "a":
				m.state = viewAdd
				m.input.Focus()
				return m, textinput.Blink
			case "d":
				if index := m.list.Index(); index >= 0 && index < len(m.list.Items()) {
					m.config.Apps = append(m.config.Apps[:index], m.config.Apps[index+1:]...)
					config.Save(m.config) // Save immediately for now
					m.list.RemoveItem(index)
				}
				return m, nil
			case "u":
				// Check for updates for the selected item
				if index := m.list.Index(); index >= 0 && index < len(m.list.Items()) {
					selectedItem := m.list.Items()[index].(item)
					return m, checkUpdateCmd(selectedItem.app, index)
				}
			}
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case assetsFetchedMsg:
		if msg.err != nil && len(msg.assets) == 0 {
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		
		if len(msg.assets) == 0 {
			m.err = fmt.Errorf("no compatible assets found for your system")
			m.status = ""
			return m, nil
		}
		
		// If there's an error but we have assets, it's a warning - show it as status
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = ""
		}
		
		items := []list.Item{}
		for _, a := range msg.assets {
			items = append(items, assetItem{asset: a})
		}
		m.assetList.SetItems(items)
		m.assetList.Title = fmt.Sprintf("Select Asset for %s", msg.app.Name)
		m.state = viewSelectAsset
		m.selectedApp = &msg.app
		// Update the app's Latest field in config now that we fetched it
		for idx, app := range m.config.Apps {
			if app.RepoURL == msg.app.RepoURL {
				m.config.Apps[idx].Latest = msg.app.Latest
				config.Save(m.config)
				break
			}
		}
		return m, nil

	case repoCheckedMsg:

		if msg.err != nil {
			m.err = msg.err
			m.state = viewList
			return m, nil // potentially show error
		}
		
		// Logic moved to manager, but here we need to insert into list
		// Re-load config or just use the data returned? 
		// The Msg now needs to return the added app, not just the release.
		
		m.config.Apps = append(m.config.Apps, msg.app)
		// Config is already saved by manager in the logic below (see checkRepoArgCmd)
		
		m.list.InsertItem(len(m.list.Items()), item{app: msg.app})
		m.state = viewList
		m.input.Reset()
		return m, nil

	case updateCheckedMsg:
		if msg.err != nil {
			// handle error, maybe statusbar
			return m, nil 
		}
		// update the item in the list
		idx := msg.index
		if idx >= 0 && idx < len(m.config.Apps) {
			m.config.Apps[idx].Latest = msg.release.TagName
			config.Save(m.config)
			// Update list item
			cmd = m.list.SetItem(idx, item{app: m.config.Apps[idx]})
			cmds = append(cmds, cmd)
		}

	case downloadedMsg:
		m.status = "Installing (enter password if prompted)..."
		// Prepare install command
		installCmd, err := installer.GetInstallCmd(msg.path)
		if err != nil {
			m.err = err
			m.status = ""
			os.Remove(msg.path) // Cleanup
			return m, nil
		}
		
		// Run interactive command
		cmd = tea.Exec(&execCmdAdapter{installCmd}, func(err error) tea.Msg {
			os.Remove(msg.path) // Cleanup after install
			return installFinishedMsg{err: err}
		})
		cmds = append(cmds, cmd)

	case installFinishedMsg:
		if msg.err != nil {
			m.status = ""
			m.err = fmt.Errorf("installation failed: %v", msg.err)
		} else {
			// Success! Re-check installed version and update config
			m.err = nil
			m.status = "Verifying installation..."
			if m.selectedApp != nil {
				return m, recheckInstalledWithDelayCmd(*m.selectedApp)
			}
		}
	
	case installedRecheckedMsg:
		// Update the app's version and latest in config and list
		m.status = ""
		for idx, app := range m.config.Apps {
			if app.RepoURL == msg.app.RepoURL {
				m.config.Apps[idx].Version = msg.version
				// Also update Latest to ensure we have the correct release tag
				if msg.latest != "" {
					m.config.Apps[idx].Latest = msg.latest
				}
				config.Save(m.config)
				// Update list item
				cmd = m.list.SetItem(idx, item{app: m.config.Apps[idx]})
				cmds = append(cmds, cmd)
				break
			}
		}
		m.selectedApp = nil
	}

	if m.state == viewList {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press any key to continue...", m.err)
	}

	if m.status != "" {
		return fmt.Sprintf("\n  %s\n", m.status)
	}

	if m.state == viewSelectAsset {
		return docStyle.Render(m.assetList.View())
	}

	if m.state == viewAdd {
		return fmt.Sprintf(
			"Enter GitHub Repo URL:\n\n%s\n\n(esc to cancel)\n",
			m.input.View(),
		)
	}
	return docStyle.Render(m.list.View())
}

// Commands and Messages

type repoCheckedMsg struct {
	app config.App
	err error
}

func checkRepoArgCmd(url string) tea.Cmd {
	return func() tea.Msg {
		// We need to load config first to pass to manager, 
		// but Model has it. Accessing Model from Cmd is hard.
		// However, we can just load it again or pass it...
		// Simplified: We actually just want the logic to run.
		// BUT: Manager saves the config. The Model has an in-memory copy.
		// We should probably NOT save in manager if called from TUI? 
		// Or we reload config in TUI?
		
		// Let's modify the flow.
		// The TUI should call the manager.
		
		cfg, err := config.Load() 
		if err != nil {
			return repoCheckedMsg{err: err}
		}
		
		res, err := manager.AddApp(cfg, url)
		if err != nil {
			return repoCheckedMsg{err: err}
		}
		
		return repoCheckedMsg{app: res.App, err: nil}
	}
}

type assetsFetchedMsg struct {
	assets  []github.Asset
	app     config.App
	release *github.Release
	err     error
}

func fetchAssetsCmd(app config.App) tea.Cmd {
	return func() tea.Msg {
		rel, err := github.GetLatestRelease(app.RepoURL)
		if err != nil {
			return assetsFetchedMsg{err: err}
		}
		
		assets, err := installer.GetCompatibleAssets(rel)
		if err != nil {
			// Try to get all assets as a fallback
			allAssets := installer.GetAllAssets(rel)
			if len(allAssets) > 0 {
				// Return all assets with a warning in the error
				app.Latest = rel.TagName
				return assetsFetchedMsg{
					assets: allAssets, 
					app: app, 
					release: rel, 
					err: fmt.Errorf("warning: %v. Showing all available assets", err),
				}
			}
			return assetsFetchedMsg{err: err}
		}
		
		// Update app with latest release tag
		app.Latest = rel.TagName
		
		return assetsFetchedMsg{assets: assets, app: app, release: rel, err: nil}
	}
}

type assetItem struct {
	asset github.Asset
}

func (i assetItem) Title() string       { return i.asset.Name }
func (i assetItem) Description() string { 
	pkgType := packages.DetectType(i.asset.Name)
	sysType := system.GetSystemPreferredType()
	
	sizeStr := fmt.Sprintf("Size: %d bytes", i.asset.Size)
	typeStr := fmt.Sprintf("Type: %s", packages.DisplayName(pkgType))
	
	// Warn if package type doesn't match system
	warning := ""
	if pkgType != sysType && sysType != packages.Unknown {
		warning = " ⚠️  Not native to your system"
	}
	
	return fmt.Sprintf("%s | %s%s", sizeStr, typeStr, warning)
}
func (i assetItem) FilterValue() string { return i.asset.Name }


type updateCheckedMsg struct {
	index   int
	release *github.Release
	err     error
}

func checkUpdateCmd(app config.App, index int) tea.Cmd {
	return func() tea.Msg {
		rel, err := github.GetLatestRelease(app.RepoURL)
		return updateCheckedMsg{index: index, release: rel, err: err}
	}
}

type downloadedMsg struct {
	path string
}

type installFinishedMsg struct {
	err error
}

type installedRecheckedMsg struct {
	app     config.App
	version string
	latest  string
}

func recheckInstalledCmd(app config.App) tea.Cmd {
	return func() tea.Msg {
		version, _, installed := system.CheckInstalled(app.Name)
		if !installed {
			// Try checking with repo name as well
			parts := strings.Split(app.RepoURL, "/")
			if len(parts) > 0 {
				repoName := parts[len(parts)-1]
				if repoName != app.Name {
					if ver, _, ok := system.CheckInstalled(repoName); ok {
						version = ver
					}
				}
			}
		}
		return installedRecheckedMsg{app: app, version: version, latest: app.Latest}
	}
}

func recheckInstalledWithDelayCmd(app config.App) tea.Cmd {
	return func() tea.Msg {
		// Wait for package manager database to update
		time.Sleep(1 * time.Second)
		version, _, installed := system.CheckInstalled(app.Name)
		if !installed {
			// Try checking with repo name as well
			parts := strings.Split(app.RepoURL, "/")
			if len(parts) > 0 {
				repoName := parts[len(parts)-1]
				if repoName != app.Name {
					if ver, _, ok := system.CheckInstalled(repoName); ok {
						version = ver
					}
				}
			}
		}
		return installedRecheckedMsg{app: app, version: version, latest: app.Latest}
	}
}

func downloadAssetCmd(asset *github.Asset) tea.Cmd {
	return func() tea.Msg {
		path, err := installer.DownloadAsset(asset)
		if err != nil {
			return installFinishedMsg{err: err}
		}
		return downloadedMsg{path: path}
	}
}

// execCmdAdapter adapts exec.Cmd to satisfy tea.ExecCommand interface
type execCmdAdapter struct {
*exec.Cmd
}

func (c *execCmdAdapter) SetStdin(r io.Reader)  { c.Stdin = r }
func (c *execCmdAdapter) SetStdout(w io.Writer) { c.Stdout = w }
func (c *execCmdAdapter) SetStderr(w io.Writer) { c.Stderr = w }
