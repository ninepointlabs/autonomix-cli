package manager

import (
	"testing"

	"github.com/ninepointlabs/autonomix-cli/config"
)

func TestFindApp(t *testing.T) {
	cfg := &config.Config{Apps: []config.App{
		{Name: "Repokase", RepoURL: "https://github.com/ninepointlabs/repokase"},
		{Name: "Jubako", RepoURL: "https://github.com/someone/jubako-app"},
		{Name: "tool", RepoURL: "https://github.com/a/tool"},
		{Name: "tool-b", RepoURL: "https://github.com/b/tool"},
	}}

	tests := []struct {
		query   string
		want    int
		wantErr bool
	}{
		{"repokase", 0, false},
		{"REPOKASE", 0, false},
		{"ninepointlabs/repokase", 0, false},
		{"https://github.com/ninepointlabs/repokase/", 0, false},
		{"github.com/someone/jubako-app", 1, false},
		{"jubako-app", 1, false},
		{"Jubako", 1, false},
		{"b/tool", 3, false},
		{"tool", -1, true}, // name of one app, repo name of another
		{"missing", -1, true},
		{"", -1, true},
	}
	for _, tt := range tests {
		got, err := FindApp(cfg, tt.query)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("FindApp(%q) = %d, %v; want %d, err=%v", tt.query, got, err, tt.want, tt.wantErr)
		}
	}
}
