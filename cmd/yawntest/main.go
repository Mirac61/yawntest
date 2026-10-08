package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Mirac61/yawntest/internal/changes"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/report"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

const (
	exitProblems = 1 // see report.Result.HasProblems; lets CI fail
	exitError    = 2
)

type options struct {
	root     string
	check    bool
	force    bool
	runTests bool
	changed  bool
	json     bool
}

func main() {
	opts := parseFlags()
	result := collect(opts)
	if opts.json {
		mustReport(report.JSON(os.Stdout, result))
	} else {
		printText(opts, result)
	}
	if result.HasProblems() {
		os.Exit(exitProblems)
	}
}

func parseFlags() options {
	check := flag.Bool("check", false, "report untested patterns without writing files")
	force := flag.Bool("force", false, "regenerate existing yawntest files")
	runTests := flag.Bool("run", false, "run the generated tests afterwards and report failures as findings")
	changed := flag.Bool("changed", false, "only look at code changed since the last commit")
	jsonOutput := flag.Bool("json", false, "print the result as JSON")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: yawntest [--check] [--force] [--run] [--changed] [--json] [path]")
		flag.PrintDefaults()
	}
	flag.Parse()

	return options{
		root:     rootArg(),
		check:    *check,
		force:    *force,
		runTests: *runTests,
		changed:  *changed,
		json:     *jsonOutput,
	}
}

func collect(opts options) report.Result {
	if opts.check {
		return check(opts)
	}

	result := report.Result{Generated: generate(opts)}
	if opts.runTests {
		dirs := must(scan.Dirs(opts.root, scan.IsYawntestFile))
		result.Findings = must(run.YawntestTests(dirs))
	}
	return result
}

func printText(opts options, result report.Result) {
	if opts.check {
		mustReport(report.Untested(os.Stdout, result.Untested))
		mustReport(report.Hints(os.Stdout, result.Untested))
		mustReport(report.ErrorPaths(os.Stdout, result.ErrorPaths))
		return
	}

	mustReport(report.Generated(os.Stdout, result.Generated))
	if opts.runTests {
		mustReport(report.Findings(os.Stdout, result.Findings))
	}
}

func check(opts options) report.Result {
	files := must(scan.Untested(opts.root, true))
	sourceDirs := must(scan.Dirs(opts.root, scan.IsSourceFile))
	errorPaths := must(run.UntestedErrorPaths(sourceDirs))
	if opts.changed {
		changed := must(changes.SinceHEAD(opts.root))
		files = must(changes.KeepChangedMatches(files, changed))
		errorPaths = must(changes.KeepChangedErrorPaths(errorPaths, changed))
	}
	files = must(run.WithoutCovered(files))
	return report.Result{Untested: files, ErrorPaths: errorPaths}
}

// Without --force existing yawntest files count as tests, so only sources with new candidates show up.
func generate(opts options) []report.GeneratedFile {
	files := must(scan.Untested(opts.root, !opts.force))
	if opts.changed {
		changed := must(changes.SinceHEAD(opts.root))
		files = must(changes.KeepChangedFiles(files, changed))
	}
	files = must(run.WithoutCovered(files))

	var results []report.GeneratedFile
	for _, file := range files {
		if len(file.Matches) > 0 {
			results = append(results, must(generateFile(file, opts.force)))
		}
	}
	return results
}

func generateFile(file scan.File, force bool) (report.GeneratedFile, error) {
	result := report.GeneratedFile{Source: file.Path, TestPath: gen.TestPath(file.Path)}

	output, err := gen.File(file.Package, file.BuildConstraint, file.Matches)
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

// Every error in a CLI run is fatal, so must ends the program instead of returning.
func must[T any](value T, err error) T {
	if err != nil {
		fail(err)
	}
	return value
}

func mustReport(err error) {
	if err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "yawntest:", err)
	os.Exit(exitError)
}

func rootArg() string {
	if flag.NArg() == 0 {
		return "."
	}
	return strings.TrimSuffix(flag.Arg(0), "/...")
}
