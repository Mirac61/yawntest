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
	command   string // check, gen or run
	root      string
	force     bool
	changed   bool
	since     string // git ref; like changed, but since the merge base with it
	static    bool   // run no tests
	format    string // text, json, github or sarif
	sarifFile string // --sarif: also write SARIF here
	json      bool   // --json, folded into format by parseArgs
}

// filtered reports whether only changed code counts.
func (o options) filtered() bool {
	return o.changed || o.since != ""
}

var versionArgs = []string{"version", "-version", "--version"}

// stampedVersion is set with -ldflags "-X main.stampedVersion=v1.2.3" by the release workflow
// and the GitHub Action, which build from a source tree without go install's module version.
var stampedVersion string

// version is the version stamped into the binary, "(devel)" for builds that have none.
func version() string {
	if stampedVersion != "" {
		return stampedVersion
	}
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
	case "sarif":
		printWarnings(result.Warnings) // SARIF has no place for issues without a file
		mustReport(report.SARIF(os.Stdout, result, version()))
	default:
		printWarnings(result.Warnings)
		printText(opts, result)
	}
	if opts.sarifFile != "" {
		writeSARIF(opts.sarifFile, result)
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
	coverage.Progress = statusLine("running tests")

	if opts.command == "check" {
		check(opts, tree, changed, coverage, &result)
		return result
	}

	result.Generated = generate(opts, tree, changed, coverage)
	if opts.command == "run" {
		dirs := must(scan.Dirs(opts.root, scan.IsYawntestFile))
		result.Findings = must(run.YawntestTests(dirs, statusLine("running the generated tests")))
	}
	return result
}

func printText(opts options, result report.Result) {
	mustReport(report.Text(os.Stdout, result, report.View{Command: opts.command, Static: opts.static}))
}

func writeSARIF(path string, result report.Result) {
	file, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	if err := report.SARIF(file, result, version()); err != nil {
		file.Close()
		fail(fmt.Errorf("write %s: %w", path, err))
	}
	if err := file.Close(); err != nil {
		fail(fmt.Errorf("write %s: %w", path, err))
	}
}

// statusLine shows "running tests 3/12…" on a terminal's stderr while go test runs, and
// erases it when the batch is done. Elsewhere, like in CI logs, it shows nothing.
func statusLine(label string) run.Progress {
	if !isTerminal(os.Stderr) {
		return nil
	}
	return func(done, total int) {
		if done == total {
			fmt.Fprint(os.Stderr, "\r\x1b[K")
			return
		}
		fmt.Fprintf(os.Stderr, "\r\x1b[K%s %d/%d…", label, done, total)
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
