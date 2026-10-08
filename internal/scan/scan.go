package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
)

type File struct {
	Path            string
	Package         string
	BuildConstraint string // e.g. "//go:build linux", empty if the file always builds
	Matches         []detect.Match
	Hints           []detect.Hint
}

type identSet map[string]bool

// Yawntest files count as tests only with countYawntest; generation rewrites them anyway.
func Untested(root string, countYawntest bool) ([]File, error) {
	fset := token.NewFileSet()
	var files []File
	testIdentsByDir := map[string]identSet{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		switch {
		case IsYawntestFile(path) && !countYawntest:
			return nil
		case strings.HasSuffix(path, "_test.go"):
			dir := filepath.Dir(path)
			if testIdentsByDir[dir] == nil {
				testIdentsByDir[dir] = identSet{}
			}
			collectIdents(file, testIdentsByDir[dir])
		case !ast.IsGenerated(file):
			files = append(files, File{
				Path:            path,
				Package:         file.Name.Name,
				BuildConstraint: BuildConstraint(path, file),
				Matches:         detect.File(fset, file),
				Hints:           detect.Hints(fset, file),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return withoutTested(files, testIdentsByDir), nil
}

func HasMatches(files []File) bool {
	for _, file := range files {
		if len(file.Matches) > 0 {
			return true
		}
	}
	return false
}

// Dirs returns every directory below root holding a file whose name passes keep.
func Dirs(root string, keep func(fileName string) bool) ([]string, error) {
	var dirs []string
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		dir := filepath.Dir(path)
		if !found[dir] && keep(entry.Name()) {
			found[dir] = true
			dirs = append(dirs, dir)
		}
		return nil
	})
	return dirs, err
}

func IsYawntestFile(name string) bool {
	return strings.HasSuffix(name, "_yawntest_test.go")
}

func IsSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func skipDir(name string) bool {
	return name == "vendor" ||
		name == "testdata" ||
		strings.HasPrefix(name, ".") ||
		strings.HasPrefix(name, "_")
}

func collectIdents(file *ast.File, idents identSet) {
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok {
			idents[ident.Name] = true
		}
		return true
	})
}

// ponytail: any name match counts as tested; coverage (M4) is precise.
func withoutTested(files []File, testIdentsByDir map[string]identSet) []File {
	var untested []File
	for _, file := range files {
		testIdents := testIdentsByDir[filepath.Dir(file.Path)]

		var matches []detect.Match
		for _, match := range file.Matches {
			if !testIdents[match.Symbol()] {
				matches = append(matches, match)
			}
		}

		file.Matches = matches
		if len(matches) > 0 || len(file.Hints) > 0 {
			untested = append(untested, file)
		}
	}
	return untested
}
