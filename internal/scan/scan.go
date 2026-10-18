package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
)

type File struct {
	Path            string
	Package         string
	BuildConstraint string // e.g. "//go:build linux", empty if the file always builds
	Matches         []detect.Match
	Hints           []detect.Hint
	ErrorReturns    []detect.ErrorReturn
}

// Broken is a file that doesn't parse. It's skipped, the rest of the tree is still checked.
type Broken struct {
	Path string
	Err  error
}

// Tree is one parse of every Go file below a root.
type Tree struct {
	Files  []File // sources, generated files left out, in walk order
	Broken []Broken

	testIdents     map[string]identSet // per dir, from user-written tests
	yawntestIdents map[string]identSet // per dir, from *_yawntest_test.go
}

type identSet map[string]bool

// Scan parses root; exclude, if set, leaves out files and whole dirs it returns true for.
func Scan(root string, exclude func(path string) bool) (Tree, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if exclude != nil && path != root && exclude(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && skipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return Tree{}, err
	}

	tree := Tree{testIdents: map[string]identSet{}, yawntestIdents: map[string]identSet{}}
	order := map[string]int{}
	for i, path := range paths {
		order[path] = i
	}
	for _, dirPaths := range byDir(paths) {
		if err := tree.addDir(dirPaths); err != nil {
			return Tree{}, err
		}
	}
	slices.SortFunc(tree.Files, func(a, b File) int { return order[a.Path] - order[b.Path] })
	return tree, nil
}

// byDir groups paths by directory, dirs in order of their first file.
func byDir(paths []string) [][]string {
	var groups [][]string
	index := map[string]int{}
	for _, path := range paths {
		dir := filepath.Dir(path)
		i, seen := index[dir]
		if !seen {
			i = len(groups)
			index[dir] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], path)
	}
	return groups
}

type parsedFile struct {
	path string
	src  []byte
	ast  *ast.File
}

// addDir parses one directory as a package, so detection sees helpers and consts across files.
func (t *Tree) addDir(paths []string) error {
	fset := token.NewFileSet()
	var sources []parsedFile
	var packageFiles []*ast.File
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Broken = append(t.Broken, Broken{Path: path, Err: err})
			continue
		}

		dir := filepath.Dir(path)
		switch {
		case IsYawntestFile(path):
			collectIdents(file, t.idents(t.yawntestIdents, dir))
		case strings.HasSuffix(path, "_test.go"):
			collectIdents(file, t.idents(t.testIdents, dir))
		default:
			packageFiles = append(packageFiles, file)
			if !ast.IsGenerated(file) {
				sources = append(sources, parsedFile{path: path, src: src, ast: file})
			}
		}
	}

	pkg := detect.NewPackage(packageFiles)
	for _, source := range sources {
		ignored := detect.Ignores(fset, source.ast)
		t.Files = append(t.Files, File{
			Path:            source.path,
			Package:         source.ast.Name.Name,
			BuildConstraint: BuildConstraint(source.path, source.ast),
			Matches:         unignored(pkg.File(fset, source.ast), ignored, func(m detect.Match) int { return m.Line }),
			Hints:           unignored(detect.Hints(fset, source.ast), ignored, func(h detect.Hint) int { return h.Line }),
			ErrorReturns:    unignored(pkg.ErrorReturns(fset, source.ast, source.src), ignored, func(r detect.ErrorReturn) int { return r.Line }),
		})
	}
	return nil
}

func (t *Tree) idents(byDir map[string]identSet, dir string) identSet {
	if byDir[dir] == nil {
		byDir[dir] = identSet{}
	}
	return byDir[dir]
}

func unignored[T any](items []T, ignored detect.Ignored, line func(T) int) []T {
	var kept []T
	for _, item := range items {
		if !ignored(line(item)) {
			kept = append(kept, item)
		}
	}
	return kept
}

// Untested returns the files with untested matches or with hints. Yawntest files count as
// tests only with countYawntest; generation rewrites them anyway.
func (t Tree) Untested(countYawntest bool) []File {
	var untested []File
	for _, file := range t.Files {
		dir := filepath.Dir(file.Path)
		var matches []detect.Match
		for _, match := range file.Matches {
			symbol := match.Symbol()
			tested := t.testIdents[dir][symbol] || countYawntest && t.yawntestIdents[dir][symbol]
			if !tested {
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

// HasTests reports whether dir has test files, yawntest's own included.
func (t Tree) HasTests(dir string) bool {
	return len(t.testIdents[dir]) > 0 || len(t.yawntestIdents[dir]) > 0
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

// collectIdents adds the names a test file refers to. strings.Split doesn't count as a
// mention of the package's own Split; only selectors on the package under test do.
func collectIdents(file *ast.File, idents identSet) {
	underTest := strings.TrimSuffix(file.Name.Name, "_test")
	imports := detect.ImportNames(file)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.SelectorExpr:
			ident, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, isImport := imports[ident.Name]; isImport && ident.Name != underTest {
				return false
			}
		case *ast.Ident:
			idents[node.Name] = true
		}
		return true
	})
}
