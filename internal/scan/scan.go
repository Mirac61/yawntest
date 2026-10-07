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

type identSet map[string]bool

func Untested(root string) ([]detect.Match, error) {
	fset := token.NewFileSet()
	var matches []detect.Match
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
		case strings.HasSuffix(path, "_test.go"):
			dir := filepath.Dir(path)
			if testIdentsByDir[dir] == nil {
				testIdentsByDir[dir] = identSet{}
			}
			collectIdents(file, testIdentsByDir[dir])
		case !ast.IsGenerated(file):
			matches = append(matches, detect.File(fset, file)...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return withoutTested(matches, testIdentsByDir), nil
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
func withoutTested(matches []detect.Match, testIdentsByDir map[string]identSet) []detect.Match {
	var untested []detect.Match
	for _, match := range matches {
		testIdents := testIdentsByDir[filepath.Dir(match.File)]
		if !testIdents[match.Symbol()] {
			untested = append(untested, match)
		}
	}
	return untested
}
