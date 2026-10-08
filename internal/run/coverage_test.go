package run

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Mirac61/yawntest/internal/scan"
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

func TestWithoutCovered(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module cover\n\ngo 1.22\n")
	writeFile(t, dir, "calc.go", `package cover

type Item struct {
	Name string `+"`json:\"name\"`"+`
}

func Double(n int) int { return n * 2 }

func Triple(n int) int { return n * 3 }

func Quadruple(n int) int { return n * 4 }

func route(op string, n int) int {
	switch op {
	case "double":
		return Double(n)
	case "triple":
		return Triple(n)
	}
	return Quadruple(n)
}
`)
	// Like a router test: Double only runs through route, its name never appears.
	writeFile(t, dir, "calc_test.go", "package cover\n\nimport \"testing\"\n\nfunc TestRoute(t *testing.T) { route(\"double\", 1) }\n")
	// --force: yawntest's own tests must not count, or their files would never be rewritten.
	writeFile(t, dir, "calc_yawntest_test.go", "package cover\n\nimport \"testing\"\n\nfunc TestYawntest_Quadruple(t *testing.T) { route(\"\", 1) }\n")

	files, err := scan.Untested(dir, false)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	files, err = WithoutCovered(files)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}

	var got []string
	for _, file := range files {
		for _, match := range file.Matches {
			got = append(got, match.Name)
		}
	}
	// Item has no statements to cover, so only its name could mark it tested.
	want := []string{"Item", "Triple", "Quadruple"}
	if !slices.Equal(got, want) {
		t.Errorf("untested = %v, want %v", got, want)
	}
}

func TestWithoutCoveredBrokenTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module cover\n\ngo 1.22\n")
	writeFile(t, dir, "calc.go", "package cover\n\nfunc Double(n int) int { return n * 2 }\n")
	writeFile(t, dir, "calc_test.go", "package cover\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { undefined() }\n")

	files, err := scan.Untested(dir, true)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Generation must still work when the user's tests are broken, so the names decide alone.
	files, err = WithoutCovered(files)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if len(files) != 1 || len(files[0].Matches) != 1 {
		t.Errorf("files = %+v, want Double kept", files)
	}
}
