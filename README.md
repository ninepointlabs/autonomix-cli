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
