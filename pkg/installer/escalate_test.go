package installer

import (
	"os"
	"os/exec"
	"testing"
)

func TestEscalatorWithoutTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	// A graphical frontend usually hands the CLI /dev/null, which is a
	// character device but not a terminal.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	want := "sudo"
	if _, err := exec.LookPath("pkexec"); err == nil {
		want = "pkexec"
	}

	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()
	for name, f := range map[string]*os.File{"pipe": r, "/dev/null": devNull} {
		os.Stdin = f
		if got := Escalator(); got != want {
			t.Errorf("Escalator() with %s stdin = %q, want %q", name, got, want)
		}
	}
}
