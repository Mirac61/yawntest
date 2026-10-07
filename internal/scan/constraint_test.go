package scan

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestBuildConstraint(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		source   string
		want     string
	}{
		{name: "explicit tag", fileName: "open.go", source: "//go:build darwin || linux\n\npackage p\n", want: "//go:build darwin || linux"},
		{name: "tag after a comment", fileName: "open.go", source: "// Copyright\n\n//go:build linux\n\npackage p\n", want: "//go:build linux"},
		{name: "no constraint", fileName: "open.go", source: "package p\n", want: ""},
		{name: "plain os name applies everywhere", fileName: "linux.go", source: "package p\n", want: ""},
		{name: "os suffix", fileName: "open_linux.go", source: "package p\n", want: "//go:build linux"},
		{name: "arch suffix", fileName: "simd_amd64.go", source: "package p\n", want: "//go:build amd64"},
		{name: "os and arch", fileName: "open_linux_arm64.go", source: "package p\n", want: "//go:build linux && arm64"},
		{name: "first part never counts", fileName: "linux_amd64.go", source: "package p\n", want: "//go:build amd64"},
		{name: "unknown suffix", fileName: "open_helpers.go", source: "package p\n", want: ""},
		{name: "comment after package", fileName: "open.go", source: "package p\n\n//go:build linux\n", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), test.fileName, test.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := BuildConstraint(test.fileName, file); got != test.want {
				t.Errorf("BuildConstraint = %q, want %q", got, test.want)
			}
		})
	}
}
