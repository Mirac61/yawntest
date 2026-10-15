// Package load gives detection type information with nothing but the standard library: go list
// compiles the dependencies and names their export data, and go/types checks the scanned
// sources against it. That's what golang.org/x/tools/go/packages does, minus the dependency.
package load

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Loader type-checks the packages of the modules below a root.
type Loader struct {
	packages map[string]listed // by clean absolute dir, every package of the listed modules
	dirs     map[string]string // import path → dir, for the same packages
	exports  map[string]string // import path → export data file

	mu       sync.Mutex
	importer types.Importer // shared, so every dependency's export data is read once
	fset     *token.FileSet
}

type listed struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Export     string
	Standard   bool
	DepOnly    bool
	ImportMap  map[string]string
	Module     *struct{ Main bool }
	Error      *struct{ Err string }
}

// New lists the module that holds root and every module nested below it. A module whose
// packages can't be listed at all becomes a warning; detection then works without types there.
func New(root string) (*Loader, []string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	loader := &Loader{packages: map[string]listed{}, dirs: map[string]string{}, exports: map[string]string{}}

	var warnings []string
	for _, module := range modules(absolute) {
		pattern := "./..."
		if rel, err := filepath.Rel(module, absolute); err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			pattern = "./" + filepath.ToSlash(rel) + "/..." // root is inside this module: list only that part
		}
		if err := loader.list(module, pattern); err != nil {
			warnings = append(warnings, fmt.Sprintf("no type information for %s, using syntax only: %v", module, err))
		}
	}
	return loader, warnings, nil
}

// modules is the module around root plus those nested in it, skipping what the go tool skips.
func modules(root string) []string {
	var dirs []string
	if gomod, err := goEnv(root, "GOMOD"); err == nil && gomod != "" && gomod != os.DevNull {
		dirs = append(dirs, filepath.Dir(gomod))
	}
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "go.mod" && filepath.Dir(path) != root {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	return dirs
}

func goEnv(dir, name string) (string, error) {
	cmd := exec.Command("go", "env", name)
	cmd.Dir = dir
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

// list runs go list -export, which builds the export data of every dependency (cached by go).
// -e keeps going past broken packages; they come back with Error set.
func (l *Loader) list(module, pattern string) error {
	cmd := exec.Command("go", "list", "-e", "-deps", "-export",
		"-json=ImportPath,Dir,GoFiles,Export,Standard,DepOnly,ImportMap,Module,Error", pattern)
	cmd.Dir = module
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	count, err := l.add(&stdout)
	if err != nil {
		return err
	}
	if count == 0 && runErr != nil {
		return fmt.Errorf("%v: %s", runErr, firstLine(stderr.String()))
	}
	return nil
}

// add reads go list's stream of JSON objects and returns how many packages it held.
func (l *Loader) add(r io.Reader) (int, error) {
	decoder := json.NewDecoder(r)
	count := 0
	for {
		var pkg listed
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			return count, nil
		}
		if err != nil {
			return count, fmt.Errorf("read go list output: %w", err)
		}
		count++
		if pkg.Export != "" {
			l.exports[pkg.ImportPath] = pkg.Export
		}
		// The module's own packages, also those only reached as dependencies of the pattern:
		// checking ./api/... still needs to see into ./db when api calls it.
		ownModule := pkg.Module != nil && pkg.Module.Main
		if (ownModule || !pkg.DepOnly) && !pkg.Standard && pkg.Dir != "" {
			dir := filepath.Clean(pkg.Dir)
			l.packages[dir] = pkg
			l.dirs[pkg.ImportPath] = dir
		}
	}
}

// Dir is where the package with this import path lives, for packages of the listed modules.
func (l *Loader) Dir(path string) (string, bool) {
	dir, ok := l.dirs[path]
	return dir, ok
}

// Check type-checks the files of dir that the current build includes. Files of other build
// configurations (open_windows.go on Linux) get no type information. Type errors don't stop
// it: whatever could be resolved is in the Info, and detection falls back to syntax for the rest.
func (l *Loader) Check(dir string, fset *token.FileSet, files []*ast.File) (string, *types.Info, bool) {
	// go list reports absolute dirs; the scan walks the path as given, often "./api".
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, false
	}
	pkg, ok := l.packages[absolute]
	if !ok {
		return "", nil, false
	}
	built := map[string]bool{}
	for _, name := range pkg.GoFiles {
		built[name] = true
	}
	var checked []*ast.File
	for _, file := range files {
		if built[filepath.Base(fset.Position(file.Pos()).Filename)] {
			checked = append(checked, file)
		}
	}
	if len(checked) == 0 {
		return "", nil, false
	}

	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Types:      map[ast.Expr]types.TypeAndValue{},
	}
	config := types.Config{
		Importer: mappedImporter{loader: l, fset: fset, importMap: pkg.ImportMap},
		Error:    func(error) {}, // keep going; see above
	}
	config.Check(pkg.ImportPath, fset, checked, info)
	return pkg.ImportPath, info, true
}

// mappedImporter turns source import paths into canonical ones (vendoring) before the gc
// importer reads their export data.
type mappedImporter struct {
	loader    *Loader
	fset      *token.FileSet
	importMap map[string]string
}

func (m mappedImporter) Import(path string) (*types.Package, error) {
	if canonical, ok := m.importMap[path]; ok {
		path = canonical
	}
	l := m.loader
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.importer == nil || l.fset != m.fset {
		l.fset = m.fset
		l.importer = importer.ForCompiler(m.fset, "gc", func(path string) (io.ReadCloser, error) {
			file, ok := l.exports[path]
			if !ok {
				return nil, fmt.Errorf("no export data for %s", path)
			}
			return os.Open(file)
		})
	}
	return l.importer.Import(path)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
