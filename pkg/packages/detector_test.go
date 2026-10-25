package packages

import "testing"

func TestNameFromFile(t *testing.T) {
	cases := map[string]string{
		"idea-0.1.1-1-x86_64.pkg.tar.zst":        "idea",
		"/tmp/x/idea-0.1.1-1-x86_64.pkg.tar.zst": "idea",
		"autonomix-cli-0.4.2-1-any.pkg.tar.xz":   "autonomix-cli",
		"idea_0.1.1_amd64.deb":                   "idea",
		"my-tool_1.0.0-1_arm64.deb":              "my-tool",
		"idea-0.1.1-1.x86_64.rpm":                "idea",
		"my-tool-1.0.0-1.fc40.noarch.rpm":        "my-tool",
		"idea.pkg.tar.zst":                       "",
		"idea_linux_amd64.tar.gz":                "",
		"Idea-0.1.1.AppImage":                    "",
	}
	for in, want := range cases {
		if got := NameFromFile(in); got != want {
			t.Errorf("NameFromFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseQueryOutput(t *testing.T) {
	pacman := "Name            : idea\nVersion         : 0.1.1-1\nDescription     : Ideas: capture them\n"
	if got := parseQueryOutput(Pacman, pacman); got != "idea" {
		t.Errorf("pacman: got %q, want idea", got)
	}
	if got := parseQueryOutput(Deb, "idea\n"); got != "idea" {
		t.Errorf("deb: got %q, want idea", got)
	}
	if got := parseQueryOutput(Pacman, "error: could not open file"); got != "" {
		t.Errorf("pacman error: got %q, want empty", got)
	}
}
