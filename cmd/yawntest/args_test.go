package main

import (
	"errors"
	"flag"
	"io"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{name: "check", args: []string{"check"}, want: options{command: "check", root: "."}},
		{name: "path with dots", args: []string{"gen", "./..."}, want: options{command: "gen", root: "."}},
		{name: "subdir", args: []string{"check", "internal/..."}, want: options{command: "check", root: "internal"}},
		{
			name: "all flags",
			args: []string{"run", "--force", "--changed", "--json", "api"},
			want: options{command: "run", root: "api", force: true, changed: true, json: true},
		},
		{name: "no command", args: nil, wantErr: true},
		{name: "unknown command", args: []string{"test"}, wantErr: true},
		{name: "old flag style", args: []string{"--check", "."}, wantErr: true},
		{name: "check writes nothing, so no --force", args: []string{"check", "--force"}, wantErr: true},
		{name: "flag after path", args: []string{"check", ".", "--json"}, wantErr: true},
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

func TestVersion(t *testing.T) {
	if version() == "" {
		t.Error("version() is empty")
	}
}

func TestParseArgsHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"run", "-h"}} {
		if _, err := parseArgs(args, io.Discard); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("parseArgs(%q) err = %v, want flag.ErrHelp", args, err)
		}
	}
}
