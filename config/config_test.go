package config

import "testing"

func TestStripVersion(t *testing.T) {
	cases := map[string]string{
		"Repokase 0.1.0":      "Repokase",
		"Jubako 0.4.5":        "Jubako",
		"Tool v1.2.0":         "Tool",
		"v1.2.0 - Tool":       "Tool",
		"Tool (1.0.0-beta.1)": "Tool",
		"Autonomix CLI":       "Autonomix CLI",
		"omaforge":            "omaforge",
		"v0.3.2":              "",
		"":                    "",
	}
	for in, want := range cases {
		if got := StripVersion(in); got != want {
			t.Errorf("StripVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
