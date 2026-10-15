package run

import (
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

// yawntestTests matches the test funcs gen writes, and only those.
const yawntestTests = "^(Test|Fuzz)Yawntest_"

type ErrorPath struct {
	File string
	Line int
	Func string // the func the return is in
	Code string // the return statement, first line only
}

// Coverage runs a directory's tests with a cover profile at most once per variant and hands the
// blocks to whoever asks. check needs them twice, for error paths and for tested funcs.
type Coverage struct {
	Progress Progress // optional

	mu       sync.Mutex
	profiles map[coverKey]*profile
}

// Progress is told how many of a batch of go test runs are done, for a status line.
type Progress func(done, total int)

type coverKey struct {
	dir          string
	skipYawntest bool
}

type profile struct {
	once   sync.Once
	blocks []coverBlock
	err    error
}

func NewCoverage() *Coverage {
	return &Coverage{profiles: map[coverKey]*profile{}}
}

func (c *Coverage) blocks(dir string, skipYawntest bool) ([]coverBlock, error) {
	key := coverKey{dir: filepath.Clean(dir), skipYawntest: skipYawntest}
	c.mu.Lock()
	p := c.profiles[key]
	if p == nil {
		p = &profile{}
		c.profiles[key] = p
	}
	c.mu.Unlock()

	p.once.Do(func() {
		var args []string
		if skipYawntest {
			args = []string{"-skip", yawntestTests}
		}
		p.blocks, p.err = coverProfile(dir, args...)
	})
	return p.blocks, p.err
}

// prefetch fills the profiles of dirs in parallel; go test of one package rarely uses every core.
func (c *Coverage) prefetch(dirs []string, skipYawntest bool) {
	var todo []string
	c.mu.Lock()
	for _, dir := range dirs {
		if c.profiles[coverKey{dir: filepath.Clean(dir), skipYawntest: skipYawntest}] == nil {
			todo = append(todo, dir)
		}
	}
	c.mu.Unlock()
	parallel(todo, c.Progress, func(dir string) { c.blocks(dir, skipYawntest) })
}

// parallel calls do once per item, a few at a time, and reports each finished one to progress.
func parallel[T any](items []T, progress Progress, do func(T)) {
	limit := make(chan struct{}, min(max(runtime.NumCPU()/2, 2), 8))
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	if progress != nil && len(items) > 0 {
		progress(0, len(items))
	}
	for _, item := range items {
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-limit }()
			do(item)
			if progress != nil {
				mu.Lock()
				done++
				progress(done, len(items))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

// ErrorPaths is the error-path part of check.
type ErrorPaths struct {
	Paths    []ErrorPath
	NoTests  []string // dirs with error returns but no tests at all; listing each return would be noise
	Warnings []string // dirs whose tests don't build, so their error paths are unknown
}

// UntestedErrorPaths runs each dir's tests with coverage and returns the error returns that never ran.
func UntestedErrorPaths(files []scan.File, hasTests func(dir string) bool, coverage *Coverage) ErrorPaths {
	var result ErrorPaths
	var dirs []string
	seen := map[string]bool{}
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if len(file.ErrorReturns) == 0 || seen[dir] {
			continue
		}
		seen[dir] = true
		if hasTests(dir) {
			dirs = append(dirs, dir)
		} else {
			result.NoTests = append(result.NoTests, dir)
		}
	}
	coverage.prefetch(dirs, false)

	failed := map[string]bool{}
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if len(file.ErrorReturns) == 0 || !hasTests(dir) || failed[dir] {
			continue
		}
		blocks, err := coverage.blocks(dir, false)
		if err != nil {
			failed[dir] = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: error paths skipped, %v", dir, err))
			continue
		}
		for _, ret := range file.ErrorReturns {
			position := token.Position{Line: ret.Line, Column: ret.Column}
			if neverRan(position, filepath.Base(file.Path), blocks) {
				result.Paths = append(result.Paths, ErrorPath{File: file.Path, Line: ret.Line, Func: ret.Func, Code: ret.Code})
			}
		}
	}
	return result
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
		//yawntest:ignore needs a go tool that writes no profile at all
		return nil, fmt.Errorf("tests don't run: %s", goOutputLine(string(output), runErr))
	}
	blocks, err := parseProfile(data)
	if err != nil {
		return nil, err
	}
	if runErr != nil && len(blocks) == 0 {
		return nil, fmt.Errorf("tests don't build: %s", goOutputLine(string(output), runErr))
	}
	return blocks, nil
}

// goOutputLine is the first line of go's output that says something, or the error itself.
func goOutputLine(output string, err error) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "FAIL") {
			return line
		}
	}
	return fmt.Sprint(err)
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

// WithoutCovered drops func matches whose body already runs in the package's tests. The name
// check in scan misses funcs that tests only reach indirectly, e.g. through a router.
//
// gen skips yawntest's own tests, so --force still rewrites their files. check counts them:
// they are tests like any other, and the run is then shared with the error paths.
// Without tests, or when they don't build, the names decide alone.
func WithoutCovered(files []scan.File, coverage *Coverage, skipYawntest bool) []scan.File {
	var dirs []string
	seen := map[string]bool{}
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if len(file.Matches) > 0 && !seen[dir] && hasTests(dir, skipYawntest) {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	coverage.prefetch(dirs, skipYawntest)

	var kept []scan.File
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		var blocks []coverBlock
		if seen[dir] {
			blocks, _ = coverage.blocks(dir, skipYawntest)
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
	return kept
}

func hasTests(dir string, skipYawntest bool) bool {
	tests, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	return slices.ContainsFunc(tests, func(path string) bool {
		return !skipYawntest || !scan.IsYawntestFile(path)
	})
}

func ran(blocks []coverBlock, fileName string, startLine, endLine int) bool {
	return slices.ContainsFunc(blocks, func(block coverBlock) bool {
		return block.file == fileName && block.count > 0 && block.startLine <= endLine && block.endLine >= startLine
	})
}
