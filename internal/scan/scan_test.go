package scan

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// testdata holds:
//   - Tested: called from a_test.go
//   - Untested: no test mentions it
//   - OnlyInYawntest: covered by a generated yawntest file
//   - Generated: lives in a "Code generated ... DO NOT EDIT." file
//   - Vendored: lives under vendor/
//   - sub/, z.go: a subdir sorted between files, so Dirs must not list testdata twice
//   - fee.go: no candidates, only a money hint, and must still be returned
func TestUntested(t *testing.T) {
	tests := []struct {
		name          string
		countYawntest bool
		want          []string
	}{
		{name: "yawntest files count as tests", countYawntest: true, want: []string{"Untested"}},
		{name: "yawntest files ignored", countYawntest: false, want: []string{"Untested", "OnlyInYawntest"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree, err := Scan("testdata", Options{})
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			files := tree.Untested(test.countYawntest)

			var names []string
			hintCount := 0
			for _, file := range files {
				if got, want := file.Package, "pkg"; got != want {
					t.Errorf("%s: Package = %q, want %q", file.Path, got, want)
				}
				for _, match := range file.Matches {
					names = append(names, match.Name)
				}
				hintCount += len(file.Hints)
			}
			if hintCount != 1 {
				t.Errorf("got %d hints, want 1 (defaultFee in a file without candidates)", hintCount)
			}
			if !slices.Equal(names, test.want) {
				t.Errorf("names = %v, want %v", names, test.want)
			}
		})
	}
}

func TestDirs(t *testing.T) {
	tests := []struct {
		name string
		keep func(string) bool
		want []string
	}{
		{name: "yawntest files", keep: IsYawntestFile, want: []string{"testdata"}},
		{name: "source files skip vendor", keep: IsSourceFile, want: []string{"testdata", "testdata/sub"}},
		{name: "nothing", keep: func(string) bool { return false }, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dirs, err := Dirs("testdata", test.keep)
			if err != nil {
				t.Fatalf("dirs: %v", err)
			}
			if !slices.Equal(dirs, test.want) {
				t.Errorf("dirs = %v, want %v", dirs, test.want)
			}
		})
	}
}

func TestSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "vendor", want: true},
		{name: "testdata", want: true},
		{name: ".git", want: true},
		{name: "_examples", want: true},
		{name: "internal", want: false},
		{name: "vendors", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := skipDir(test.name); got != test.want {
				t.Errorf("skipDir(%q) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

func TestScanTree(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("text.go", `package text

import "strings"

// Split is untested: the test below only calls strings.Split.
func Split(s string) []string { return strings.Fields(s) }

// Upper is tested through text.Upper in the external test package.
func Upper(s string) string { return strings.ToUpper(s) }

// Shout calls a helper in another file that prints, so it isn't pure.
func Shout(s string) string { return say(s) }

//yawntest:ignore covered by the e2e suite
func Ignored(s string) string { return s }
`)
	write("say.go", "package text\n\nimport \"fmt\"\n\nfunc say(s string) string { fmt.Println(s); return s }\n")
	write("broken.go", "package text\n\nfunc {\n")
	write("text_test.go", `package text_test

import (
	"strings"
	"testing"

	"example.com/text"
)

func TestX(t *testing.T) {
	_ = strings.Split("a b", " ")
	_ = text.Upper("a")
}
`)

	tree, err := Scan(dir, Options{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(tree.Broken) != 1 || filepath.Base(tree.Broken[0].Path) != "broken.go" {
		t.Errorf("broken = %+v, want broken.go only", tree.Broken)
	}
	if !tree.HasTests(dir) {
		t.Error("HasTests = false, want true")
	}

	var names []string
	for _, file := range tree.Untested(true) {
		for _, match := range file.Matches {
			names = append(names, match.Name)
		}
	}
	if want := []string{"Split"}; !slices.Equal(names, want) {
		t.Errorf("untested = %v, want %v", names, want)
	}
}
