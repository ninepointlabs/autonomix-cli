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

	stdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = stdin }()

	want := "sudo"
	if _, err := exec.LookPath("pkexec"); err == nil {
		want = "pkexec"
	}
	if got := Escalator(); got != want {
		t.Errorf("Escalator() with piped stdin = %q, want %q", got, want)
	}
}
