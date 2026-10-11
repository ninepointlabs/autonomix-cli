# Copilot Instructions for Autonomix CLI

## Project Overview

Autonomix CLI is a Go-based TUI application that manages Linux application installations from GitHub releases. It tracks applications, detects available package formats (.deb, .rpm, .flatpak, .snap, .appimage, Arch packages), and integrates with system package managers to show installed versions.

## Build and Test

**Build:**
```bash
go build
```

**Run locally:**
```bash
go run main.go
```

**Run tests:**
```bash
go test ./...
```

**Run specific test:**
```bash
go test -run TestName ./pkg/installer
```

**Release build:**
Uses GoReleaser - see `.goreleaser.yaml` for configuration. Builds for Linux (amd64/arm64) and generates .deb, .rpm, and Arch packages.

## Architecture

### Core Flow
1. **main.go**: Entry point. Handles CLI args for adding repos (`autonomix-cli add <url>` or just `autonomix-cli <url>`). Ensures the app tracks itself at `SelfRepoURL`.
2. **config/**: Manages `~/.autonomix/config.json` persistence. Stores list of tracked apps with their repo URLs, versions, and latest release info.
3. **pkg/manager**: Orchestrates adding apps - cleans GitHub URLs, fetches releases, detects system-installed versions via `pkg/system`.
4. **pkg/github**: API client for fetching GitHub releases and assets.
5. **pkg/system**: Queries system package managers (dpkg, rpm, pacman, flatpak, snap) to detect installed versions.
6. **pkg/packages**: Detects package type from asset filename (deb, rpm, flatpak, etc.).
7. **pkg/installer**: Filters compatible assets based on OS/architecture and package type, handles installation commands.
8. **tui/model.go**: Bubble Tea TUI with three states: `viewList` (main list), `viewAdd` (text input for URL), `viewSelectAsset` (choose which asset to install).

### Key Data Flow
- User adds repo → `manager.AddApp()` → GitHub API → detect system version → save to config → refresh TUI
- User presses 'u' on item → fetch latest release → compare versions → prompt to install if update available
- Version comparison uses `updater.NormalizeVersion()` to strip "v" prefixes and package revision suffixes (e.g., "-1")

## Conventions

**Self-tracking**: The app always tracks itself via `SelfRepoURL` constant (defined in both `main.go` and `tui/model.go`). On startup, it adds itself if missing and updates its version from the `version` variable (set by GoReleaser).

**URL normalization**: GitHub URLs are cleaned to base repo format (`https://github.com/owner/repo`) - strips `/releases`, trailing slashes, etc.

**Version normalization**: The `updater.NormalizeVersion()` function removes:
- "v" prefix (v1.0.0 → 1.0.0)
- Debian revision suffix (1.0.0-1 → 1.0.0)
- RPM dist tags (1.0.0-1.el9 → 1.0.0)

This ensures accurate version comparisons between GitHub releases and system-installed packages.

**TUI keybindings**:
- Start typing → add new repo
- Enter → confirm
- u → check/install updates
- d → delete (stop tracking)
- q/Ctrl+C → quit

**State management**: TUI uses three states (`viewList`, `viewAdd`, `viewSelectAsset`). Always return to `viewList` after operations. The list is rebuilt on state transitions to reflect config changes.

**Error handling**: Operations (add, update, delete) show status messages via `model.statusMessage` and `model.statusTime`. Messages auto-clear after 3 seconds.
