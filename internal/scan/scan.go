package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Mirac61/lazytest/internal/detect"
)

type File struct {
	Path    string
	Package string
	Matches []detect.Match
}

type identSet map[string]bool

// Lazytest files count as tests only with countLazytest; generation rewrites them anyway.
func Untested(root string, countLazytest bool) ([]File, error) {
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
		case strings.HasSuffix(path, "_lazytest_test.go") && !countLazytest:
			return nil
		case strings.HasSuffix(path, "_test.go"):
			dir := filepath.Dir(path)
			if testIdentsByDir[dir] == nil {
				testIdentsByDir[dir] = identSet{}
			}
			collectIdents(file, testIdentsByDir[dir])
		case !ast.IsGenerated(file):
			files = append(files, File{
				Path:    path,
				Package: file.Name.Name,
				Matches: detect.File(fset, file),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return withoutTested(files, testIdentsByDir), nil
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

		if len(matches) > 0 {
			file.Matches = matches
			untested = append(untested, file)
		}
	}
	return untested
}
