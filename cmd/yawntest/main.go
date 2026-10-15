package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"runtime/debug"
	"slices"

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
	command string // check, gen or run
	root    string
	force   bool
	changed bool
	since   string // git ref; like changed, but since the merge base with it
	static  bool   // run no tests
	format  string // text, json or github
	json    bool   // --json, folded into format by parseArgs
}

// filtered reports whether only changed code counts.
func (o options) filtered() bool {
	return o.changed || o.since != ""
}

var versionArgs = []string{"version", "-version", "--version"}

// version is the module version go install stamped into the binary, "(devel)" for other builds.
func version() string {
	info, ok := debug.ReadBuildInfo()
	return versionOf(info, ok)
}

func versionOf(info *debug.BuildInfo, ok bool) string {
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}

func main() {
	if len(os.Args) == 2 && slices.Contains(versionArgs, os.Args[1]) {
		fmt.Println(version())
		return
	}
	opts, err := parseArgs(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		os.Exit(exitError)
	}

	report.Color = opts.format == "text" && isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == ""
	result := collect(opts)
	switch opts.format {
	case "json":
		mustReport(report.JSON(os.Stdout, result))
	case "github":
		mustReport(report.GitHub(os.Stdout, result))
		printText(opts, result)
	default:
		printWarnings(result.Warnings)
		printText(opts, result)
	}
	if result.HasProblems() {
		os.Exit(exitProblems)
	}
}

func collect(opts options) report.Result {
	tree := must(scan.Scan(opts.root))
	result := report.Result{}
	for _, broken := range tree.Broken {
		result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %s, it doesn't parse: %v", broken.Path, broken.Err))
	}

	var changed map[string]changes.File
	if opts.filtered() {
		changed = must(changes.Since(opts.root, opts.since))
	}
	coverage := run.NewCoverage()

	if opts.command == "check" {
		check(opts, tree, changed, coverage, &result)
		return result
	}

	result.Generated = generate(opts, tree, changed, coverage)
	if opts.command == "run" {
		dirs := must(scan.Dirs(opts.root, scan.IsYawntestFile))
		result.Findings = must(run.YawntestTests(dirs))
	}
	return result
}

func printText(opts options, result report.Result) {
	if opts.command == "check" {
		mustReport(report.Untested(os.Stdout, result.Untested))
		mustReport(report.Hints(os.Stdout, result.Untested))
		if !opts.static {
			mustReport(report.ErrorPaths(os.Stdout, result.ErrorPaths, result.NoTests))
		}
		return
	}

	mustReport(report.Generated(os.Stdout, result.Generated))
	if opts.command == "run" {
		mustReport(report.Findings(os.Stdout, result.Findings, result.Generated))
	}
}

func printWarnings(warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "yawntest: warning:", warning)
	}
}

// check fills result; changed is nil unless only changed code counts.
func check(opts options, tree scan.Tree, changed map[string]changes.File, coverage *run.Coverage, result *report.Result) {
	files := tree.Untested(true)
	sources := tree.Files
	if changed != nil {
		files = must(changes.KeepChangedMatches(files, changed))
		sources = must(changes.KeepTouched(sources, changed))
	}
	if !opts.static {
		errorPaths := run.UntestedErrorPaths(sources, tree.HasTests, coverage)
		result.ErrorPaths = errorPaths.Paths
		if changed != nil {
			result.ErrorPaths = must(changes.KeepChangedErrorPaths(result.ErrorPaths, changed))
		}
		result.NoTests = errorPaths.NoTests
		result.Warnings = append(result.Warnings, errorPaths.Warnings...)
		files = run.WithoutCovered(files, coverage, false)
	}
	result.Untested = files
}

// Without --force existing yawntest files count as tests, so only sources with new candidates show up.
func generate(opts options, tree scan.Tree, changed map[string]changes.File, coverage *run.Coverage) []report.GeneratedFile {
	files := tree.Untested(!opts.force)
	if changed != nil {
		files = must(changes.KeepChangedFiles(files, changed))
	}
	if !opts.static {
		files = run.WithoutCovered(files, coverage, true)
	}

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

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
