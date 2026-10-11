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

## Usage

Run the application:

```bash
autonomix-cli
```

### Commands

```bash
autonomix-cli add <url>     # Track a GitHub repository
autonomix-cli check         # List tracked apps with available updates
autonomix-cli update [-y]   # Install available updates (-y: don't ask)
autonomix-cli --version
```

`update` only touches apps installed through the system package manager (dpkg, rpm, pacman). Apps found only as a binary on `PATH`, or via Flatpak/Snap, are reported and skipped.

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
