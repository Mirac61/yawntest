package run

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

type ErrorPath struct {
	File string
	Line int
	Code string // the return statement, first line only
}

// UntestedErrorPaths runs each dir's tests with coverage and returns the error returns that never ran.
func UntestedErrorPaths(dirs []string) ([]ErrorPath, error) {
	var paths []ErrorPath
	for _, dir := range dirs {
		blocks, err := coverProfile(dir)
		if err != nil {
			return nil, err
		}
		dirPaths, err := uncoveredErrorReturns(dir, blocks)
		if err != nil {
			return nil, err
		}
		paths = append(paths, dirPaths...)
	}
	return paths, nil
}

type coverBlock struct {
	file                string // base name, the profile uses import paths
	startLine, startCol int
	endLine, endCol     int
	count               int
}

func (b coverBlock) contains(position token.Position) bool {
	afterStart := position.Line > b.startLine || position.Line == b.startLine && position.Column >= b.startCol
	beforeEnd := position.Line < b.endLine || position.Line == b.endLine && position.Column <= b.endCol
	return afterStart && beforeEnd
}

// Failing tests still write a profile, so only a missing profile is an error. Tests that don't
// build write one too, but without blocks, which would make every error return look reached.
func coverProfile(dir string, testArgs ...string) ([]coverBlock, error) {
	profile, err := os.CreateTemp("", "yawntest-*.cover")
	if err != nil {
		return nil, err
	}
	profile.Close()
	defer os.Remove(profile.Name())

	args := append([]string{"test", "-coverprofile=" + profile.Name()}, testArgs...)
	cmd := exec.Command("go", append(args, ".")...)
	cmd.Dir = dir
	output, runErr := cmd.CombinedOutput()

	data, err := os.ReadFile(profile.Name())
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("coverage for %s: %v\n%s", dir, runErr, output)
	}
	blocks, err := parseProfile(data)
	if err != nil {
		return nil, err
	}
	if runErr != nil && len(blocks) == 0 {
		return nil, fmt.Errorf("coverage for %s: %v\n%s", dir, runErr, output)
	}
	return blocks, nil
}

// Lines look like: example.com/pkg/file.go:9.2,10.16 2 1
func parseProfile(data []byte) ([]coverBlock, error) {
	var blocks []coverBlock
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			return nil, fmt.Errorf("bad coverage line %q", line)
		}

		block := coverBlock{file: path.Base(line[:colon])}
		var statements int
		_, err := fmt.Sscanf(line[colon+1:], "%d.%d,%d.%d %d %d",
			&block.startLine, &block.startCol, &block.endLine, &block.endCol, &statements, &block.count)
		if err != nil {
			return nil, fmt.Errorf("bad coverage line %q: %w", line, err)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func uncoveredErrorReturns(dir string, blocks []coverBlock) ([]ErrorPath, error) {
	sourceFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}

	var paths []ErrorPath
	for _, sourcePath := range sourceFiles {
		if strings.HasSuffix(sourcePath, "_test.go") {
			continue
		}
		filePaths, err := uncoveredInFile(sourcePath, blocks)
		if err != nil {
			return nil, err
		}
		paths = append(paths, filePaths...)
	}
	return paths, nil
}

func uncoveredInFile(sourcePath string, blocks []coverBlock) ([]ErrorPath, error) {
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourcePath, source, 0)
	if err != nil {
		return nil, err
	}

	var paths []ErrorPath
	ast.Inspect(file, func(node ast.Node) bool {
		ret, ok := node.(*ast.ReturnStmt)
		if !ok || !returnsErr(ret) {
			return true
		}

		start, end := fset.Position(ret.Pos()), fset.Position(ret.End())
		if neverRan(start, filepath.Base(sourcePath), blocks) {
			paths = append(paths, ErrorPath{
				File: sourcePath,
				Line: start.Line,
				Code: firstLine(string(source[start.Offset:end.Offset])),
			})
		}
		return true
	})
	return paths, nil
}

// returnsErr matches `return err`, `return x, err` and wrapped forms like fmt.Errorf("...: %w", err).
func returnsErr(ret *ast.ReturnStmt) bool {
	if len(ret.Results) == 0 {
		return false
	}

	last := ret.Results[len(ret.Results)-1]
	if isErrIdent(last) {
		return true
	}
	call, ok := last.(*ast.CallExpr)
	if !ok {
		return false
	}
	for _, arg := range call.Args {
		if isErrIdent(arg) {
			return true
		}
	}
	return false
}

func isErrIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "err"
}

// Statements outside every block weren't instrumented and don't count.
func neverRan(position token.Position, fileName string, blocks []coverBlock) bool {
	instrumented := false
	for _, block := range blocks {
		if block.file != fileName || !block.contains(position) {
			continue
		}
		if block.count > 0 {
			return false
		}
		instrumented = true
	}
	return instrumented
}

// WithoutCovered drops func matches whose body already runs in the package's own tests. The
// name check in scan misses funcs that tests only reach indirectly, e.g. through a router.
func WithoutCovered(files []scan.File) ([]scan.File, error) {
	blocksByDir := map[string][]coverBlock{}
	var kept []scan.File
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		blocks, seen := blocksByDir[dir]
		if !seen && len(file.Matches) > 0 {
			var err error
			if blocks, err = ownTestCoverage(dir); err != nil {
				return nil, err
			}
			blocksByDir[dir] = blocks
		}

		var matches []detect.Match
		for _, match := range file.Matches {
			if !ran(blocks, filepath.Base(file.Path), match.Line, match.EndLine) {
				matches = append(matches, match)
			}
		}
		file.Matches = matches
		if len(matches) > 0 || len(file.Hints) > 0 {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

// ownTestCoverage skips yawntest's tests, so --force still rewrites their files. Without
// user-written tests, or when they don't build, it returns nil and the names decide alone.
func ownTestCoverage(dir string) ([]coverBlock, error) {
	tests, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	hasOwnTests := slices.ContainsFunc(tests, func(path string) bool { return !scan.IsYawntestFile(path) })
	if !hasOwnTests {
		return nil, nil
	}

	blocks, err := coverProfile(dir, "-skip", "Yawntest")
	if err != nil {
		return nil, nil
	}
	return blocks, nil
}

func ran(blocks []coverBlock, fileName string, startLine, endLine int) bool {
	return slices.ContainsFunc(blocks, func(block coverBlock) bool {
		return block.file == fileName && block.count > 0 && block.startLine <= endLine && block.endLine >= startLine
	})
}
