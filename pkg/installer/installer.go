package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
	"golang.org/x/sys/unix"
)

// Log receives progress output from downloads and installs. Commands that
// print machine-readable output on stdout point it at stderr.
var Log io.Writer = os.Stdout

// GetCompatibleAssets returns a list of assets that are compatible with the current system.
func GetCompatibleAssets(release *github.Release) ([]github.Asset, error) {
	sysType := system.GetSystemPreferredType()
	if sysType == packages.Unknown {
		return nil, fmt.Errorf("could not detect system package manager")
	}

	arch := runtime.GOARCH
	// Map go arch to package arch strings commonly used
	archKeywords := []string{arch}
	if arch == "amd64" {
		archKeywords = append(archKeywords, "x86_64", "x64")
	} else if arch == "arm64" {
		archKeywords = append(archKeywords, "aarch64", "armv8")
	}
	
	// Add universal/architecture-independent keywords
	archKeywords = append(archKeywords, "all", "noarch", "any")

	var compatible []github.Asset
	availableTypes := make(map[packages.Type]bool)
	
	for _, asset := range release.Assets {
		detectedType := packages.DetectType(asset.Name)
		if detectedType != packages.Unknown {
			availableTypes[detectedType] = true
		}
		
		if detectedType != sysType {
			continue
		}

		nameLower := strings.ToLower(asset.Name)
		
		// Include if it matches arch, or if it seems universal (no arch keyword)
		// But excluding if it matches wrong arch is safer.
		// Let's include if it matches at least one keyword.
		
		matchedArch := false
		for _, kw := range archKeywords {
			if strings.Contains(nameLower, kw) {
				matchedArch = true
				break
			}
		}

		if matchedArch {
			compatible = append(compatible, asset)
		}
	}
	
	// If no strict matches, do we want to search for "noarch" or "all"?
	if len(compatible) == 0 {
		for _, asset := range release.Assets {
			detectedType := packages.DetectType(asset.Name)
			if detectedType != sysType {
				continue
			}
			// Check if it explicitly says another arch
			// If not, maybe it's universal?
			// This is heuristic.
		}
	}

	// If still no compatible assets, provide helpful error message
	if len(compatible) == 0 && len(availableTypes) > 0 {
		var typeNames []string
		for t := range availableTypes {
			typeNames = append(typeNames, string(t))
		}
		return nil, fmt.Errorf("no %s packages found for %s. Available types: %s", 
			sysType, arch, strings.Join(typeNames, ", "))
	}

	return compatible, nil
}

// GetAllAssets returns all installable assets from a release, regardless of system compatibility.
// Useful as a fallback when no compatible assets are found.
func GetAllAssets(release *github.Release) []github.Asset {
	arch := runtime.GOARCH
	archKeywords := []string{arch}
	if arch == "amd64" {
		archKeywords = append(archKeywords, "x86_64", "x64")
	} else if arch == "arm64" {
		archKeywords = append(archKeywords, "aarch64", "armv8")
	}
	archKeywords = append(archKeywords, "all", "noarch", "any")
	
	var all []github.Asset
	for _, asset := range release.Assets {
		detectedType := packages.DetectType(asset.Name)
		// Only include recognized package types
		if detectedType == packages.Unknown {
			continue
		}
		
		// Filter by arch
		nameLower := strings.ToLower(asset.Name)
		matchedArch := false
		for _, kw := range archKeywords {
			if strings.Contains(nameLower, kw) {
				matchedArch = true
				break
			}
		}
		
		if matchedArch {
			all = append(all, asset)
		}
	}
	return all
}

// DownloadAsset downloads the specified asset to the system temp directory.
func DownloadAsset(asset *github.Asset) (string, error) {
	return DownloadAssetTo(asset, os.TempDir())
}

// DownloadAssetTo downloads the specified asset into dir.
func DownloadAssetTo(asset *github.Asset, dir string) (string, error) {
	downloadPath := filepath.Join(dir, asset.Name)

	fmt.Fprintf(Log, "Downloading %s...\n", asset.BrowserDownloadURL)
	if err := downloadFile(downloadPath, asset.BrowserDownloadURL); err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	
	return downloadPath, nil
}

// DownloadUpdate finds the update in release and downloads it into dir,
// returning the path to the file.
func DownloadUpdate(release *github.Release, dir string) (string, error) {
	assets, err := GetCompatibleAssets(release)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return "", fmt.Errorf("no compatible assets found")
	}
	
	// Default behavior: pick the first one
	return DownloadAssetTo(&assets[0], dir)
}

// Escalator returns the command used to run the installer as root: sudo when
// stdin is a terminal that can answer a password prompt, otherwise pkexec
// (if installed) so a graphical frontend gets a polkit dialog instead.
func Escalator() string {
	if isTerminal(os.Stdin) {
		return "sudo"
	}
	if _, err := exec.LookPath("pkexec"); err == nil {
		return "pkexec"
	}
	return "sudo"
}

// isTerminal reports whether f is a terminal. /dev/null is a character
// device too, so the file mode alone can't tell them apart.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

// GetInstallCmd returns the exec.Cmd that installs all the packages in paths
// with a single elevated command, so the password is asked for once.
// It does NOT set Stdin/Stdout/Stderr, the caller should do that or use tea.Exec
func GetInstallCmd(paths ...string) (*exec.Cmd, error) {
	// pkexec runs commands with a minimal environment, so pass absolute paths.
	absPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		absPaths = append(absPaths, abs)
	}
	args, err := installArgs(system.GetSystemPreferredType(), absPaths)
	if err != nil {
		return nil, err
	}
	return exec.Command(Escalator(), args...), nil
}

// installArgs returns the command line that installs the packages in paths
// with the package manager for sysType.
func installArgs(sysType packages.Type, paths []string) ([]string, error) {
	var args []string
	switch sysType {
	case packages.Deb:
		args = []string{"apt-get", "install", "-y"}
	case packages.Rpm:
		args = []string{"rpm", "-Uvh"}
	case packages.Pacman:
		args = []string{"pacman", "-U", "--noconfirm"}
	default:
		return nil, fmt.Errorf("unsupported install type: %s", sysType)
	}
	return append(args, paths...), nil
}

// InstallPackages installs every package in paths with one elevated command.
func InstallPackages(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	cmd, err := GetInstallCmd(paths...)
	if err != nil {
		return err
	}

	// Connect to stdout/stderr so user sees password prompt and output, and
	// keep stderr to explain a failure.
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stdout = Log
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	fmt.Fprintf(Log, "Installing %s...\n", strings.Join(paths, " "))
	return installError(cmd.Args[0], cmd.Run(), stderr.String())
}

// installError explains err from running the installer through root, the
// escalator, using the installer's stderr.
func installError(root string, err error, stderr string) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		switch {
		// pkexec exits 126 when the polkit dialog is dismissed and 127 when
		// authentication fails or isn't allowed.
		case root == "pkexec" && code == 126:
			return fmt.Errorf("authentication cancelled")
		case root == "pkexec" && code == 127:
			return fmt.Errorf("authentication failed")
		case root == "sudo" && strings.Contains(stderr, "incorrect password"):
			return fmt.Errorf("authentication failed")
		}
	}
	if out := lastLines(stderr, 10); out != "" {
		return fmt.Errorf("%w: %s", err, out)
	}
	return err
}

// lastLines returns the last n non-empty lines of s, joined by newlines.
func lastLines(s string, n int) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func findMatchingAsset(assets []github.Asset, sysType packages.Type) (*github.Asset, error) {
	arch := runtime.GOARCH
	// Map go arch to package arch strings commonly used
	archKeywords := []string{arch}
	if arch == "amd64" {
		archKeywords = append(archKeywords, "x86_64", "x64")
	} else if arch == "arm64" {
		archKeywords = append(archKeywords, "aarch64", "armv8")
	}

	// Add universal/architecture-independent keywords
	archKeywords = append(archKeywords, "all", "noarch", "any")

	for _, asset := range assets {
		detectedType := packages.DetectType(asset.Name)
		if detectedType != sysType {
			continue
		}

		// Check arch
		nameLower := strings.ToLower(asset.Name)
		for _, kw := range archKeywords {
			if strings.Contains(nameLower, kw) {
				return &asset, nil
			}
		}
		
		// Fallback: if no arch info is in the name, but type matches, it might be universal or the only one.
		// But risky. Let's look for one that doesn't contradict.
		// Actually, let's just return the first match of the type if strict arch match fails, 
		// but typically release assets have arch in name.
	}

	return nil, fmt.Errorf("no matching asset found for type %s and arch %s", sysType, arch)
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

