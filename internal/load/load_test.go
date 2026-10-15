package load

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Mirac61/yawntest/internal/scan"
)

// a calls into b. Only type information can tell what those calls do.
var module = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.22\n",
	"b/b.go": `package b

import (
	"os"
	"strconv"
)

func Read(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func Atoi(s string) (int, error) { return strconv.Atoi(s) }

type Client struct{}

func (Client) Get() error {
	_, err := os.Stat("x")
	return err
}
`,
	"a/a.go": `package a

import (
	"time"

	"example.com/m/b"
)

// Pure looks pure, but b.Read reads a file.
func Pure(s string) string { return b.Read(s) }

// Parse passes on an error that only depends on s: a test can reach it.
func Parse(s string) (int, error) {
	n, err := b.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Fetch passes on an error from the file system: a unit test can't reach it.
func Fetch() error {
	if err := (b.Client{}).Get(); err != nil {
		return err
	}
	return nil
}

// Later calls the method time.Time.After, a comparison, not the timer func time.After.
func Later(x, y time.Time) bool { return x.After(y) }
`,
	// Not in the current build: checked from syntax alone, without failing the package.
	"a/a_other.go": "//go:build never\n\npackage a\n\nfunc Other(s string) string { return s }\n",
	// Imports a package go list never built: Check still gives what it could resolve.
	"c/c.go": "package c\n\nimport \"example.com/m/gone\"\n\nfunc Len(s string) int { return len(s) + gone.N }\n",
}

func TestTypesFollowCallsAcrossPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list")
	}
	root := t.TempDir()
	for name, content := range module {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Only a is checked, the way `yawntest check ./a/...` would; b is reached through a's calls.
	loader, warnings, err := New(filepath.Join(root, "a"))
	if err != nil || len(warnings) > 0 {
		t.Fatalf("New = %v, %v", warnings, err)
	}

	// c's import has no export data: the import fails, the rest is still typed.
	cDir := filepath.Join(root, "c")
	cLoader, _, _ := New(cDir)
	fset := token.NewFileSet()
	cFile, err := parser.ParseFile(fset, filepath.Join(cDir, "c.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if path, info, ok := cLoader.Check(cDir, fset, []*ast.File{cFile}); !ok || path != "example.com/m/c" || len(info.Defs) == 0 {
		t.Errorf("Check(c) = %q, %v, %v; want partial types", path, info, ok)
	}

	tests := []struct {
		name         string
		options      scan.Options
		candidates   []string
		errorReturns []string
	}{
		{
			name:       "syntax only",
			options:    scan.Options{},
			candidates: []string{"Pure", "Parse", "Later", "Other"},
		},
		{
			name:         "with types",
			options:      scan.Options{Types: loader},
			candidates:   []string{"Parse", "Later", "Other"},
			errorReturns: []string{"return 0, err"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree, err := scan.Scan(filepath.Join(root, "a"), test.options)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			var candidates, errorReturns []string
			for _, file := range tree.Files {
				for _, match := range file.Matches {
					candidates = append(candidates, match.Name)
				}
				for _, ret := range file.ErrorReturns {
					errorReturns = append(errorReturns, ret.Code)
				}
			}
			if !slices.Equal(candidates, test.candidates) {
				t.Errorf("candidates = %v, want %v", candidates, test.candidates)
			}
			if !slices.Equal(errorReturns, test.errorReturns) {
				t.Errorf("error returns = %q, want %q", errorReturns, test.errorReturns)
			}
		})
	}
}

// A module that doesn't list is a warning, not a failure: the scan goes on without types.
func TestNewWithBrokenModule(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("this is not a go.mod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader, warnings, err := New(dir)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("New = %v, %v; want one warning", warnings, err)
	}
	if _, _, ok := loader.Check(dir, nil, nil); ok {
		t.Error("Check of an unlisted dir = ok")
	}
}
