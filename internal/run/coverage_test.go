package run

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

func TestParseProfileBadLines(t *testing.T) {
	for _, line := range []string{"no colon here", "example.com/x.go:9.2,10 2 1"} {
		if _, err := parseProfile([]byte("mode: set\n" + line + "\n")); err == nil {
			t.Errorf("parseProfile(%q) = nil error, want one", line)
		}
	}
	if _, err := coverProfileOf(t, "no colon here"); err == nil || !strings.Contains(err.Error(), "bad coverage line") {
		t.Errorf("err = %v, want the bad line reported from a written profile", err)
	}
}

// coverProfileOf runs coverProfile against a fake go that writes profile into -coverprofile.
func coverProfileOf(t *testing.T, profile string) ([]coverBlock, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake go is a shell script")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nfor arg; do case $arg in -coverprofile=*) printf 'mode: set\\n%s\\n' '" + profile + "' > \"${arg#-coverprofile=}\";; esac; done\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return coverProfile(t.TempDir())
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

	tree, err := scan.Scan(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	paths := UntestedErrorPaths(tree.Files, tree.HasTests, NewCoverage()).Paths
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

	tree, err := scan.Scan(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Without the warning, the empty profile would report every error return as reached.
	got := UntestedErrorPaths(tree.Files, tree.HasTests, NewCoverage())
	if len(got.Paths) != 0 || len(got.Warnings) != 1 {
		t.Errorf("got %+v, want no paths and one warning for tests that don't build", got)
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

	tree, err := scan.Scan(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	files := WithoutCovered(tree.Untested(false), NewCoverage(), true)

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

	tree, err := scan.Scan(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Generation must still work when the user's tests are broken, so the names decide alone.
	files := WithoutCovered(tree.Untested(true), NewCoverage(), true)
	if len(files) != 1 || len(files[0].Matches) != 1 {
		t.Errorf("files = %+v, want Double kept", files)
	}
}
