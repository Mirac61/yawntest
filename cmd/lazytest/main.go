package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Mirac61/lazytest/internal/changes"
	"github.com/Mirac61/lazytest/internal/gen"
	"github.com/Mirac61/lazytest/internal/report"
	"github.com/Mirac61/lazytest/internal/run"
	"github.com/Mirac61/lazytest/internal/scan"
)

const (
	exitProblems = 1 // untested candidates with --check, findings with --run; lets CI fail
	exitError    = 2
)

type options struct {
	root    string
	force   bool
	changed bool
}

func main() {
	check := flag.Bool("check", false, "report untested patterns without writing files")
	force := flag.Bool("force", false, "regenerate existing lazytest files")
	runTests := flag.Bool("run", false, "run the generated tests afterwards and report failures as findings")
	changed := flag.Bool("changed", false, "only look at code changed since the last commit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: lazytest [--check] [--force] [--run] [--changed] [path]")
		flag.PrintDefaults()
	}
	flag.Parse()

	opts := options{root: rootArg(), force: *force, changed: *changed}
	if *check {
		runCheck(opts)
		return
	}
	runGenerate(opts)
	if *runTests {
		runFindings(opts.root)
	}
}

func runCheck(opts options) {
	files, err := scan.Untested(opts.root, true)
	if err != nil {
		fail(err)
	}
	if opts.changed {
		files = onlyChanged(opts.root, files, changes.KeepChangedMatches)
	}

	if err := report.Untested(os.Stdout, files); err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
	if len(files) > 0 {
		os.Exit(exitProblems)
	}
}

// Without --force existing lazytest files count as tests, so only sources with new candidates show up.
func runGenerate(opts options) {
	files, err := scan.Untested(opts.root, !opts.force)
	if err != nil {
		fail(err)
	}
	if opts.changed {
		files = onlyChanged(opts.root, files, changes.KeepChangedFiles)
	}

	var results []report.GeneratedFile
	for _, file := range files {
		result, err := generateFile(file, opts.force)
		if err != nil {
			fail(err)
		}
		results = append(results, result)
	}

	if err := report.Generated(os.Stdout, results); err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
}

type changeFilter func([]scan.File, map[string]changes.File) ([]scan.File, error)

func onlyChanged(root string, files []scan.File, keep changeFilter) []scan.File {
	changed, err := changes.SinceHEAD(root)
	if err != nil {
		fail(err)
	}
	kept, err := keep(files, changed)
	if err != nil {
		fail(err)
	}
	return kept
}

func runFindings(root string) {
	dirs, err := scan.Dirs(root, scan.IsLazytestFile)
	if err != nil {
		fail(err)
	}
	findings, err := run.LazytestTests(dirs)
	if err != nil {
		fail(err)
	}
	if err := report.Findings(os.Stdout, findings); err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
	if len(findings) > 0 {
		os.Exit(exitProblems)
	}
}

func generateFile(file scan.File, force bool) (report.GeneratedFile, error) {
	result := report.GeneratedFile{Source: file.Path, TestPath: gen.TestPath(file.Path)}

	output, err := gen.File(file.Package, file.Matches)
	if err != nil {
		return result, fmt.Errorf("generate %s: %w", file.Path, err)
	}
	result.Output = output
	if output.Code == nil {
		return result, nil
	}

	exists, err := fileExists(result.TestPath)
	if err != nil {
		return result, err
	}
	if exists && !force {
		return result, nil
	}

	if err := os.WriteFile(result.TestPath, output.Code, 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", result.TestPath, err)
	}
	result.Written = true
	return result, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazytest:", err)
	os.Exit(exitError)
}

func rootArg() string {
	if flag.NArg() == 0 {
		return "."
	}
	return strings.TrimSuffix(flag.Arg(0), "/...")
}
