package run

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseProfile(t *testing.T) {
	profile := "mode: set\nexample.com/cover/parse.go:9.2,10.16 2 1\nexample.com/cover/parse.go:11.3,12.1 1 0\n"

	blocks, err := parseProfile([]byte(profile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []coverBlock{
		{file: "parse.go", startLine: 9, startCol: 2, endLine: 10, endCol: 16, count: 1},
		{file: "parse.go", startLine: 11, startCol: 3, endLine: 12, endCol: 1, count: 0},
	}
	if !slices.Equal(blocks, want) {
		t.Errorf("blocks = %+v, want %+v", blocks, want)
	}
}

func TestReturnsErr(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{code: "return err", want: true},
		{code: "return 0, err", want: true},
		{code: `return 0, fmt.Errorf("parse: %w", err)`, want: true},
		{code: "return nil", want: false},
		{code: `return 0, errors.New("boom")`, want: false},
		{code: "return", want: false},
	}

	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nfunc f() {\n"+test.code+"\n}\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			ret := file.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.ReturnStmt)
			if got := returnsErr(ret); got != test.want {
				t.Errorf("returnsErr(%s) = %v, want %v", test.code, got, test.want)
			}
		})
	}
}

func TestUntestedErrorPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module cover\n\ngo 1.22\n")
	writeFile(t, dir, "parse.go", `package cover

import (
	"fmt"
	"strconv"
)

func Parse(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", s, err)
	}
	return n, nil
}

func MustParse(s string) int {
	n, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return n
}
`)
	writeFile(t, dir, "parse_test.go", "package cover\n\nimport \"testing\"\n\nfunc TestParse(t *testing.T) { Parse(\"1\") }\n")

	paths, err := UntestedErrorPaths([]string{dir})
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	want := []ErrorPath{{
		File: filepath.Join(dir, "parse.go"),
		Line: 11,
		Code: `return 0, fmt.Errorf("parse %q: %w", s, err)`,
	}}
	if !slices.Equal(paths, want) {
		t.Errorf("paths = %+v, want %+v", paths, want)
	}
}

func TestUntestedErrorPathsBrokenTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module cover\n\ngo 1.22\n")
	writeFile(t, dir, "parse.go", "package cover\n\nimport \"strconv\"\n\nfunc Parse(s string) (int, error) {\n\tn, err := strconv.Atoi(s)\n\tif err != nil {\n\t\treturn 0, err\n\t}\n\treturn n, nil\n}\n")
	writeFile(t, dir, "parse_test.go", "package cover\n\nimport \"testing\"\n\nfunc TestParse(t *testing.T) { undefined() }\n")

	// Without the error, the empty profile would report every error return as reached.
	if paths, err := UntestedErrorPaths([]string{dir}); err == nil {
		t.Errorf("got paths %+v and no error, want an error for tests that don't build", paths)
	}
}
