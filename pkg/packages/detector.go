package packages

import (
	"os/exec"
	"path/filepath"
	"strings"
)

type Type string

const (
	Deb      Type = "deb"
	Rpm      Type = "rpm"
	Flatpak  Type = "flatpak"
	Snap     Type = "snap"
	Pacman   Type = "pacman"
	AppImage Type = "appimage"
	Unknown  Type = "unknown"
)

func DetectType(filename string) Type {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".deb") {
		return Deb
	}
	if strings.HasSuffix(lower, ".rpm") {
		return Rpm
	}
	if strings.HasSuffix(lower, ".flatpak") || strings.HasSuffix(lower, ".flatpakref") {
		return Flatpak
	}
	if strings.HasSuffix(lower, ".snap") {
		return Snap
	}
	if strings.HasSuffix(lower, ".pkg.tar.zst") || strings.HasSuffix(lower, ".pkg.tar.xz") {
		return Pacman
	}
	if strings.HasSuffix(lower, ".appimage") {
		return AppImage // Bonus, usually useful
	}
	return Unknown
}

func DisplayName(t Type) string {
	switch t {
	case Deb:
		return "Debian Package (.deb)"
	case Rpm:
		return "RPM Package (.rpm)"
	case Flatpak:
		return "Flatpak"
	case Snap:
		return "Snap Package"
	case Pacman:
		return "Arch Package"
	case AppImage:
		return "AppImage"
	default:
		return "Unknown"
	}
}

// NameFromFile parses the package name from a package file name, e.g.
// "idea-0.1.1-1-x86_64.pkg.tar.zst" -> "idea", "idea_0.1.1_amd64.deb" -> "idea",
// "idea-0.1.1-1.x86_64.rpm" -> "idea". It returns "" when the name doesn't
// follow the package type's naming convention.
func NameFromFile(filename string) string {
	base := filepath.Base(filename)
	lower := strings.ToLower(base)
	switch DetectType(base) {
	case Pacman:
		// name-pkgver-pkgrel-arch.pkg.tar.*
		stem := base[:strings.Index(lower, ".pkg.tar.")]
		return trimDashFields(stem, 3)
	case Deb:
		// name_version_arch.deb
		stem := base[:len(base)-len(".deb")]
		if i := strings.Index(stem, "_"); i > 0 {
			return stem[:i]
		}
	case Rpm:
		// name-version-release.arch.rpm
		stem := base[:len(base)-len(".rpm")]
		if i := strings.LastIndex(stem, "."); i > 0 {
			stem = stem[:i]
		}
		return trimDashFields(stem, 2)
	}
	return ""
}

// trimDashFields drops the last n dash-separated fields of s, returning ""
// if s doesn't have more than n fields.
func trimDashFields(s string, n int) string {
	parts := strings.Split(s, "-")
	if len(parts) <= n {
		return ""
	}
	return strings.Join(parts[:len(parts)-n], "-")
}

// queryCommands read the package name from a package file per type.
var queryCommands = map[Type][]string{
	Pacman: {"pacman", "-Qip"},
	Deb:    {"dpkg-deb", "--show", "--showformat=${Package}"},
	Rpm:    {"rpm", "-qp", "--queryformat", "%{NAME}"},
}

// NameOfFile returns the name the package manager will install the package
// file at path as, asking the package manager and falling back to parsing
// the file name.
func NameOfFile(path string) string {
	t := DetectType(path)
	if args, ok := queryCommands[t]; ok {
		cmd := exec.Command(args[0], append(args[1:], path)...)
		if out, err := cmd.Output(); err == nil {
			if name := parseQueryOutput(t, string(out)); name != "" {
				return name
			}
		}
	}
	return NameFromFile(path)
}

func parseQueryOutput(t Type, out string) string {
	if t != Pacman {
		return strings.TrimSpace(out)
	}
	// pacman -Qip prints "Name            : idea" among other fields
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "Name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
