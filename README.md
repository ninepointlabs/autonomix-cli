# Autonomix CLI

Autonomix CLI is a terminal-based utility written in Go that allows you to easily install and manage applications directly from GitHub Releases.

## Features

- **Install from GitHub**: Add any GitHub repository URL to track.
- **Auto-Detection**: Recognizes `.deb`, `.rpm`, `.flatpak`, `.snap`, `.appimage`, and Arch packages.
- **Smart Updates**: Checks for new releases on GitHub.
- **System Integration**: Detects if the application is already installed on your system (dpkg, rpm, pacman, flatpak, snap) and shows the installed version.
- **TUI**: Simple and easy-to-use Terminal User Interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Installation

### From GitHub Releases

Go to the [Releases](https://github.com/ninepointlabs/autonomix-cli/releases) page and download the package for your system.

**Debian/Ubuntu:**
```bash
sudo dpkg -i autonomix-cli_*.deb
```

**Fedora/RHEL:**
```bash
sudo rpm -i autonomix-cli_*.rpm
```

**Arch Linux:**
```bash
sudo pacman -U autonomix-cli_*.pkg.tar.zst
```

### Keeping it updated with [autonomix-cli](https://github.com/ninepointlabs/autonomix-cli)

Once installed, Autonomix CLI tracks itself, so `autonomix-cli update` picks up new releases. To add it explicitly:

```bash
autonomix-cli add https://github.com/ninepointlabs/autonomix-cli
```

### Omarchy bar widget

[omarchy-autonomix](https://github.com/ninepointlabs/omarchy-autonomix) puts your tracked apps and their updates in the Omarchy bar, using the `--json` commands below.

## Usage

Run the application:

```bash
autonomix-cli
```

### Commands

```bash
autonomix-cli add <url>             # Track a GitHub repository (URL or owner/repo)
autonomix-cli list                  # List tracked apps and what's installed (no network)
autonomix-cli check [app...]        # Check tracked apps for updates
autonomix-cli update [-y] [app...]  # Install available updates (-y: don't ask)
autonomix-cli install <app>         # Install a tracked app that isn't installed yet
autonomix-cli remove <app>          # Stop tracking an app (does not uninstall it)
autonomix-cli --version
```

`<app>` can be the app name, repo name, `owner/repo` or the repository URL.

`update` only touches apps installed through the system package manager (dpkg, rpm, pacman). Apps found only as a binary on `PATH`, or via Flatpak/Snap, are reported and skipped.

### JSON output

Add `--json` to `add`, `list`, `check`, `update`, `install` or `remove` to get machine-readable output on stdout, for scripts and graphical frontends. Progress messages go to stderr, `--json` implies `-y`, and errors are printed as `{"error": "..."}` with exit code 1. Each app looks like this:

```json
{
  "name": "Repokase",
  "repo": "ninepointlabs/repokase",
  "repo_url": "https://github.com/ninepointlabs/repokase",
  "status": "update_available",
  "installed": "0.2.1-1",
  "installed_type": "pacman",
  "latest": "v0.2.2",
  "release_url": "https://github.com/ninepointlabs/repokase/releases/tag/v0.2.2",
  "last_checked": "2026-10-14T12:00:00-05:00"
}
```

`status` is one of `up_to_date`, `update_available`, `not_installed`, `unmanaged` (installed outside the system package manager), `unchecked` (installed, no release fetched yet) or `failed` (with `error`). After `update` or `install`, apps that were installed have `"updated": true`.

When stdin isn't a terminal, packages are installed with `pkexec` instead of `sudo`, so a graphical frontend gets a polkit password dialog.

Set `GITHUB_TOKEN` (or `GH_TOKEN`) to raise the GitHub API rate limit from 60 to 5000 requests per hour.

### Omarchy

To update Autonomix apps whenever you run `omarchy update`, install the post-update hook:

```bash
install -m755 contrib/omarchy/autonomix.hook ~/.config/omarchy/hooks/post-update.d/autonomix.hook
```

### Controls

- **Start Typing**: To add a new GitHub repository URL.
- **Enter**: Confirm adding a repo.
- **u**: Check for updates for the selected app.
- **d**: Delete/Remove an app from the list (stops tracking).
- **q / Ctrl+C**: Quit.

## Configuration

Configuration is stored in `~/.autonomix/config.json`.

## building

```bash
go build
```
