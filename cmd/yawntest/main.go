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
	json    bool
}

var versionArgs = []string{"version", "-version", "--version"}

// version is the module version go install stamped into the binary, "(devel)" for other builds.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
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

	report.Color = !opts.json && isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == ""
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

func collect(opts options) report.Result {
	if opts.command == "check" {
		return check(opts)
	}

	result := report.Result{Generated: generate(opts)}
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
		mustReport(report.ErrorPaths(os.Stdout, result.ErrorPaths))
		return
	}

	mustReport(report.Generated(os.Stdout, result.Generated))
	if opts.command == "run" {
		mustReport(report.Findings(os.Stdout, result.Findings, result.Generated))
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

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
