package installer

import (
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
)

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

// DownloadAsset downloads the specified asset
func DownloadAsset(asset *github.Asset) (string, error) {
	tempDir := os.TempDir()
	fileName := asset.Name
	downloadPath := filepath.Join(tempDir, fileName)

	fmt.Printf("Downloading %s...\n", asset.BrowserDownloadURL)
	if err := downloadFile(downloadPath, asset.BrowserDownloadURL); err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	
	return downloadPath, nil
}

// DownloadUpdate finds and downloads the update, returning the path to the file.
func DownloadUpdate(release *github.Release) (string, error) {
	assets, err := GetCompatibleAssets(release)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return "", fmt.Errorf("no compatible assets found")
	}
	
	// Default behavior: pick the first one
	return DownloadAsset(&assets[0])
}

// GetInstallCmd returns the exec.Cmd to install the package.
// It does NOT set Stdin/Stdout/Stderr, the caller should do that or use tea.Exec
func GetInstallCmd(path string) (*exec.Cmd, error) {
	sysType := system.GetSystemPreferredType()
	
	switch sysType {
	case packages.Deb:
		// sudo apt-get install -y ./path
		// Using relative path for apt sometimes requires ./
		absPath, _ := filepath.Abs(path)
		return exec.Command("sudo", "apt-get", "install", "-y", absPath), nil
	case packages.Rpm:
		return exec.Command("sudo", "rpm", "-Uvh", path), nil
	case packages.Pacman:
		return exec.Command("sudo", "pacman", "-U", "--noconfirm", path), nil
	default:
		return nil, fmt.Errorf("unsupported install type: %s", sysType)
	}
}

func InstallUpdate(release *github.Release) error {
	path, err := DownloadUpdate(release)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	cmd, err := GetInstallCmd(path)
	if err != nil {
		return err
	}

	// Connect to stdout/stderr so user sees password prompt and output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Installing %s...\n", path)
	return cmd.Run()
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

