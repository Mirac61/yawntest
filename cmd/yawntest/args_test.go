package main

import (
	"errors"
	"flag"
	"io"
	"runtime/debug"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{name: "check", args: []string{"check"}, want: options{command: "check", root: ".", format: "text"}},
		{name: "path with dots", args: []string{"gen", "./..."}, want: options{command: "gen", root: ".", format: "text"}},
		{name: "subdir", args: []string{"check", "internal/..."}, want: options{command: "check", root: "internal", format: "text"}},
		{
			name: "all flags",
			args: []string{"run", "--force", "--changed", "--json", "api"},
			want: options{command: "run", root: "api", force: true, changed: true, json: true, format: "json"},
		},
		{
			name: "flags after the path",
			args: []string{"check", "./...", "--static", "--since", "origin/main"},
			want: options{command: "check", root: ".", static: true, since: "origin/main", format: "text"},
		},
		{
			name: "github format",
			args: []string{"check", "--format=github", "api/..."},
			want: options{command: "check", root: "api", format: "github"},
		},
		{name: "no command", args: nil, wantErr: true},
		{name: "unknown command", args: []string{"test"}, wantErr: true},
		{name: "old flag style", args: []string{"--check", "."}, wantErr: true},
		{name: "check writes nothing, so no --force", args: []string{"check", "--force"}, wantErr: true},
		{name: "run always runs tests, so no --static", args: []string{"run", "--static"}, wantErr: true},
		{name: "two paths", args: []string{"check", "a", "b"}, wantErr: true},
		{name: "unknown format", args: []string{"check", "--format", "xml"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseArgs(test.args, io.Discard)
			if (err != nil) != test.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestVersionOf(t *testing.T) {
	stamped := &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}
	tests := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"no build info", nil, false, "(devel)"},
		{"unstamped build", &debug.BuildInfo{}, true, "(devel)"},
		{"go install", stamped, true, "v0.2.0"},
	}
	for _, tt := range tests {
		if got := versionOf(tt.info, tt.ok); got != tt.want {
			t.Errorf("%s: versionOf = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseArgsHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"run", "-h"}} {
		if _, err := parseArgs(args, io.Discard); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("parseArgs(%q) err = %v, want flag.ErrHelp", args, err)
		}
	}
}
