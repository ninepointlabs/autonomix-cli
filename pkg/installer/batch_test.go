package installer

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
)

func TestInstallArgs(t *testing.T) {
	paths := []string{"/tmp/a/one", "/tmp/b/two", "/tmp/c/three"}
	tests := []struct {
		typ  packages.Type
		want []string
	}{
		{packages.Pacman, []string{"pacman", "-U", "--noconfirm", "/tmp/a/one", "/tmp/b/two", "/tmp/c/three"}},
		{packages.Deb, []string{"apt-get", "install", "-y", "/tmp/a/one", "/tmp/b/two", "/tmp/c/three"}},
		{packages.Rpm, []string{"rpm", "-Uvh", "/tmp/a/one", "/tmp/b/two", "/tmp/c/three"}},
	}
	for _, tt := range tests {
		got, err := installArgs(tt.typ, paths)
		if err != nil {
			t.Errorf("installArgs(%s) returned error: %v", tt.typ, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("installArgs(%s) = %q, want %q", tt.typ, got, tt.want)
		}
	}

	if _, err := installArgs(packages.Flatpak, paths); err == nil {
		t.Error("installArgs(flatpak) should fail")
	}
}

func TestGetInstallCmdEscalatesOnce(t *testing.T) {
	if system.GetSystemPreferredType() == packages.Unknown {
		t.Skip("Skipping test as no package manager detected")
	}

	cmd, err := GetInstallCmd("a.pkg", "b.pkg", "c.pkg")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cmd.Args[0], Escalator(); got != want {
		t.Errorf("command runs %q, want escalator %q", got, want)
	}
	escalations := 0
	for _, a := range cmd.Args {
		if a == "sudo" || a == "pkexec" {
			escalations++
		}
	}
	if escalations != 1 {
		t.Errorf("command %q escalates %d times, want 1", cmd.Args, escalations)
	}
	// Every package is passed by absolute path, after the installer's options.
	tail := cmd.Args[len(cmd.Args)-3:]
	for i, name := range []string{"a.pkg", "b.pkg", "c.pkg"} {
		if !filepath.IsAbs(tail[i]) || filepath.Base(tail[i]) != name {
			t.Errorf("package %d is %q, want an absolute path to %s", i, tail[i], name)
		}
	}
}

func exitErr(t *testing.T, code string) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+code).Run()
	if err == nil {
		t.Fatalf("exit %s did not fail", code)
	}
	return err
}

func TestInstallError(t *testing.T) {
	if err := installError("pkexec", nil, "noise"); err != nil {
		t.Errorf("installError(nil) = %v, want nil", err)
	}

	tests := []struct {
		name   string
		root   string
		code   string
		stderr string
		want   string
	}{
		{"polkit dialog dismissed", "pkexec", "126", "", "authentication cancelled"},
		{"polkit auth failed", "pkexec", "127", "", "authentication failed"},
		{"sudo wrong password", "sudo", "1", "sudo: 3 incorrect password attempts\n", "authentication failed"},
		{"installer error", "pkexec", "1", "loading packages...\nerror: failed to commit transaction (conflicting files)\n", "exit status 1: loading packages...\nerror: failed to commit transaction (conflicting files)"},
		{"no output", "sudo", "1", "", "exit status 1"},
	}
	for _, tt := range tests {
		got := installError(tt.root, exitErr(t, tt.code), tt.stderr)
		if got == nil || got.Error() != tt.want {
			t.Errorf("%s: installError() = %v, want %q", tt.name, got, tt.want)
		}
	}
}

func TestLastLines(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 15; i++ {
		b.WriteString("line\n\n")
	}
	b.WriteString("last\n")
	got := strings.Split(lastLines(b.String(), 10), "\n")
	if len(got) != 10 || got[9] != "last" {
		t.Errorf("lastLines() = %q, want 10 lines ending in \"last\"", got)
	}
}
