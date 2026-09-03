package system

import (
	"os/exec"
	"strings"
	"time"

	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
)

// CheckInstalled checks if an application is installed via various package managers.
// It returns the version string, the package type, true if found.
func CheckInstalled(appName string) (string, packages.Type, bool) {
	// Generate candidate names to check
	// e.g. "My App" -> ["My App", "my app", "my-app"]
	candidates := []string{appName}
	
	lower := strings.ToLower(appName)
	if lower != appName {
		candidates = append(candidates, lower)
	}
	
	// Replace both spaces and underscores with hyphens
	kebab := strings.ReplaceAll(strings.ReplaceAll(lower, " ", "-"), "_", "-")
	if kebab != lower && kebab != appName {
		candidates = append(candidates, kebab)
	}

	// Heuristic: strip "-cli" or " cli" suffix
	suffixes := []string{"-cli", " cli", "_cli", "cli"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			trimmed := strings.TrimSuffix(lower, suffix)
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				candidates = append(candidates, trimmed)
			}
		}
	}

	// Deduplicate candidates
	unique := make(map[string]bool)
	var finalCandidates []string
	for _, c := range candidates {
		if !unique[c] {
			unique[c] = true
			finalCandidates = append(finalCandidates, c)
		}
	}

	for _, name := range finalCandidates {
		if name == "" {
			continue
		}
		
		// Try each package manager with this name
		
		// Check Snap
		if ver, ok := checkSnap(name); ok {
			return ver, packages.Snap, true
		}
		
		// Check Flatpak
		if ver, ok := checkFlatpak(name); ok {
			return ver, packages.Flatpak, true
		}
		
		// Check Dpkg (Debian/Ubuntu)
		if ver, ok := checkDpkg(name); ok {
			return ver, packages.Deb, true
		}
		
		// Check Pacman (Arch)
		if ver, ok := checkPacman(name); ok {
			return ver, packages.Pacman, true
		}
		
		// Check RPM
		if ver, ok := checkRpm(name); ok {
			return ver, packages.Rpm, true
		}

		// Check Binary in Path (Fallback)
		if ver, ok := checkBinary(name); ok {
			return ver, packages.Unknown, true
		}
	}

	return "", packages.Unknown, false
}

func checkBinary(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	
	// Try to get version
	versionArgs := [][]string{
		{"--version"},
		{"-v"},
		{"version"},
	}

	for _, args := range versionArgs {
		cmd := exec.Command(path, args...)
		// Add timeout to prevent hanging on misbehaving binaries
		done := make(chan error, 1)
		var out []byte
		go func() {
			out, err = cmd.Output()
			done <- err
		}()
		
		select {
		case <-done:
			if err == nil && len(out) > 0 {
				// clean up output, take first line
				ver := strings.TrimSpace(string(out))
				if idx := strings.Index(ver, "\n"); idx != -1 {
					ver = ver[:idx]
				}
				return ver, true
			}
		case <-time.After(2 * time.Second):
			// Timeout - kill the process and try next arg
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}
	}
	
	return "detected", true
}

// GetSystemPreferredType returns the preferred package type for the running system
func GetSystemPreferredType() packages.Type {
	// Use /etc/os-release for accurate detection
	osID := getOSID()
	
	switch osID {
	case "arch", "manjaro", "endeavouros", "garuda":
		return packages.Pacman
	case "debian", "ubuntu", "linuxmint", "pop", "elementary":
		return packages.Deb
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return packages.Rpm
	}
	
	// Fallback to checking for package managers
	if _, err := exec.LookPath("pacman"); err == nil {
		return packages.Pacman
	}
	if _, err := exec.LookPath("dpkg"); err == nil {
		return packages.Deb
	}
	if _, err := exec.LookPath("rpm"); err == nil {
		return packages.Rpm
	}
	return packages.Unknown
}

func getOSID() string {
	// Read /etc/os-release and extract ID
	cmd := exec.Command("sh", "-c", ". /etc/os-release && echo $ID")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}


func checkSnap(name string) (string, bool) {
	// snap list name
	cmd := exec.Command("snap", "list", name)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return "", false
	}
	fields := strings.Fields(lines[1])
	if len(fields) >= 2 {
		return fields[1], true
	}
	return "", false
}

func checkFlatpak(name string) (string, bool) {
	// flatpak list --app --columns=application,version
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application,name,version")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	
	lowerName := strings.ToLower(name)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// Expected: com.example.App Name Version
		if len(fields) >= 3 {
			appID := strings.ToLower(fields[0])
			appName := strings.ToLower(fields[1])
			
			// Heuristic: if ID ends with name or name matches
			if appName == lowerName || strings.HasSuffix(appID, "." + lowerName) {
				return fields[2], true
			}
		}
	}
	return "", false
}

func checkDpkg(name string) (string, bool) {
	// dpkg-query -W -f='${Version}' name
	cmd := exec.Command("dpkg-query", "-W", "-f=${Version}", name)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return string(out), true
	}
	return "", false
}

func checkPacman(name string) (string, bool) {
	// pacman -Q name
    // output: name version
	cmd := exec.Command("pacman", "-Q", name)
	out, err := cmd.Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 2 {
			return parts[1], true
		}
	}
	return "", false
}

func checkRpm(name string) (string, bool) {
	// rpm -q --qf "%{VERSION}" name
	cmd := exec.Command("rpm", "-q", "--qf", "%{VERSION}", name)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return string(out), true
	}
	return "", false
}
