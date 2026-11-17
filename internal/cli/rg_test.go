package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRgArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOpts rgOptions
		wantRest []string
		wantErr  string
	}{
		{
			name:     "no ws flags",
			args:     []string{"needle", "-t", "go"},
			wantRest: []string{"needle", "-t", "go"},
		},
		{
			name:     "repo repeated and comma separated",
			args:     []string{"--repo", "fe,api", "--repo=infra", "needle"},
			wantOpts: rgOptions{repos: []string{"fe", "api", "infra"}},
			wantRest: []string{"needle"},
		},
		{
			name:     "ground then rg flags",
			args:     []string{"--ground", "-i", "needle"},
			wantOpts: rgOptions{ground: true},
			wantRest: []string{"-i", "needle"},
		},
		{
			name:     "all",
			args:     []string{"--all", "needle"},
			wantOpts: rgOptions{all: true},
			wantRest: []string{"needle"},
		},
		{
			name:     "roots with nothing else",
			args:     []string{"--roots"},
			wantOpts: rgOptions{roots: true},
			wantRest: nil,
		},
		{
			name:     "double dash ends ws flags and is dropped",
			args:     []string{"--repo", "fe", "--", "-e", "-needle"},
			wantOpts: rgOptions{repos: []string{"fe"}},
			wantRest: []string{"-e", "-needle"},
		},
		{
			name:     "rg long flag that looks like ours is passed through",
			args:     []string{"--glob", "*.go", "needle"},
			wantRest: []string{"--glob", "*.go", "needle"},
		},
		{
			name:     "rg flags stop ws parsing even if ws flags follow",
			args:     []string{"-i", "--ground", "needle"},
			wantRest: []string{"-i", "--ground", "needle"},
		},
		{
			name:     "help",
			args:     []string{"--help"},
			wantOpts: rgOptions{help: true},
		},
		{
			name:    "ground and all conflict",
			args:    []string{"--ground", "--all", "needle"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "repo without value",
			args:    []string{"--repo"},
			wantErr: "requires a value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, rest, err := parseRgArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(opts, tt.wantOpts) {
				t.Errorf("opts = %+v, want %+v", opts, tt.wantOpts)
			}
			if !reflect.DeepEqual(rest, tt.wantRest) {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
		})
	}
}
